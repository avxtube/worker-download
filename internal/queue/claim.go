package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"worker-download/internal/core/enums"
	"worker-download/internal/db/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ─── Claim ────────────────────────────────────────────────────
//
// The enqueuer (vdohide-service) inserts pending jobs into video_process.
// Workers never scan the files collection — they only claim from here.
//
// Claim is atomic: FindOneAndUpdate flips pending → processing and stamps
// workerId + claimedAt in one operation, so two workers can never grab the
// same job. Sort must match the index {processType, status, priority: -1,
// createdAt: 1} — highest priority first, then oldest.

// Claim atomically claims the next pending job for this worker.
// Returns (nil, nil) when the queue is empty.
func Claim(ctx context.Context, workerID string) (*models.VideoProcess, error) {
	now := time.Now()
	leaseExpiresAt := now.Add(2 * time.Minute)
	legacyLeaseCutoff := now.Add(-2 * time.Minute)
	job, err := models.VideoProcessModel.FindOneAndUpdate(ctx,
		bson.M{
			"processType": enums.ProcessTypeDownload,
			"$or": []bson.M{
				{"status": enums.ProcessStatusPending, "$or": []bson.M{
					{"nextRetryAt": bson.M{"$exists": false}},
					{"nextRetryAt": bson.M{"$lte": now}},
				}},
				{"status": enums.ProcessStatusProcessing, "$or": []bson.M{
					{"leaseExpiresAt": bson.M{"$lte": now}},
					{"leaseExpiresAt": bson.M{"$exists": false}, "heartbeatAt": bson.M{"$lte": legacyLeaseCutoff}},
					{"leaseExpiresAt": bson.M{"$exists": false}, "heartbeatAt": bson.M{"$exists": false}, "claimedAt": bson.M{"$lte": legacyLeaseCutoff}},
				}},
			},
		},
		bson.M{
			"$set": bson.M{
				"status":         enums.ProcessStatusProcessing,
				"workerId":       workerID,
				"claimedAt":      now,
				"heartbeatAt":    now,
				"leaseExpiresAt": leaseExpiresAt,
				"startedAt":      now,
			},
			"$unset": bson.M{"finishedAt": "", "nextRetryAt": ""},
		},
		options.FindOneAndUpdate().
			SetSort(bson.D{{Key: "priority", Value: -1}, {Key: "createdAt", Value: 1}}).
			SetReturnDocument(options.After),
	)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // queue empty — not an error
		}
		return nil, err
	}
	return job, nil
}

// ResumeOwn returns this worker's own processing job, if any — used on
// startup to resume work interrupted by a crash/restart instead of
// claiming a new job while an old one still holds a slot.
func ResumeOwn(ctx context.Context, workerID string) (*models.VideoProcess, error) {
	job, err := models.VideoProcessModel.FindOne(ctx, bson.M{
		"processType": enums.ProcessTypeDownload,
		"status":      enums.ProcessStatusProcessing,
		"workerId":    workerID,
	})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

// ─── Job lifecycle ────────────────────────────────────────────

// Complete marks a job as completed (terminal). The partial unique index
// only covers pending/processing, so the same file can be re-enqueued later.
func Complete(ctx context.Context, jobID, workerID string) error {
	now := time.Now()
	result, err := models.VideoProcessModel.Col().UpdateOne(ctx, bson.M{
		"_id": jobID, "status": enums.ProcessStatusProcessing, "workerId": workerID,
	}, bson.M{
		"$set": bson.M{
			"status":         enums.ProcessStatusCompleted,
			"overallPercent": 100.0,
			"finishedAt":     now,
			"updatedAt":      now,
		},
		"$unset": bson.M{"leaseExpiresAt": ""},
	})
	if err == nil && result.MatchedCount == 0 {
		return fmt.Errorf("job lease lost before completion")
	}
	return err
}

// MaxRetries — a job fails this many times before going terminal.
const MaxRetries = 3

// CategoryPermanent marks a failure that retrying cannot fix. Written to
// errorCategory so the admin can tell "give up, the source is bad" apart from
// "it broke, we already tried 3 times".
const CategoryPermanent = "permanent"

// Sentinel errors a JobHandler can return to control settling:
var (
	// ErrJobCancelled — admin set status=cancelled mid-run; leave the doc
	// alone (don't overwrite with completed/failed).
	ErrJobCancelled = errors.New("job cancelled")
	// ErrJobRequeue — failure is not the job's fault (e.g. disk full);
	// Release back to pending WITHOUT counting a retry.
	ErrJobRequeue = errors.New("job requeue")
	// ErrPermanent — retrying cannot help (source file is truncated/corrupt).
	// Go terminal on the first attempt instead of burning MaxRetries runs on
	// input that will never decode: the retry path keeps source.mp4 and skips
	// the download, so every attempt would re-run the same doomed encode.
	ErrPermanent = errors.New("permanent failure")
)

// retryBackoff returns the wait before attempt n runs again (1m, 2m, ...).
func retryBackoff(attempt int) time.Duration {
	return time.Duration(1<<(attempt-1)) * time.Minute
}

// RetryOrFail settles a failed run. Under MaxRetries the SAME doc goes back
// to pending with a backoff (nextRetryAt) — no new doc, retryCount is the
// single source of truth. At MaxRetries the job goes terminal AND the file
// is flipped waiting → error, which stops the enqueuer from ever re-picking
// it (the enqueuer only scans waiting files). Returns retried=true if the
// job was requeued.
func RetryOrFail(ctx context.Context, job *models.VideoProcess, workerID, errMsg string, category string) (retried bool, err error) {
	attempt := 1
	if job.RetryCount != nil {
		attempt = *job.RetryCount + 1
	}

	if attempt < MaxRetries && category != CategoryPermanent {
		result, updateErr := models.VideoProcessModel.Col().UpdateOne(ctx, bson.M{
			"_id": job.ID, "status": enums.ProcessStatusProcessing, "workerId": workerID,
		}, bson.M{
			"$set": bson.M{
				"status":        enums.ProcessStatusPending,
				"error":         errMsg,
				"errorCategory": category,
				"nextRetryAt":   time.Now().Add(retryBackoff(attempt)),
				"updatedAt":     time.Now(),
			},
			"$inc":   bson.M{"retryCount": 1},
			"$unset": bson.M{"workerId": "", "claimedAt": "", "heartbeatAt": "", "leaseExpiresAt": "", "startedAt": ""},
		})
		if updateErr != nil {
			return false, updateErr
		}
		if result.MatchedCount == 0 {
			return false, fmt.Errorf("job lease lost before retry")
		}
		return true, nil
	}

	// terminal — mark job failed, then flip the file out of waiting so the
	// enqueuer stops re-enqueuing it forever (needs admin action to retry)
	result, err := models.VideoProcessModel.Col().UpdateOne(ctx, bson.M{
		"_id": job.ID, "status": enums.ProcessStatusProcessing, "workerId": workerID,
	}, bson.M{
		"$set": bson.M{
			"status":        enums.ProcessStatusFailed,
			"error":         errMsg,
			"errorCategory": category,
			"finishedAt":    time.Now(),
			"updatedAt":     time.Now(),
		},
		"$inc":   bson.M{"retryCount": 1},
		"$unset": bson.M{"leaseExpiresAt": ""},
	})
	if err != nil {
		return false, err
	}
	if result.MatchedCount == 0 {
		return false, fmt.Errorf("job lease lost before failure")
	}

	if job.FileID != nil {
		now := time.Now()
		_, fErr := models.FileModel.Col().UpdateOne(ctx,
			bson.M{"_id": *job.FileID, "status": bson.M{"$in": []string{enums.FileStatusPending, enums.FileStatusQueue, enums.FileStatusProcessing}}},
			bson.M{"$set": bson.M{"status": enums.FileStatusError, "updatedAt": now}},
		)
		if fErr != nil {
			return false, fErr
		}
	}
	return false, nil
}

// Release returns a claimed job to the queue (processing → pending),
// clearing ownership. Called on graceful shutdown so another worker can
// pick the job up immediately instead of waiting for the reaper.
func Release(ctx context.Context, jobID, workerID string) error {
	_, err := models.VideoProcessModel.FindOneAndUpdate(ctx,
		bson.M{
			"_id":      jobID,
			"status":   enums.ProcessStatusProcessing,
			"workerId": workerID,
		},
		bson.M{
			"$set":   bson.M{"status": enums.ProcessStatusPending, "updatedAt": time.Now()},
			"$unset": bson.M{"workerId": "", "claimedAt": "", "heartbeatAt": "", "leaseExpiresAt": "", "startedAt": ""},
		},
	)
	if err != nil && errors.Is(err, mongo.ErrNoDocuments) {
		return nil // already completed/reaped — nothing to release
	}
	return err
}

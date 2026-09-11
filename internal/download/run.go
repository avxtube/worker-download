package download

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"worker-download/internal/config"
	"worker-download/internal/core/enums"
	"worker-download/internal/core/utils"
	"worker-download/internal/db/models"
	"worker-download/internal/downloader"
	"worker-download/internal/queue"
	"worker-download/internal/uploader"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func s3VideoObjectKey(fileID, fileName string) string {
	return fileID + "/" + fileName
}

func s3TempObjectKey(now time.Time, fileID, fileName string) string {
	return fmt.Sprintf("%s/%s_%s", now.Format("2006-01-02"), fileID, fileName)
}

func Run(ctx context.Context, process *models.VideoProcess) error {
	if process.FileID == nil || strings.TrimSpace(*process.FileID) == "" {
		return fmt.Errorf("video_process.fileId is required: %w", queue.ErrPermanent)
	}
	file, err := models.FileModel.FindByID(ctx, *process.FileID)
	if err != nil {
		return fmt.Errorf("load file %s: %w", *process.FileID, err)
	}
	slug := strings.TrimSpace(file.Slug)
	if slug == "" {
		slug = file.ID
	}
	workDir := filepath.Join(config.AppConfig.WorkDir, process.ID)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("create work directory: %w", err)
	}
	processLogger := utils.NewProcessLogger(slug, workDir)
	defer processLogger.Close()
	log.Printf("START job=%s file=%s work=%s", process.ID, file.ID, workDir)

	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	_, _ = models.FileModel.UpdateByID(ctx, file.ID, bson.M{"$set": bson.M{
		"status": enums.FileStatusProcessing, "updatedAt": time.Now(),
	}})

	outputPath, sourceIngest, sourceType, m3u8URL, err := acquireSource(ctx, process, file, workDir, slug)
	if err != nil {
		return err
	}

	if err := downloader.ValidateVideoFile(outputPath); err != nil {
		return fmt.Errorf("validate source video: %v: %w", err, queue.ErrPermanent)
	}
	var assets []downloader.SplitAsset
	if config.AppConfig.MediaLayout == "separated" {
		lock := utils.AcquireProcessingLock("processing")
		defer lock.Release()
		startStep(ctx, process.ID, "merge")
		assets, err = downloader.SplitMedia(ctx, outputPath, filepath.Join(workDir, "separated"), trackedPercent(ctx, process.ID, slug, "merge"))
		if err != nil {
			if errors.Is(context.Cause(ctx), queue.ErrJobCancelled) {
				return queue.ErrJobCancelled
			}
			return fmt.Errorf("split video/audio/subtitles: %w", err)
		}
		videoPath := ""
		for _, asset := range assets {
			if asset.Kind == downloader.SplitAssetVideo {
				videoPath = asset.Path
				break
			}
		}
		if videoPath == "" {
			return fmt.Errorf("split produced no video asset")
		}
		outputPath = videoPath
		completeStep(ctx, process.ID, "merge")
	} else {
		if filepath.Base(outputPath) != models.FileNameOriginal {
			lock := utils.AcquireProcessingLock("processing")
			defer lock.Release()
			startStep(ctx, process.ID, "merge")
			normalized := filepath.Join(workDir, models.FileNameOriginal)
			if err := downloader.EnsureH264Faststart(ctx, outputPath, normalized, trackedPercent(ctx, process.ID, slug, "merge")); err != nil {
				if errors.Is(context.Cause(ctx), queue.ErrJobCancelled) {
					return queue.ErrJobCancelled
				}
				return fmt.Errorf("normalize video: %w", err)
			}
			outputPath = normalized
			completeStep(ctx, process.ID, "merge")
		}
		info, statErr := os.Stat(outputPath)
		if statErr != nil {
			return fmt.Errorf("stat output: %w", statErr)
		}
		assets = []downloader.SplitAsset{{
			Kind: downloader.SplitAssetVideo, Path: outputPath,
			FileName: models.FileNameOriginal, MimeType: "video/mp4", Size: info.Size(),
		}}
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		return fmt.Errorf("stat output: %w", err)
	}
	probe, probeErr := downloader.ProbeVideoInfo(outputPath)
	if probeErr != nil {
		log.Printf("Probe warning job=%s: %v", process.ID, probeErr)
	}

	storage, err := resolveTempStorage(ctx, process)
	if err != nil {
		return err
	}
	if err := uploader.PrepareStorageCredentials(storage, config.AppConfig.StorageEncryptionKey); err != nil {
		return fmt.Errorf("prepare temp storage credentials: %w", err)
	}

	now := time.Now()
	startStep(ctx, process.ID, "upload")
	for _, asset := range assets {
		objectKey := s3TempObjectKey(now, file.ID, asset.FileName)
		switch storage.Provider {
		case enums.StorageTypeS3:
			if err := uploader.UploadToS3(ctx, storage, asset.Path, objectKey, trackedBytes(ctx, process.ID, slug, "upload")); err != nil {
				return fmt.Errorf("upload temp S3 %s: %w", asset.FileName, err)
			}
		case enums.StorageTypeLocal:
			if storage.Local == nil || strings.TrimSpace(storage.Local.BasePath) == "" {
				return fmt.Errorf("local temp storage has no basePath")
			}
			target := filepath.Join(storage.Local.BasePath, filepath.FromSlash(objectKey))
			if err := copyFileLocal(asset.Path, target, trackedBytes(ctx, process.ID, slug, "upload")); err != nil {
				return fmt.Errorf("copy %s to local temp storage: %w", asset.FileName, err)
			}
		default:
			return fmt.Errorf("unsupported temp storage provider %q", storage.Provider)
		}
		assetSize := asset.Size
		if assetSize <= 0 {
			if assetInfo, statErr := os.Stat(asset.Path); statErr == nil {
				assetSize = assetInfo.Size()
			}
		}
		uploadedAt := time.Now()
		ingestFilter := bson.M{
			"fileId": file.ID, "destination": "storage", "sourceType": enums.IngestSourceTypeProcessed,
			"storageId": storage.ID, "key": objectKey,
		}
		ingestUpdate := bson.M{
			"$set": bson.M{
				"status": "uploaded", "fileName": asset.FileName, "mime": asset.MimeType,
				"size": assetSize, "uploadedAt": uploadedAt, "updatedAt": uploadedAt,
			},
			"$setOnInsert": bson.M{
				"_id": newUUID(), "fileId": file.ID, "destination": "storage",
				"sourceType": enums.IngestSourceTypeProcessed, "storageId": storage.ID,
				"key": objectKey, "createdAt": uploadedAt,
			},
		}
		if _, err := models.IngestModel.Col().UpdateOne(ctx, ingestFilter, ingestUpdate, options.Update().SetUpsert(true)); err != nil {
			return fmt.Errorf("save processed ingest %s: %w", asset.FileName, err)
		}
	}
	completeStep(ctx, process.ID, "upload")
	size := info.Size()
	uploadedAt := time.Now()
	if sourceIngest != nil {
		_, _ = models.IngestModel.Col().UpdateOne(ctx,
			bson.M{"_id": sourceIngest.ID, "status": bson.M{"$in": []string{"uploaded", "processing"}}},
			bson.M{"$set": bson.M{"status": "consumed", "consumedAt": uploadedAt, "updatedAt": uploadedAt}},
		)
	}

	fileUpdate := bson.M{
		"status": enums.FileStatusReadyOriginal, "updatedAt": uploadedAt,
		"metadata.mediaLayout": config.AppConfig.MediaLayout,
	}
	audioTracks, subtitleTracks := 0, 0
	for _, asset := range assets {
		switch asset.Kind {
		case downloader.SplitAssetAudio:
			audioTracks++
		case downloader.SplitAssetSubtitle:
			subtitleTracks++
		}
	}
	if config.AppConfig.MediaLayout == "separated" {
		fileUpdate["metadata.audioTrackCount"] = audioTracks
		fileUpdate["metadata.subtitleTrackCount"] = subtitleTracks
	}
	if probe != nil {
		if probe.Duration > 0 {
			fileUpdate["metadata.duration"] = probe.Duration
		}
		shortSide := probe.Height
		if probe.Width > 0 && (shortSide == 0 || probe.Width < shortSide) {
			shortSide = probe.Width
		}
		if shortSide > 0 {
			fileUpdate["metadata.highestQuality"] = DetermineHighestResolution(int(shortSide))
		}
	}
	if _, err := models.FileModel.UpdateByID(ctx, file.ID, bson.M{"$set": fileUpdate}); err != nil {
		return fmt.Errorf("mark file ready_original: %w", err)
	}
	_, _ = models.VideoProcessModel.UpdateOne(ctx, bson.M{"_id": process.ID}, bson.M{"$set": bson.M{
		"sourceType": sourceType, "m3u8_url": m3u8URL, "file_name": models.FileNameOriginal,
		"file_size": size, "updatedAt": uploadedAt,
	}})
	log.Printf("COMPLETE job=%s ingests=%d storage=%s", process.ID, len(assets), storage.ID)
	processLogger.Close()
	if err := downloader.Cleanup(workDir); err != nil {
		log.Printf("Cleanup warning job=%s work=%s: %v", process.ID, workDir, err)
	}
	return nil
}

func acquireSource(ctx context.Context, process *models.VideoProcess, file *models.File, workDir, slug string) (string, *models.Ingest, string, string, error) {
	sourcePath := filepath.Join(workDir, "source.mp4")
	ingest, err := models.IngestModel.FindOne(ctx, bson.M{
		"fileId":     file.ID,
		"sourceType": bson.M{"$ne": enums.IngestSourceTypeProcessed},
		"status":     bson.M{"$in": []string{"uploaded", "processing"}},
	}, options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err == nil && ingest != nil {
		if info, ok := reusableSourceFile(sourcePath); ok {
			log.Printf("Reusing source.mp4 (%.2f MB)", float64(info.Size())/1024/1024)
			return sourcePath, ingest, ingest.SourceType, "", nil
		}
		startStep(ctx, process.ID, "download")
		if err := downloadIngest(ctx, ingest, sourcePath, process.ID, slug); err != nil {
			return "", ingest, ingest.SourceType, "", err
		}
		completeStep(ctx, process.ID, "download")
		return sourcePath, ingest, ingest.SourceType, "", nil
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil, "", "", fmt.Errorf("find source ingest: %w", err)
	}

	mergedPath := filepath.Join(workDir, models.FileNameOriginal)
	if info, ok := reusableSourceFile(mergedPath); ok {
		log.Printf("Reusing file_original.mp4 (%.2f MB)", float64(info.Size())/1024/1024)
		return mergedPath, nil, "remote", "", nil
	}

	sourceURL, playlistURL := "", ""
	if file.Metadata != nil {
		sourceURL = strings.TrimSpace(derefStr(file.Metadata.Source))
		playlistURL = strings.TrimSpace(derefStr(file.Metadata.Playlists))
	}
	if playlistURL == "" && downloader.IsDirectVideoURL(sourceURL) {
		startStep(ctx, process.ID, "download")
		if err := downloader.DownloadDirectFile(ctx, sourceURL, sourcePath, trackedBytes(ctx, process.ID, slug, "download")); err != nil {
			return "", nil, "remote", "", fmt.Errorf("direct download: %w", err)
		}
		completeStep(ctx, process.ID, "download")
		return sourcePath, nil, "remote", "", nil
	}
	if playlistURL == "" && strings.Contains(strings.ToLower(sourceURL), ".m3u8") {
		playlistURL = sourceURL
	}
	if playlistURL == "" {
		if sourceURL == "" {
			return "", nil, "", "", fmt.Errorf("file has no source ingest, metadata.playlists, or metadata.source: %w", queue.ErrPermanent)
		}
		scraperURL := getScraperURL(ctx)
		if scraperURL == "" {
			return "", nil, "", "", fmt.Errorf("metadata.playlists is empty and scraper URL is not configured")
		}
		var fetchErr error
		playlistURL, _, fetchErr = downloader.FetchM3U8FromScraper(scraperURL, sourceURL)
		if fetchErr != nil {
			return "", nil, "", "", fmt.Errorf("scraper: %w", fetchErr)
		}
	}

	startStep(ctx, process.ID, "download")
	hlsCtx := downloader.WithReferer(ctx, sourceURL)
	result, err := downloader.DownloadHLSSegments(hlsCtx, playlistURL, workDir, &downloader.DownloadProgress{
		OnProgress: trackedSegments(ctx, process.ID, slug),
	})
	if err != nil {
		return "", nil, "remote", playlistURL, fmt.Errorf("download HLS: %w", err)
	}
	completeStep(ctx, process.ID, "download")
	startStep(ctx, process.ID, "merge")
	outputPath := filepath.Join(workDir, models.FileNameOriginal)
	merge, err := downloader.MergeToMP4(ctx, result.SegmentFiles, outputPath, trackedPercent(ctx, process.ID, slug, "merge"))
	if err != nil {
		return "", nil, "remote", playlistURL, fmt.Errorf("merge HLS: %w", err)
	}
	completeStep(ctx, process.ID, "merge")
	_ = merge
	_, _ = models.VideoProcessModel.UpdateOne(ctx, bson.M{"_id": process.ID}, bson.M{"$set": bson.M{"resolution": result.ResolutionFull}})
	return outputPath, nil, "remote", playlistURL, nil
}

func downloadIngest(ctx context.Context, ingest *models.Ingest, outputPath, processID, slug string) error {
	if ingest.Destination == "upload_server" {
		if ingest.TemporaryPath == nil || strings.TrimSpace(*ingest.TemporaryPath) == "" {
			return fmt.Errorf("upload_server ingest has no temporaryPath: %w", queue.ErrPermanent)
		}
		return copyFileLocal(*ingest.TemporaryPath, outputPath, trackedBytes(ctx, processID, slug, "download"))
	}
	if ingest.StorageID == nil || ingest.Key == nil || *ingest.Key == "" {
		return fmt.Errorf("storage ingest has no storageId/key: %w", queue.ErrPermanent)
	}
	storage, err := models.StorageModel.FindByID(ctx, *ingest.StorageID)
	if err != nil {
		return fmt.Errorf("load ingest storage: %w", err)
	}
	if err := uploader.PrepareStorageCredentials(storage, config.AppConfig.StorageEncryptionKey); err != nil {
		return err
	}
	switch storage.Provider {
	case enums.StorageTypeS3:
		return downloader.DownloadFromS3(ctx, storage, *ingest.Key, outputPath, trackedBytes(ctx, processID, slug, "download"))
	case enums.StorageTypeLocal:
		if storage.Local == nil || storage.Local.BasePath == "" {
			return fmt.Errorf("local ingest storage has no basePath")
		}
		return copyFileLocal(filepath.Join(storage.Local.BasePath, filepath.FromSlash(*ingest.Key)), outputPath, trackedBytes(ctx, processID, slug, "download"))
	default:
		return fmt.Errorf("unsupported ingest storage provider %q", storage.Provider)
	}
}

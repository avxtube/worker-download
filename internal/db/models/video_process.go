package models

import (
	"time"

	"github.com/zergolf1994/goose"
)

const FileNameOriginal = "file_original.mp4"

type StepProgress struct {
	Status    *string    `bson:"status,omitempty" json:"status,omitempty"`
	Percent   *float64   `bson:"percent,omitempty" json:"percent,omitempty"`
	Current   *int       `bson:"current,omitempty" json:"current,omitempty"`
	Total     *int       `bson:"total,omitempty" json:"total,omitempty"`
	Speed     *string    `bson:"speed,omitempty" json:"speed,omitempty"`
	StartedAt *time.Time `bson:"startedAt,omitempty" json:"startedAt,omitempty"`
	EndedAt   *time.Time `bson:"endedAt,omitempty" json:"endedAt,omitempty"`
}

type VideoProcess struct {
	ID                   string      `bson:"_id" json:"id" goose:"required,default:uuid"`
	FileID               *string     `bson:"fileId" json:"fileId" goose:"required,index"`
	Slug                 *string     `bson:"-" json:"slug,omitempty"`
	ProcessType          string      `bson:"processType" json:"processType" goose:"default:download"`
	DedupeKey            string      `bson:"dedupeKey" json:"dedupeKey"`
	Status               *string     `bson:"status,omitempty" json:"status,omitempty" goose:"index"`
	Priority             *int        `bson:"priority,omitempty" json:"priority,omitempty"`
	WorkerID             *string     `bson:"workerId,omitempty" json:"workerId,omitempty" goose:"index"`
	TargetStorageID      *string     `bson:"targetStorageId,omitempty" json:"targetStorageId,omitempty"`
	DestinationStorageID *string     `bson:"destinationStorageId,omitempty" json:"destinationStorageId,omitempty"`
	TransferMode         *string     `bson:"transferMode,omitempty" json:"transferMode,omitempty"`
	SourceStorageID      *string     `bson:"sourceStorageId,omitempty" json:"sourceStorageId,omitempty"`
	TempStorageID        *string     `bson:"tempStorageId,omitempty" json:"tempStorageId,omitempty"`
	MigrationID          *string     `bson:"migrationId,omitempty" json:"migrationId,omitempty"`
	SourceMediaIDs       []string    `bson:"sourceMediaIds,omitempty" json:"sourceMediaIds,omitempty"`
	ClaimedAt            *time.Time  `bson:"claimedAt,omitempty" json:"claimedAt,omitempty"`
	HeartbeatAt          *time.Time  `bson:"heartbeatAt,omitempty" json:"heartbeatAt,omitempty"`
	LeaseExpiresAt       *time.Time  `bson:"leaseExpiresAt,omitempty" json:"leaseExpiresAt,omitempty"`
	StartedAt            *time.Time  `bson:"startedAt,omitempty" json:"startedAt,omitempty"`
	FinishedAt           *time.Time  `bson:"finishedAt,omitempty" json:"finishedAt,omitempty"`
	NextRetryAt          *time.Time  `bson:"nextRetryAt,omitempty" json:"nextRetryAt,omitempty"`
	OverallPercent       *float64    `bson:"overallPercent,omitempty" json:"overallPercent,omitempty"`
	Timeline             interface{} `bson:"timeline,omitempty" json:"timeline,omitempty"`
	FileName             *string     `bson:"file_name,omitempty" json:"fileName,omitempty"`
	FileSize             interface{} `bson:"file_size,omitempty" json:"fileSize,omitempty"`
	Resolution           *string     `bson:"resolution,omitempty" json:"resolution,omitempty"`
	SourceType           *string     `bson:"sourceType,omitempty" json:"sourceType,omitempty"`
	M3U8URL              *string     `bson:"m3u8_url,omitempty" json:"m3u8Url,omitempty"`
	Resolutions          []string    `bson:"resolutions,omitempty" json:"resolutions,omitempty"`
	Completed            []string    `bson:"completed,omitempty" json:"completed,omitempty"`
	Error                *string     `bson:"error,omitempty" json:"error,omitempty"`
	ErrorCategory        *string     `bson:"errorCategory,omitempty" json:"errorCategory,omitempty"`
	RetryCount           *int        `bson:"retryCount,omitempty" json:"retryCount,omitempty"`
	CreatedAt            time.Time   `bson:"createdAt" json:"createdAt" goose:"default:now"`
	UpdatedAt            time.Time   `bson:"updatedAt" json:"updatedAt" goose:"default:now"`
}

var VideoProcessModel = goose.NewModel[VideoProcess]("video_process")

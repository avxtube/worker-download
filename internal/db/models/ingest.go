package models

import (
	"time"

	"github.com/zergolf1994/goose"
)

type IngestMultipart struct {
	UploadID    *string    `bson:"uploadId,omitempty" json:"uploadId,omitempty"`
	PartSize    *int64     `bson:"partSize,omitempty" json:"partSize,omitempty"`
	InitiatedAt *time.Time `bson:"initiatedAt,omitempty" json:"initiatedAt,omitempty"`
	CompletedAt *time.Time `bson:"completedAt,omitempty" json:"completedAt,omitempty"`
}

type Ingest struct {
	ID            string           `bson:"_id" json:"id" goose:"required,default:uuid"`
	FileID        string           `bson:"fileId" json:"fileId" goose:"required,index"`
	Destination   string           `bson:"destination" json:"destination"`
	Status        string           `bson:"status" json:"status" goose:"default:pending"`
	FileName      string           `bson:"fileName" json:"fileName"`
	Mime          *string          `bson:"mime,omitempty" json:"mime,omitempty"`
	Size          *int64           `bson:"size,omitempty" json:"size,omitempty"`
	Checksum      *string          `bson:"checksum,omitempty" json:"checksum,omitempty"`
	UploadedBy    *string          `bson:"uploadedBy,omitempty" json:"uploadedBy,omitempty"`
	SourceType    string           `bson:"sourceType" json:"sourceType" goose:"default:upload"`
	UploadNodeID  *string          `bson:"uploadNodeId,omitempty" json:"uploadNodeId,omitempty"`
	TemporaryPath *string          `bson:"temporaryPath,omitempty" json:"temporaryPath,omitempty"`
	StorageID     *string          `bson:"storageId,omitempty" json:"storageId,omitempty"`
	Key           *string          `bson:"key,omitempty" json:"key,omitempty"`
	Multipart     *IngestMultipart `bson:"multipart,omitempty" json:"multipart,omitempty"`
	Error         *string          `bson:"error,omitempty" json:"error,omitempty"`
	UploadedAt    *time.Time       `bson:"uploadedAt,omitempty" json:"uploadedAt,omitempty"`
	ConsumedAt    *time.Time       `bson:"consumedAt,omitempty" json:"consumedAt,omitempty"`
	ExpiresAt     *time.Time       `bson:"expiresAt,omitempty" json:"expiresAt,omitempty"`
	CreatedAt     time.Time        `bson:"createdAt" json:"createdAt" goose:"default:now"`
	UpdatedAt     time.Time        `bson:"updatedAt" json:"updatedAt" goose:"default:now"`
}

var IngestModel = goose.NewModel[Ingest]("ingests")

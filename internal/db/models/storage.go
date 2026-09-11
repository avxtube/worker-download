package models

import (
	"time"

	"github.com/zergolf1994/goose"
)

type StorageLocalConfig struct {
	BasePath string `bson:"basePath" json:"basePath"`
}

type StorageS3Config struct {
	Endpoint                 *string `bson:"endpoint,omitempty" json:"endpoint,omitempty"`
	Region                   string  `bson:"region" json:"region"`
	Bucket                   string  `bson:"bucket" json:"bucket"`
	Prefix                   string  `bson:"prefix,omitempty" json:"prefix,omitempty"`
	ForcePathStyle           bool    `bson:"forcePathStyle" json:"forcePathStyle"`
	CredentialsConfigured    bool    `bson:"credentialsConfigured" json:"credentialsConfigured"`
	AccessKeyIDEncrypted     string  `bson:"accessKeyIdEncrypted,omitempty" json:"-"`
	SecretAccessKeyEncrypted string  `bson:"secretAccessKeyEncrypted,omitempty" json:"-"`
	AccessKeyID              string  `bson:"-" json:"-"`
	SecretAccessKey          string  `bson:"-" json:"-"`
}

type StorageHealth struct {
	CheckedAt *time.Time `bson:"checkedAt,omitempty" json:"checkedAt,omitempty"`
	LatencyMS *float64   `bson:"latencyMs,omitempty" json:"latencyMs,omitempty"`
	Message   *string    `bson:"message,omitempty" json:"message,omitempty"`
}

type StorageCapacity struct {
	TotalBytes interface{} `bson:"totalBytes,omitempty" json:"totalBytes,omitempty"`
	UsedBytes  interface{} `bson:"usedBytes,omitempty" json:"usedBytes,omitempty"`
	FreeBytes  interface{} `bson:"freeBytes,omitempty" json:"freeBytes,omitempty"`
}

type Storage struct {
	ID        string              `bson:"_id" json:"id" goose:"required,default:uuid"`
	Name      string              `bson:"name" json:"name" goose:"required"`
	Provider  string              `bson:"provider" json:"provider"`
	Enabled   bool                `bson:"enabled" json:"enabled"`
	Priority  int                 `bson:"priority" json:"priority"`
	Purposes  []string            `bson:"purposes" json:"purposes"`
	Kinds     []string            `bson:"kinds" json:"kinds"`
	PublicURL *string             `bson:"publicUrl,omitempty" json:"publicUrl,omitempty"`
	OriginURL *string             `bson:"originUrl,omitempty" json:"originUrl,omitempty"`
	Local     *StorageLocalConfig `bson:"local,omitempty" json:"local,omitempty"`
	S3        *StorageS3Config    `bson:"s3,omitempty" json:"s3,omitempty"`
	Status    string              `bson:"status" json:"status"`
	Health    *StorageHealth      `bson:"health,omitempty" json:"health,omitempty"`
	Capacity  *StorageCapacity    `bson:"capacity,omitempty" json:"capacity,omitempty"`
	CreatedBy string              `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy string              `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	DeletedAt *time.Time          `bson:"deletedAt,omitempty" json:"deletedAt,omitempty"`
	CreatedAt time.Time           `bson:"createdAt" json:"createdAt" goose:"default:now"`
	UpdatedAt time.Time           `bson:"updatedAt" json:"updatedAt" goose:"default:now"`
}

var StorageModel = goose.NewModel[Storage]("storages")

func (s *Storage) GetPath() string {
	if s.Local != nil {
		return s.Local.BasePath
	}
	return ""
}

func (s *Storage) IsOnline() bool {
	return s.Enabled && s.DeletedAt == nil && s.Status == "online"
}

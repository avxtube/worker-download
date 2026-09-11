package models

import (
	"time"

	"github.com/zergolf1994/goose"
)

type FileMetadata struct {
	Duration           *float64   `bson:"duration,omitempty" json:"duration,omitempty"`
	HighestQuality     *int       `bson:"highestQuality,omitempty" json:"highestQuality,omitempty"`
	Playlists          *string    `bson:"playlists,omitempty" json:"playlists,omitempty"`
	Source             *string    `bson:"source,omitempty" json:"source,omitempty"`
	TrashedAt          *time.Time `bson:"trashedAt,omitempty" json:"trashedAt,omitempty"`
	TrashedBy          *string    `bson:"trashedBy,omitempty" json:"trashedBy,omitempty"`
	DeletedAt          *time.Time `bson:"deletedAt,omitempty" json:"deletedAt,omitempty"`
	DeletedBy          *string    `bson:"deletedBy,omitempty" json:"deletedBy,omitempty"`
	MediaLayout        *string    `bson:"mediaLayout,omitempty" json:"mediaLayout,omitempty"`
	AudioTrackCount    *int       `bson:"audioTrackCount,omitempty" json:"audioTrackCount,omitempty"`
	SubtitleTrackCount *int       `bson:"subtitleTrackCount,omitempty" json:"subtitleTrackCount,omitempty"`
}

type File struct {
	ID        string        `bson:"_id" json:"id" goose:"required,default:uuid"`
	OwnerType *string       `bson:"ownerType,omitempty" json:"ownerType,omitempty"`
	OwnerID   *string       `bson:"ownerId,omitempty" json:"ownerId,omitempty"`
	Status    string        `bson:"status" json:"status" goose:"default:pending"`
	Type      string        `bson:"type" json:"type" goose:"default:video"`
	Kind      string        `bson:"kind" json:"kind" goose:"default:stream"`
	Name      string        `bson:"name" json:"name" goose:"required"`
	Slug      string        `bson:"slug" json:"slug" goose:"unique,index"`
	Metadata  *FileMetadata `bson:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt time.Time     `bson:"createdAt" json:"createdAt" goose:"default:now"`
	UpdatedAt time.Time     `bson:"updatedAt" json:"updatedAt" goose:"default:now"`
}

var FileModel = goose.NewModel[File]("files")

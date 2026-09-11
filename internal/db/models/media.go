package models

import (
	"time"

	"github.com/zergolf1994/goose"
)

type Media struct {
	ID        string      `bson:"_id" json:"id" goose:"required,default:uuid"`
	Type      string      `bson:"type" json:"type" goose:"default:video"`
	FileID    string      `bson:"fileId" json:"fileId" goose:"required,index"`
	StorageID string      `bson:"storageId" json:"storageId" goose:"required,index"`
	Quality   *string     `bson:"quality,omitempty" json:"quality,omitempty"`
	Key       string      `bson:"key" json:"key" goose:"required"`
	Mime      string      `bson:"mime" json:"mime" goose:"required"`
	Size      interface{} `bson:"size,omitempty" json:"size,omitempty"`
	Width     *int        `bson:"width,omitempty" json:"width,omitempty"`
	Height    *int        `bson:"height,omitempty" json:"height,omitempty"`
	Slug      string      `bson:"slug" json:"slug" goose:"required,unique"`
	CreatedAt time.Time   `bson:"createdAt" json:"createdAt" goose:"default:now"`
	UpdatedAt time.Time   `bson:"updatedAt" json:"updatedAt" goose:"default:now"`
}

var MediaModel = goose.NewModel[Media]("medias")

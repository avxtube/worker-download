package models

import (
	"time"

	"github.com/zergolf1994/goose"
)

type WorkerSystemInfo struct {
	DiskTotal  int64   `bson:"diskTotal,omitempty" json:"diskTotal,omitempty"`
	DiskUsed   int64   `bson:"diskUsed,omitempty" json:"diskUsed,omitempty"`
	DiskFree   int64   `bson:"diskFree,omitempty" json:"diskFree,omitempty"`
	MemTotal   int64   `bson:"memTotal,omitempty" json:"memTotal,omitempty"`
	MemUsed    int64   `bson:"memUsed,omitempty" json:"memUsed,omitempty"`
	CPUPercent float64 `bson:"cpuPercent,omitempty" json:"cpuPercent,omitempty"`
}

type Worker struct {
	ID           string            `bson:"_id" json:"id" goose:"required,default:uuid"`
	WorkerID     string            `bson:"workerId" json:"workerId" goose:"index"`
	Hostname     string            `bson:"hostname,omitempty" json:"hostname,omitempty"`
	IP           string            `bson:"ip,omitempty" json:"ip,omitempty"`
	PID          int               `bson:"pid,omitempty" json:"pid,omitempty"`
	Version      *string           `bson:"version,omitempty" json:"version,omitempty"`
	Capabilities []string          `bson:"capabilities,omitempty" json:"capabilities,omitempty"`
	StorageID    *string           `bson:"storageId,omitempty" json:"storageId,omitempty"`
	Enable       bool              `bson:"enable" json:"enable"`
	Type         string            `bson:"type,omitempty" json:"type,omitempty"`
	Status       string            `bson:"status" json:"status"`
	ActiveJobs   int               `bson:"activeJobs" json:"activeJobs"`
	MaxJobs      int               `bson:"maxJobs" json:"maxJobs"`
	System       *WorkerSystemInfo `bson:"system,omitempty" json:"system,omitempty"`
	HeartbeatAt  time.Time         `bson:"heartbeatAt" json:"heartbeatAt" goose:"index"`
	CreatedAt    time.Time         `bson:"createdAt" json:"createdAt" goose:"default:now"`
	UpdatedAt    time.Time         `bson:"updatedAt" json:"updatedAt" goose:"default:now"`
}

var WorkerModel = goose.NewModel[Worker]("workers")

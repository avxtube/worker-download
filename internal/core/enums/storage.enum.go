package enums

// ─── Storage Types ───────────────────────────────────────────────────

const (
	StorageTypeLocal = "local"
	StorageTypeS3    = "s3"
)

// ─── Storage Statuses ────────────────────────────────────────────────

const (
	StorageStatusOnline  = "online"
	StorageStatusUnknown = "unknown"
	StorageStatusOffline = "offline"
	StorageStatusError   = "error"
)

// ─── Storage Accepts ─────────────────────────────────────────────────

const (
	StorageAcceptUpload  = "upload"
	StorageAcceptTemp    = "temp"
	StorageAcceptStorage = "storage"
	StorageAcceptVideo   = "video"
	StorageAcceptImage   = "image"
	StorageAcceptOther   = "other"
)

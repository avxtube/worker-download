package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// AppConfig holds the application configuration loaded from environment variables.
var AppConfig Config

// Config represents the application configuration.
type Config struct {
	DashboardPort        string
	MongoURI             string
	WorkDir              string
	LogDir               string
	StorageEncryptionKey string
	WorkerVersion        string

	StorageId   string
	StoragePath string
	ScraperURL  string

	// Number of multipart S3 parts uploaded in parallel.
	S3UploadConcurrency int
	// MediaLayout controls the output produced for new originals:
	// muxed = video+default audio in file_original.mp4 (legacy)
	// separated = video-only original plus independent audio/subtitle medias.
	MediaLayout string
}

// Load reads configuration from environment variables (and .env file).
func Load() {
	// Load .env file if present (ignore error if not found)
	godotenv.Load()

	AppConfig = Config{
		DashboardPort:        getEnv("DASHBOARD_PORT", getEnv("PORT", "8885")),
		MongoURI:             getEnv("DATABASE_URL", "mongodb://localhost:27017"),
		WorkDir:              getEnv("WORK_DIR", defaultWorkDir()),
		LogDir:               getEnv("LOG_DIR", defaultLogDir()),
		StorageEncryptionKey: getEnv("STORAGE_ENCRYPTION_KEY", getEnv("BETTER_AUTH_SECRET", "")),
		WorkerVersion:        getEnv("WORKER_VERSION", "dev"),
		StorageId:            getEnv("STORAGE_ID", ""),
		StoragePath:          getEnv("STORAGE_PATH", "./files"),
		ScraperURL:           getEnv("SCRAPER_URL", ""),
		S3UploadConcurrency:  getIntEnv("S3_UPLOAD_CONCURRENCY", 3, 1, 8),
		MediaLayout:          getMediaLayoutEnv(),
	}
}

func defaultWorkDir() string {
	return filepath.Join(defaultRuntimeDir(), "work")
}

func defaultLogDir() string {
	return filepath.Join(defaultRuntimeDir(), ".log")
}

func defaultRuntimeDir() string {
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		if !strings.Contains(dir, "go-build") {
			return dir
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, ".build")
	}
	return ".build"
}

func getMediaLayoutEnv() string {
	switch getEnv("MEDIA_LAYOUT", "muxed") {
	case "separated":
		return "separated"
	default:
		return "muxed"
	}
}

func getIntEnv(key string, fallback, minValue, maxValue int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

package download

import (
	"context"
	goerrors "errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"worker-download/internal/config"
	"worker-download/internal/core/enums"
	"worker-download/internal/db/models"
	"worker-download/internal/downloader"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func newUUID() string { return uuid.New().String() }

func DetermineHighestResolution(shortSide int) int {
	switch {
	case shortSide >= 2160:
		return 2160
	case shortSide >= 1440:
		return 1440
	case shortSide >= 1080:
		return 1080
	case shortSide >= 720:
		return 720
	case shortSide >= 480:
		return 480
	case shortSide >= 360:
		return 360
	default:
		return shortSide
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func reusableSourceFile(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if info.Size() <= 10*1024 {
		return nil, false
	}
	if err := downloader.ValidateVideoFile(path); err != nil {
		if goerrors.Is(err, downloader.ErrInvalidVideo) {
			log.Printf("Invalid cached source %s: %v; retaining for diagnostics", path, err)
			return nil, false
		}
		return info, true
	}
	return info, true
}

func copyFileLocal(src, dst string, progress func(int64, int64)) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	partPath := dst + ".part"
	out, err := os.Create(partPath)
	if err != nil {
		return err
	}
	completed := false
	defer func() {
		_ = out.Close()
		if !completed {
			_ = os.Remove(partPath)
		}
	}()
	var copied int64
	buf := make([]byte, 512*1024)
	for {
		n, readErr := in.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return err
			}
			copied += int64(n)
			if progress != nil {
				progress(copied, info.Size())
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	_ = os.Remove(dst)
	if err := os.Rename(partPath, dst); err != nil {
		return err
	}
	completed = true
	return nil
}

func resolveTempStorage(ctx context.Context, process *models.VideoProcess) (*models.Storage, error) {
	base := bson.M{
		"enabled":   true,
		"status":    enums.StorageStatusOnline,
		"deletedAt": nil,
		"purposes":  "temp",
		"kinds":     "stream",
	}
	if process.TempStorageID != nil && *process.TempStorageID != "" {
		base["_id"] = *process.TempStorageID
		storage, err := models.StorageModel.FindOne(ctx, base)
		if err != nil {
			return nil, fmt.Errorf("temp storage %s is unavailable: %w", *process.TempStorageID, err)
		}
		return storage, nil
	}
	storage, err := models.StorageModel.FindOne(ctx, base, options.FindOne().SetSort(bson.D{{Key: "priority", Value: 1}, {Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("no enabled online temp stream storage: %w", err)
	}
	return storage, nil
}

func getScraperURL(ctx context.Context) string {
	if config.AppConfig.ScraperURL != "" {
		return config.AppConfig.ScraperURL
	}
	setting, err := models.SettingModel.FindOne(ctx, bson.M{"name": enums.SettingURLScraping})
	if err == nil {
		if value, ok := setting.Value.(string); ok {
			return value
		}
	}
	return ""
}

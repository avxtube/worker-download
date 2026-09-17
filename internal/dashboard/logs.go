package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"worker-download/internal/core/enums"
	"worker-download/internal/db/models"
)

const (
	maxLogResponse  = int64(512 * 1024)
	logHistoryLimit = 20
)

var safeSlug = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type logHistoryItem struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

type logHistoryResponse struct {
	Items      []logHistoryItem `json:"items"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	Total      int              `json:"total"`
	TotalPages int              `json:"totalPages"`
}

func serveLogHistory(logDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		page := 1
		if rawPage := r.URL.Query().Get("page"); rawPage != "" {
			parsed, err := strconv.Atoi(rawPage)
			if err != nil || parsed < 1 {
				http.Error(w, "invalid page", http.StatusBadRequest)
				return
			}
			page = parsed
		}

		result, err := listLogHistory(logDir, page)
		if err != nil {
			http.Error(w, "failed to list logs", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(result)
	}
}

func listLogHistory(logDir string, page int) (logHistoryResponse, error) {
	result := logHistoryResponse{Items: []logHistoryItem{}, Page: page, PageSize: logHistoryLimit}
	entries, err := os.ReadDir(logDir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}

	all := make([]logHistoryItem, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".log") {
			continue
		}
		slug := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if _, ok := processLogPath(logDir, slug); !ok {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			continue
		}
		all = append(all, logHistoryItem{
			Name:      entry.Name(),
			Size:      info.Size(),
			CreatedAt: info.ModTime().UTC(),
		})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].Name > all[j].Name
		}
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	result.Total = len(all)
	result.TotalPages = (result.Total + logHistoryLimit - 1) / logHistoryLimit
	start := (page - 1) * logHistoryLimit
	if start >= result.Total {
		return result, nil
	}
	end := min(start+logHistoryLimit, result.Total)
	result.Items = all[start:end]
	return result, nil
}

// serveProcessLogBySlug exposes a process log directly at /log/{slug}.log.
func serveProcessLogBySlug(logDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/log/")
		if !strings.HasSuffix(name, ".log") || strings.Contains(name, "/") {
			http.Error(w, "invalid log name", http.StatusBadRequest)
			return
		}
		slug := strings.TrimSuffix(name, ".log")
		logPath, ok := processLogPath(logDir, slug)
		if !ok {
			http.Error(w, "invalid log name", http.StatusBadRequest)
			return
		}
		file, err := os.Open(logPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "failed to read log", http.StatusInternalServerError)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			http.Error(w, "failed to read log", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		http.ServeContent(w, r, name, info.ModTime(), file)
	}
}

func serveProcessLog(group, logDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		jobID := strings.TrimPrefix(r.URL.Path, "/logs/")
		if jobID == "" || strings.Contains(jobID, "/") {
			http.Error(w, "invalid job id", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		process, err := models.VideoProcessModel.FindByID(ctx, jobID)
		if err != nil || process == nil || process.ProcessType != enums.ProcessTypeDownload || !belongsToWorkerGroup(process.WorkerID, group) {
			http.NotFound(w, r)
			return
		}
		slug := resolveProcessLogSlug(ctx, process)
		logPath, ok := processLogPath(logDir, slug)
		if !ok {
			http.NotFound(w, r)
			return
		}
		content, truncated, err := readLogTail(logPath, maxLogResponse)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "log is no longer available locally", http.StatusGone)
				return
			}
			http.Error(w, "failed to read log", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if truncated {
			w.Header().Set("X-Log-Truncated", "true")
			_, _ = fmt.Fprintln(w, "--- showing the latest 512 KiB ---")
		}
		_, _ = w.Write(content)
	}
}

// resolveProcessLogSlug supports the current schema where video_process has
// fileId but no slug. Older process documents that still contain a slug keep
// working, while current documents resolve the log filename through files.
func resolveProcessLogSlug(ctx context.Context, process *models.VideoProcess) string {
	if process == nil {
		return ""
	}
	if process.Slug != nil && strings.TrimSpace(*process.Slug) != "" {
		return strings.TrimSpace(*process.Slug)
	}
	if process.FileID == nil || strings.TrimSpace(*process.FileID) == "" {
		return ""
	}
	fileID := strings.TrimSpace(*process.FileID)
	file, err := models.FileModel.FindByID(ctx, fileID)
	if err == nil && file != nil && strings.TrimSpace(file.Slug) != "" {
		return strings.TrimSpace(file.Slug)
	}
	// Run uses fileId as the filename fallback when a File has no slug.
	return fileID
}

func belongsToWorkerGroup(workerID *string, group string) bool {
	if workerID == nil || *workerID == "" || group == "" {
		return false
	}
	matched, _ := regexp.MatchString(`^`+regexp.QuoteMeta(group)+`(?:@\d+)?$`, *workerID)
	return matched
}

func processLogPath(logDir, slug string) (string, bool) {
	if !safeSlug.MatchString(slug) || filepath.Base(slug) != slug {
		return "", false
	}
	return filepath.Join(logDir, slug+".log"), true
}

func readLogTail(path string, limit int64) ([]byte, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	truncated := info.Size() > limit
	if truncated {
		if _, err := file.Seek(info.Size()-limit, io.SeekStart); err != nil {
			return nil, false, err
		}
	}
	content, err := io.ReadAll(io.LimitReader(file, limit))
	return content, truncated, err
}

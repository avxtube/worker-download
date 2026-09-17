package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeProcessLogBySlug(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), ".log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "Abc_123.log"), []byte("job output"), 0644); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	serveProcessLogBySlug(logDir).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/log/Abc_123.log", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "job output" {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("Content-Type = %q", contentType)
	}
}

func TestServeProcessLogBySlugRejectsTraversal(t *testing.T) {
	logDir := t.TempDir()
	for _, path := range []string{"/log/../secret.log", "/log/a/b.log", "/log/missing-extension"} {
		recorder := httptest.NewRecorder()
		serveProcessLogBySlug(logDir).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code == http.StatusOK {
			t.Errorf("%s unexpectedly returned 200", path)
		}
	}
}

func TestProcessLogPathRejectsTraversal(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), ".log")
	want := filepath.Join(logDir, "Abc_123-test.log")
	if got, ok := processLogPath(logDir, "Abc_123-test"); !ok || got != want {
		t.Fatalf("valid path = %q, %v", got, ok)
	}
	for _, slug := range []string{"../secret", "a/b", `a\\b`, "", "name.log"} {
		if path, ok := processLogPath(logDir, slug); ok {
			t.Errorf("unexpected path for %q: %q", slug, path)
		}
	}
}

func TestReadLogTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job.log")
	if err := os.WriteFile(path, []byte("0123456789"), 0644); err != nil {
		t.Fatal(err)
	}
	content, truncated, err := readLogTail(path, 4)
	if err != nil || !truncated || string(content) != "6789" {
		t.Fatalf("tail=%q truncated=%v err=%v", content, truncated, err)
	}
}

func TestServeLogHistorySortsNewestAndPaginates(t *testing.T) {
	logDir := t.TempDir()
	baseTime := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 25; i++ {
		name := fmt.Sprintf("video-%02d.log", i)
		path := filepath.Join(logDir, name)
		if err := os.WriteFile(path, []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
		stamp := baseTime.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(logDir, "ignore.txt"), []byte("ignored"), 0644); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/log-history?page=1", nil)
	recorder := httptest.NewRecorder()
	serveLogHistory(logDir).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
	var response logHistoryResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Total != 25 || response.TotalPages != 2 || len(response.Items) != 20 {
		t.Fatalf("response = %+v", response)
	}
	if response.Items[0].Name != "video-24.log" || response.Items[19].Name != "video-05.log" {
		t.Fatalf("unexpected order: first=%q last=%q", response.Items[0].Name, response.Items[19].Name)
	}

	request = httptest.NewRequest(http.MethodGet, "/log-history?page=2", nil)
	recorder = httptest.NewRecorder()
	serveLogHistory(logDir).ServeHTTP(recorder, request)
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 5 || response.Items[0].Name != "video-04.log" || response.Items[4].Name != "video-00.log" {
		t.Fatalf("unexpected second page: %+v", response.Items)
	}
}

func TestServeLogHistoryRejectsInvalidPage(t *testing.T) {
	for _, target := range []string{"/log-history?page=0", "/log-history?page=nope"} {
		recorder := httptest.NewRecorder()
		serveLogHistory(t.TempDir()).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d", target, recorder.Code)
		}
	}
}

func TestDashboardLoadsLogsByJobID(t *testing.T) {
	html := string(indexHTML)
	if !strings.Contains(html, "fetch('/logs/'+encodeURIComponent(activeLogId)") {
		t.Fatal("dashboard must load process logs through the job-ID endpoint")
	}
}

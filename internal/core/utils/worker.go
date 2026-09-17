package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"worker-download/internal/core/enums"
)

// ─── Worker ID ───────────────────────────────────────────────

// GenerateWorkerID generates a unique worker ID.
// Priority: WORKER_ID env → download_hostname@1
func GenerateWorkerID() string {
	if envWorkerID := os.Getenv("WORKER_ID"); envWorkerID != "" {
		return envWorkerID
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	return fmt.Sprintf("%s_%s@1", enums.WorkerTypeDownload, hostname)
}

// RandomString generates a random alphanumeric string.
// If special=true, inserts dash and underscore (matches randomString from TS).
func RandomString(n int, special bool) string {
	if special {
		return RandomStringSpecial(n)
	}
	return RandomAlphaNum(n)
}

// ─── Process Logger ──────────────────────────────────────────

// ProcessLogger writes to both the terminal and a per-file log.
type ProcessLogger struct {
	file *os.File
}

// NewProcessLogger creates a per-file logger. During execution every message is
// written to both the terminal and the file-specific log.
// On retry, it appends to the existing log file instead of overwriting.
func NewProcessLogger(logDir, slug string) *ProcessLogger {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("⚠️ Failed to create log dir: %v", err)
		return &ProcessLogger{}
	}

	logPath := filepath.Join(logDir, fmt.Sprintf("%s.log", slug))
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("⚠️ Failed to open process log: %v", err)
		return &ProcessLogger{}
	}

	log.SetOutput(io.MultiWriter(os.Stdout, f))

	return &ProcessLogger{file: f}
}

// Close restores terminal-only logging and closes the per-file log.
func (pl *ProcessLogger) Close() {
	log.SetOutput(os.Stdout)
	if pl.file != nil {
		_ = pl.file.Close()
		pl.file = nil
	}
}

// Printf is kept for compatibility but is a no-op — use log.Printf directly.
func (pl *ProcessLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// LogMain writes a key milestone. Kept as a separate name so milestones stay
// greppable, but it is now a plain log.Printf — NewProcessLogger already tees
// output to stdout and the process file, so the old second Fprintf to
// GlobalWriter would just print the line twice on the console.
func LogMain(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// ─── Old Log Cleanup ──────────────────────────────────────────

// CleanOldLogs removes process log files older than 7 days.
func CleanOldLogs(logDir string) {
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return
	}

	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return
	}

	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(logDir, entry.Name()))
			removed++
		}
	}

	if removed > 0 {
		log.Printf("🧹 Removed %d old log files", removed)
	}
}

// ─── Processing Lock ─────────────────────────────────────────

// ProcessingLock is a mutex-based lock for serializing heavy operations.
type ProcessingLock struct {
	mu   *sync.Mutex
	name string
}

var (
	locksMu sync.Mutex
	locks   = map[string]*sync.Mutex{}
)

// AcquireProcessingLock acquires a named mutex lock (blocking).
// Call Release() when done.
func AcquireProcessingLock(name string) *ProcessingLock {
	locksMu.Lock()
	mu, ok := locks[name]
	if !ok {
		mu = &sync.Mutex{}
		locks[name] = mu
	}
	locksMu.Unlock()

	mu.Lock()
	return &ProcessingLock{mu: mu, name: name}
}

// Release releases the processing lock.
func (l *ProcessingLock) Release() {
	if l.mu != nil {
		l.mu.Unlock()
	}
}

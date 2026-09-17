package queue

import (
	"os"
	"path/filepath"
	"testing"

	"worker-download/internal/config"
)

func TestCleanupTerminalWorkDirRemovesOnlyJobDirectory(t *testing.T) {
	workRoot := t.TempDir()
	previous := config.AppConfig.WorkDir
	config.AppConfig.WorkDir = workRoot
	t.Cleanup(func() { config.AppConfig.WorkDir = previous })

	jobDir := filepath.Join(workRoot, "job-123")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "source.mp4"), []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	neighbor := filepath.Join(workRoot, "keep")
	if err := os.MkdirAll(neighbor, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := cleanupTerminalWorkDir("job-123"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(jobDir); !os.IsNotExist(err) {
		t.Fatalf("terminal job directory still exists: %v", err)
	}
	if _, err := os.Stat(neighbor); err != nil {
		t.Fatalf("neighbor directory was affected: %v", err)
	}
}

func TestTerminalWorkDirRejectsUnsafeJobID(t *testing.T) {
	for _, jobID := range []string{"", ".", "..", "../outside", `..\outside`, "nested/job"} {
		if _, err := terminalWorkDir(t.TempDir(), jobID); err == nil {
			t.Errorf("terminalWorkDir accepted unsafe job ID %q", jobID)
		}
	}
}

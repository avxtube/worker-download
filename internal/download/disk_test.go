package download

import (
	"os"
	"path/filepath"
	"testing"

	"worker-download/internal/db/models"
)

func TestRequiredSplitDiskBytesIncludesOutputAndReserve(t *testing.T) {
	const sourceSize = int64(5 * 1024 * 1024 * 1024)
	want := sourceSize + splitDiskReserve
	if got := requiredSplitDiskBytes(sourceSize); got != want {
		t.Fatalf("requiredSplitDiskBytes() = %d, want %d", got, want)
	}
}

func TestCleanupMergedSegmentsRemovesOnlyValidatedMergeWorkspace(t *testing.T) {
	workDir := t.TempDir()
	segmentsDir := filepath.Join(workDir, "segments")
	if err := os.MkdirAll(segmentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segmentsDir, "segment.ts"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	mergedPath := filepath.Join(workDir, models.FileNameOriginal)
	if err := cleanupMergedSegments(workDir, mergedPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(segmentsDir); !os.IsNotExist(err) {
		t.Fatalf("segments directory still exists: %v", err)
	}
}

func TestCleanupMergedSegmentsKeepsSegmentsForDifferentOutput(t *testing.T) {
	workDir := t.TempDir()
	segmentsDir := filepath.Join(workDir, "segments")
	if err := os.MkdirAll(segmentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cleanupMergedSegments(workDir, filepath.Join(workDir, "source.mp4")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(segmentsDir); err != nil {
		t.Fatalf("segments directory was removed: %v", err)
	}
}

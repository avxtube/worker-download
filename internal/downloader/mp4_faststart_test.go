package downloader

import (
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func writeTestMP4(t *testing.T, atoms ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.mp4")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	for _, atom := range atoms {
		var header [8]byte
		binary.BigEndian.PutUint32(header[:4], 8)
		copy(header[4:], atom)
		if _, err := file.Write(header[:]); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestValidateMoovBeforeMdat(t *testing.T) {
	if err := ValidateMoovBeforeMdat(writeTestMP4(t, "ftyp", "moov", "mdat")); err != nil {
		t.Fatalf("valid faststart MP4 rejected: %v", err)
	}
}

func TestValidateMoovBeforeMdatRejectsTailMoov(t *testing.T) {
	if err := ValidateMoovBeforeMdat(writeTestMP4(t, "ftyp", "mdat", "moov")); err == nil {
		t.Fatal("MP4 with moov after mdat should be rejected")
	}
}

func TestValidateMoovBeforeMdatRejectsMissingAtom(t *testing.T) {
	if err := ValidateMoovBeforeMdat(writeTestMP4(t, "ftyp", "mdat")); err == nil {
		t.Fatal("MP4 without moov should be rejected")
	}
}

func TestEnsureMoovBeforeMdatRepairsMP4(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe is not installed")
	}
	filePath := filepath.Join(t.TempDir(), "tail-moov.mp4")
	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=black:s=64x64:d=1", "-c:v", "mpeg4",
		"-movflags", "-faststart", filePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create test MP4: %v: %s", err, output)
	}
	if err := ValidateMoovBeforeMdat(filePath); err == nil {
		t.Fatal("test fixture unexpectedly has moov before mdat")
	}
	if err := EnsureMoovBeforeMdat(context.Background(), filePath, nil); err != nil {
		t.Fatalf("EnsureMoovBeforeMdat: %v", err)
	}
	if err := ValidateMoovBeforeMdat(filePath); err != nil {
		t.Fatalf("repaired MP4 is not faststart: %v", err)
	}
}

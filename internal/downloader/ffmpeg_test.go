package downloader

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFFmpegProgressSeconds(t *testing.T) {
	tests := []struct {
		line string
		want float64
		ok   bool
	}{
		{line: "out_time=00:01:30.500000", want: 90.5, ok: true},
		{line: "out_time_us=90500000", want: 90.5, ok: true},
		{line: "out_time_ms=90500000", want: 90.5, ok: true},
		{line: "frame=120", ok: false},
		{line: "out_time_us=invalid", ok: false},
	}
	for _, test := range tests {
		got, ok := ffmpegProgressSeconds(test.line)
		if ok != test.ok || (ok && math.Abs(got-test.want) > 0.0001) {
			t.Errorf("ffmpegProgressSeconds(%q) = (%v, %v), want (%v, %v)", test.line, got, ok, test.want, test.ok)
		}
	}
}

func TestFFmpegStatusSeconds(t *testing.T) {
	got, ok := ffmpegStatusSeconds("frame= 10 fps=5.0 time=00:00:12.250 bitrate=1000kbits/s")
	if !ok || math.Abs(got-12.25) > 0.0001 {
		t.Fatalf("ffmpegStatusSeconds() = (%v, %v), want (12.25, true)", got, ok)
	}
}

func TestAACStereoArgs(t *testing.T) {
	want := []string{"-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "48000"}
	if got := aacStereoArgs("128k"); !reflect.DeepEqual(got, want) {
		t.Fatalf("aacStereoArgs() = %v, want %v", got, want)
	}
}

func TestH264CmdNormalizesAudioToStereo48K(t *testing.T) {
	cmd := h264Cmd(context.Background(), "source.mkv", "output.mp4", false, encoderCPU)
	want := []string{"-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "48000"}
	if !containsArgs(cmd.Args, want) {
		t.Fatalf("h264Cmd args = %v, want sequence %v", cmd.Args, want)
	}
}

func TestH264CmdVideoOnlyDropsNonVideoStreams(t *testing.T) {
	cmd := h264Cmd(context.Background(), "source.mkv", "output.mp4", true, encoderCPU)
	want := []string{"-map", "0:v:0", "-an", "-sn", "-dn"}
	if !containsArgs(cmd.Args, want) {
		t.Fatalf("h264Cmd args = %v, want sequence %v", cmd.Args, want)
	}
}

func containsArgs(args, sequence []string) bool {
	for i := 0; i+len(sequence) <= len(args); i++ {
		if reflect.DeepEqual(args[i:i+len(sequence)], sequence) {
			return true
		}
	}
	return false
}

func TestValidateVideoFileMarksRejectedMediaInvalid(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe is not installed")
	}

	path := filepath.Join(t.TempDir(), "source.mp4")
	if err := os.WriteFile(path, []byte("not a video"), 0o600); err != nil {
		t.Fatalf("write invalid source: %v", err)
	}

	err := ValidateVideoFile(path)
	if !errors.Is(err, ErrInvalidVideo) {
		t.Fatalf("ValidateVideoFile() error = %v, want ErrInvalidVideo", err)
	}
}

func TestValidateVideoFileDoesNotMarkToolFailureInvalid(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := ValidateVideoFile(filepath.Join(t.TempDir(), "source.mp4"))
	if err == nil {
		t.Fatal("ValidateVideoFile() error = nil, want tool error")
	}
	if errors.Is(err, ErrInvalidVideo) {
		t.Fatalf("ValidateVideoFile() error = %v, must not be ErrInvalidVideo", err)
	}
}

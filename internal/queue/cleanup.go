package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"worker-download/internal/config"
)

func cleanupTerminalWorkDir(jobID string) error {
	target, err := terminalWorkDir(config.AppConfig.WorkDir, jobID)
	if err != nil {
		return err
	}
	return os.RemoveAll(target)
}

func terminalWorkDir(workRoot, jobID string) (string, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" || jobID == "." || jobID == ".." || strings.ContainsAny(jobID, `/\`) {
		return "", fmt.Errorf("unsafe job ID %q", jobID)
	}
	root, err := filepath.Abs(workRoot)
	if err != nil {
		return "", fmt.Errorf("resolve work root: %w", err)
	}
	target, err := filepath.Abs(filepath.Join(root, jobID))
	if err != nil {
		return "", fmt.Errorf("resolve job work directory: %w", err)
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("job work directory escapes work root")
	}
	return target, nil
}

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Cleanup removes stale worker directories and cleans up build artifacts.
//
// This method scans the build base directory for worker directories (SL.*),
// attempts to unmount any active mounts, and removes the directories.
//
// This method handles all the business logic but does not interact with the user.
// The caller is responsible for:
//   - Displaying progress/status to the user
//   - Confirming destructive operations
//
// Returns CleanupResult containing the number of workers cleaned and any errors.
func (s *Service) Cleanup(opts CleanupOptions) (*CleanupResult, error) {
	result := &CleanupResult{
		WorkersCleaned: 0,
		Errors:         make([]error, 0),
	}

	// Look for worker directories in BuildBase
	baseDir := s.cfg.BuildBase
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read build directory: %w", err)
	}

	workersFound := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Worker directories match pattern "SL.*"
		if strings.HasPrefix(entry.Name(), "SL.") {
			workersFound++
			workerPath := filepath.Join(baseDir, entry.Name())

			// Try to cleanup mounts for this worker
			if err := s.cleanupWorkerMounts(workerPath); err != nil {
				result.Errors = append(result.Errors,
					fmt.Errorf("failed to cleanup %s: %w", entry.Name(), err))
				s.logger.Warn("Failed to cleanup %s: %v", entry.Name(), err)
				continue
			}

			result.WorkersCleaned++
			s.logger.Info("Cleaned up %s", entry.Name())
		}
	}

	if workersFound == 0 {
		s.logger.Info("No worker directories found")
	}

	return result, nil
}

// cleanupWorkerMounts attempts to unmount and remove a worker directory
func (s *Service) cleanupWorkerMounts(workerPath string) error {
	// Common mount points in worker directories
	commonMounts := []string{
		"dev",
		"proc",
		"distfiles",
		"packages",
		"ccache",
		"logs",
		"options",
		"construction",
	}

	// Try to unmount in reverse order
	for i := len(commonMounts) - 1; i >= 0; i-- {
		mountPoint := filepath.Join(workerPath, commonMounts[i])
		// Ignore errors - mount might not exist
		exec.Command("umount", "-f", mountPoint).Run()
	}

	// Try to remove the directory
	return os.RemoveAll(workerPath)
}

// GetWorkerDirectories returns a list of active worker directories.
func (s *Service) GetWorkerDirectories() ([]string, error) {
	baseDir := s.cfg.BuildBase
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read build directory: %w", err)
	}

	workers := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "SL.") {
			workers = append(workers, filepath.Join(baseDir, entry.Name()))
		}
	}

	return workers, nil
}

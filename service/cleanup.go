package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CleanupStaleWorkers removes orphaned worker directories from crashed builds.
//
// This method is for STALE/ORPHANED workers only (no Environment instances exist).
// It uses raw exec.Command() for mount operations since these workers have no
// associated Environment objects.
//
// For ACTIVE workers (during build), use the cleanup function returned by
// build.DoBuild() instead, which properly uses the Environment abstraction.
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
func (s *Service) CleanupStaleWorkers(opts CleanupOptions) (*CleanupResult, error) {
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

		// Worker directories match pattern "SL\d\d" (e.g., SL00, SL01, SL02)
		// Note: Workers are created with fmt.Sprintf("SL%02d", workerID) in environment/bsd/bsd.go
		if strings.HasPrefix(entry.Name(), "SL") && len(entry.Name()) == 4 {
			workersFound++
			workerPath := filepath.Join(baseDir, entry.Name())

			// Try to cleanup mounts for this worker
			if err := s.cleanupWorkerMounts(workerPath); err != nil {
				result.Errors = append(result.Errors,
					fmt.Errorf("failed to cleanup %s: %w", entry.Name(), err))
				if s.logger != nil {
					s.logger.Warn("Failed to cleanup %s: %v", entry.Name(), err)
				}
				continue
			}

			result.WorkersCleaned++
			if s.logger != nil {
				s.logger.Info("Cleaned up %s", entry.Name())
			}
		}
	}

	if workersFound == 0 && s.logger != nil {
		s.logger.Info("No worker directories found")
	}

	return result, nil
}

// cleanupWorkerMounts attempts to unmount and remove a worker directory
func (s *Service) cleanupWorkerMounts(workerPath string) error {
	// Get all active mounts for this worker by parsing mount output
	// This is more reliable than hardcoding mount points since the environment
	// package may add/remove/reorder mounts over time
	cmd := exec.Command("mount")
	output, err := cmd.Output()
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("Failed to get mount list: %v", err)
		}
		// Continue with fallback cleanup
	}

	// Find all mounts under workerPath (must match exactly or be subdirectory)
	var mountPoints []string
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			// Mount format: "device on /path (type, options)"
			parts := strings.Split(line, " on ")
			if len(parts) >= 2 {
				pathParts := strings.Split(parts[1], " (")
				if len(pathParts) >= 1 {
					mountPath := pathParts[0]
					// Check if this mount is under our worker directory
					if strings.HasPrefix(mountPath, workerPath+"/") || mountPath == workerPath {
						mountPoints = append(mountPoints, mountPath)
					}
				}
			}
		}
	}

	// Sort mount points by depth (deepest first) to unmount in correct order
	// This is critical - must unmount nested mounts before parent mounts
	sortByDepthDescending(mountPoints)

	// Unmount all found mounts
	for _, mp := range mountPoints {
		if s.logger != nil {
			s.logger.Debug("Unmounting %s", mp)
		}
		cmd := exec.Command("umount", "-f", mp)
		if err := cmd.Run(); err != nil {
			if s.logger != nil {
				s.logger.Warn("Failed to unmount %s: %v", mp, err)
			}
			// Continue trying other mounts
		}
	}

	// Also try common mount points as fallback (in case mount parsing failed)
	commonMounts := []string{
		"usr/local",
		"construction",
		"options",
		"packages",
		"distfiles",
		"xports",
		"usr/games",
		"usr/share",
		"usr/sbin",
		"usr/libexec",
		"usr/libdata",
		"usr/lib",
		"usr/include",
		"usr/bin",
		"libexec",
		"lib",
		"sbin",
		"bin",
		"proc",
		"dev",
		"boot",
		"", // The base directory itself (tmpfs)
	}

	for _, mp := range commonMounts {
		mountPoint := filepath.Join(workerPath, mp)
		exec.Command("umount", "-f", mountPoint).Run()
		// Ignore errors - mount might not exist
	}

	// Try to remove the directory
	if err := os.RemoveAll(workerPath); err != nil {
		return fmt.Errorf("failed to remove directory: %w", err)
	}

	return nil
}

// sortByDepthDescending sorts paths by depth (deepest first)
// This ensures nested mounts are unmounted before their parents
func sortByDepthDescending(paths []string) {
	// Simple bubble sort by path depth (number of slashes)
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			depthI := strings.Count(paths[i], "/")
			depthJ := strings.Count(paths[j], "/")
			if depthJ > depthI {
				paths[i], paths[j] = paths[j], paths[i]
			}
		}
	}
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
		// Worker directories match pattern "SL\d\d" (e.g., SL00, SL01, SL02)
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "SL") && len(entry.Name()) == 4 {
			workers = append(workers, filepath.Join(baseDir, entry.Name()))
		}
	}

	return workers, nil
}

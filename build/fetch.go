package build

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"

	"dsynth/config"
	"dsynth/pkg"
)

// FetchStats tracks fetch statistics
type FetchStats struct {
	Total   int
	Success int
	Failed  int
}

// DoFetchOnly executes fetch-only mode (download distfiles without building)
func DoFetchOnly(head *pkg.Package, cfg *config.Config) (*FetchStats, error) {
	stats := &FetchStats{}
	var statsMu sync.Mutex

	// Count packages
	for p := head; p != nil; p = p.Next {
		if p.Flags&(pkg.PkgFNotFound|pkg.PkgFCorrupt|pkg.PkgFMeta) == 0 {
			stats.Total++
		}
	}

	fmt.Printf("Fetching distfiles for %d packages...\n", stats.Total)

	// Use worker pool for parallel fetching
	numWorkers := cfg.MaxWorkers
	if numWorkers > 8 {
		numWorkers = 8 // Limit parallelism for fetching
	}

	queue := make(chan *pkg.Package, 100)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for p := range queue {
				success := fetchPackageDistfiles(p, cfg)

				statsMu.Lock()
				if success {
					stats.Success++
					fmt.Printf("[Worker %d] ✓ %s\n", workerID, p.PortDir)
				} else {
					stats.Failed++
					fmt.Printf("[Worker %d] ✗ %s\n", workerID, p.PortDir)
				}
				statsMu.Unlock()
			}
		}(i)
	}

	// Queue packages
	go func() {
		for p := head; p != nil; p = p.Next {
			if p.Flags&(pkg.PkgFNotFound|pkg.PkgFCorrupt|pkg.PkgFMeta) == 0 {
				queue <- p
			}
		}
		close(queue)
	}()

	// Wait for completion
	wg.Wait()

	return stats, nil
}

// fetchPackageDistfiles fetches distfiles for a single package
func fetchPackageDistfiles(p *pkg.Package, cfg *config.Config) bool {
	portPath := filepath.Join(cfg.DPortsPath, p.Category, p.Name)

	args := []string{
		"-C", portPath,
		"DISTDIR=" + cfg.DistFilesPath,
		"BATCH=yes",
		"fetch",
	}

	if p.Flavor != "" {
		args = append(args, "FLAVOR="+p.Flavor)
	}

	cmd := exec.Command("make", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it's just "no distfiles needed"
		if len(output) == 0 {
			return true
		}
		return false
	}

	return true
}

// FetchRecursive fetches distfiles for a package and all its dependencies
func FetchRecursive(p *pkg.Package, cfg *config.Config, fetched map[string]bool) error {
	if fetched[p.PortDir] {
		return nil
	}

	// Fetch dependencies first
	for _, link := range p.IDependOn {
		if err := FetchRecursive(link.Pkg, cfg, fetched); err != nil {
			return err
		}
	}

	// Fetch this package
	if !fetchPackageDistfiles(p, cfg) {
		return fmt.Errorf("failed to fetch distfiles for %s", p.PortDir)
	}

	fetched[p.PortDir] = true
	return nil
}

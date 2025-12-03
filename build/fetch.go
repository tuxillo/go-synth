package build

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"

	"go-synth/config"
	"go-synth/log"
	"go-synth/pkg"
)

// FetchStats tracks fetch statistics
type FetchStats struct {
	Total   int
	Success int
	Failed  int
}

// fetchFunc is a function type for fetching package distfiles
type fetchFunc func(*pkg.Package, *config.Config) bool

// DoFetchOnly executes fetch-only mode (download distfiles without building)
func DoFetchOnly(packages []*pkg.Package, cfg *config.Config, registry *pkg.BuildStateRegistry, logger log.LibraryLogger) (*FetchStats, error) {
	return doFetchOnlyWithFetcher(packages, cfg, registry, logger, fetchPackageDistfiles)
}

// doFetchOnlyWithFetcher allows injection of fetch function for testing
func doFetchOnlyWithFetcher(packages []*pkg.Package, cfg *config.Config, registry *pkg.BuildStateRegistry, logger log.LibraryLogger, fetcher fetchFunc) (*FetchStats, error) {
	stats := &FetchStats{}
	var statsMu sync.Mutex

	// Count packages
	for _, p := range packages {
		if !registry.HasAnyFlags(p, pkg.PkgFNotFound|pkg.PkgFCorrupt|pkg.PkgFMeta) {
			stats.Total++
		}
	}

	logger.Info("Fetching distfiles for %d packages", stats.Total)

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
				success := fetcher(p, cfg)

				statsMu.Lock()
				if success {
					stats.Success++
					logger.Info("Worker %d: fetched %s", workerID, p.PortDir)
				} else {
					stats.Failed++
					logger.Warn("Worker %d: failed to fetch %s", workerID, p.PortDir)
				}
				statsMu.Unlock()
			}
		}(i)
	}

	// Queue packages
	go func() {
		for _, p := range packages {
			if !registry.HasAnyFlags(p, pkg.PkgFNotFound|pkg.PkgFCorrupt|pkg.PkgFMeta) {
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

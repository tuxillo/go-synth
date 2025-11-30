package service

import (
	"fmt"
	"os"
	"time"

	"dsynth/build"
	"dsynth/migration"
	"dsynth/pkg"
)

// Build orchestrates the complete build workflow for the specified ports.
//
// The build process includes:
//  1. Optional migration of legacy CRC data (if enabled and needed)
//  2. Package parsing and dependency resolution
//  3. Marking packages that need building (CRC-based incremental builds)
//  4. Executing the build with worker orchestration
//  5. Cleanup of build environments
//
// This method handles all the business logic but does not interact with the user.
// The caller is responsible for:
//   - Displaying progress/status to the user
//   - Prompting for confirmations
//   - Signal handling (Ctrl+C, etc.)
//
// Returns BuildResult containing stats and package information, or an error if the build fails.
func (s *Service) Build(opts BuildOptions) (*BuildResult, error) {
	startTime := time.Now()

	// Detect and perform migration if needed
	if err := s.detectAndMigrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	// Parse ports and resolve dependencies
	packages, err := s.parseAndResolve(opts.PortList)
	if err != nil {
		return nil, err
	}

	// Mark packages needing build (CRC-based incremental builds)
	needBuild, err := s.markNeedingBuild(packages, opts.Force)
	if err != nil {
		return nil, fmt.Errorf("failed to check build status: %w", err)
	}

	// If nothing to build and not forced, return early
	if needBuild == 0 && !opts.Force {
		return &BuildResult{
			Stats: &build.BuildStats{
				Total:    len(packages),
				Success:  len(packages),
				Skipped:  len(packages),
				Duration: time.Since(startTime),
			},
			Packages:  packages,
			NeedBuild: 0,
			Duration:  time.Since(startTime),
		}, nil
	}

	// Execute the build
	stats, cleanup, err := build.DoBuild(packages, s.cfg, s.logger, s.db)

	if err != nil {
		return nil, fmt.Errorf("build failed: %w", err)
	}

	// Return cleanup function to caller - they control when to call it
	// This allows proper signal handling where the signal handler can call cleanup
	return &BuildResult{
		Stats:     stats,
		Packages:  packages,
		NeedBuild: needBuild,
		Duration:  time.Since(startTime),
		Cleanup:   cleanup, // Return cleanup function for caller to manage
	}, nil
}

// detectAndMigrate checks for legacy CRC data and migrates it if configured and needed.
func (s *Service) detectAndMigrate() error {
	if !s.cfg.Migration.AutoMigrate {
		return nil
	}

	if !migration.DetectMigrationNeeded(s.cfg) {
		return nil
	}

	// Migration is needed and auto-migrate is enabled
	s.logger.Info("Migrating legacy CRC data...")
	if err := migration.MigrateLegacyCRC(s.cfg, s.db, s.logger); err != nil {
		return fmt.Errorf("CRC migration failed: %w", err)
	}
	s.logger.Info("Migration complete")

	return nil
}

// parseAndResolve parses the port list and resolves all dependencies.
func (s *Service) parseAndResolve(portList []string) ([]*pkg.Package, error) {
	if len(portList) == 0 {
		return nil, fmt.Errorf("no ports specified")
	}

	// Create build state registry
	registry := pkg.NewBuildStateRegistry()

	// Create package registry
	pkgRegistry := pkg.NewPackageRegistry()

	// Parse all port specifications
	packages, err := pkg.ParsePortList(portList, s.cfg, registry, pkgRegistry, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to parse port list: %w", err)
	}

	// Resolve dependencies
	if err := pkg.ResolveDependencies(packages, s.cfg, registry, pkgRegistry, s.logger); err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Get all packages from registry (includes all transitive dependencies)
	allPackages := pkgRegistry.AllPackages()

	return allPackages, nil
}

// markNeedingBuild determines which packages need building based on CRC comparison.
//
// If force is true, all packages are marked as needing a build regardless of CRC status.
// Returns the number of packages that need building.
func (s *Service) markNeedingBuild(packages []*pkg.Package, force bool) (int, error) {
	registry := pkg.NewBuildStateRegistry()

	// If force is enabled, all packages need to be rebuilt
	if force {
		// Clear PkgFPackaged flag for all packages to force rebuild
		for _, p := range packages {
			state := registry.Get(p)
			state.Flags = state.Flags.Clear(pkg.PkgFPackaged)
			registry.Set(p, state)
		}
		return len(packages), nil
	}

	// Normal CRC-based check
	needBuild, err := pkg.MarkPackagesNeedingBuild(packages, s.cfg, registry, s.db, s.logger)
	if err != nil {
		return 0, fmt.Errorf("failed to mark packages: %w", err)
	}

	return needBuild, nil
}

// GetBuildPlan returns information about what would be built without actually building.
//
// This is useful for displaying a build plan to the user before executing the build.
func (s *Service) GetBuildPlan(portList []string) (*BuildPlan, error) {
	// Parse and resolve dependencies
	packages, err := s.parseAndResolve(portList)
	if err != nil {
		return nil, err
	}

	// Check which packages need building
	registry := pkg.NewBuildStateRegistry()
	needBuild, err := pkg.MarkPackagesNeedingBuild(packages, s.cfg, registry, s.db, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to check build status: %w", err)
	}

	// Categorize packages
	var toBuild, toSkip []string
	for _, p := range packages {
		flags := registry.GetFlags(p)
		if flags.Has(pkg.PkgFPackaged) {
			toSkip = append(toSkip, p.PortDir)
		} else {
			toBuild = append(toBuild, p.PortDir)
		}
	}

	return &BuildPlan{
		TotalPackages: len(packages),
		ToBuild:       toBuild,
		ToSkip:        toSkip,
		NeedBuild:     needBuild,
	}, nil
}

// BuildPlan contains information about a planned build.
type BuildPlan struct {
	TotalPackages int      // Total number of packages (including dependencies)
	ToBuild       []string // Packages that will be built
	ToSkip        []string // Packages that will be skipped (already built, up-to-date)
	NeedBuild     int      // Number of packages that need building
}

// MigrationStatus returns information about legacy CRC migration.
type MigrationStatus struct {
	Needed     bool   // Whether migration is needed
	LegacyFile string // Path to legacy CRC file (if it exists)
}

// CheckMigrationStatus checks if legacy CRC migration is needed.
func (s *Service) CheckMigrationStatus() (*MigrationStatus, error) {
	needed := migration.DetectMigrationNeeded(s.cfg)
	var legacyFile string
	if needed {
		legacyFile = s.cfg.BuildBase + "/crc_index"
		if _, err := os.Stat(legacyFile); err != nil {
			// File doesn't exist despite detection (race condition?)
			needed = false
		}
	}

	return &MigrationStatus{
		Needed:     needed,
		LegacyFile: legacyFile,
	}, nil
}

// PerformMigration manually triggers legacy CRC migration.
//
// This is useful when the caller wants explicit control over when migration happens,
// rather than relying on auto-migration during Build().
func (s *Service) PerformMigration() error {
	if !migration.DetectMigrationNeeded(s.cfg) {
		return fmt.Errorf("no migration needed")
	}

	s.logger.Info("Starting manual migration of legacy CRC data...")
	if err := migration.MigrateLegacyCRC(s.cfg, s.db, s.logger); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	s.logger.Info("Migration complete")

	return nil
}

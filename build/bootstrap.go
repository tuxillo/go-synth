package build

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"dsynth/builddb"
	"dsynth/config"
	"dsynth/environment"
	"dsynth/log"
	"dsynth/pkg"

	"github.com/google/uuid"
)

// bootstrapPkg builds ports-mgmt/pkg before starting the worker pool.
// This is required because pkg is needed to create .pkg files for all other ports.
//
// The function:
// 1. Finds ports-mgmt/pkg in the package list
// 2. Checks if rebuild is needed via CRC comparison
// 3. If CRC matches, skips build and marks as PkgFSuccess
// 4. If rebuild needed, creates a temporary worker and builds pkg
// 5. Updates CRC and package index on success
//
// Returns:
//   - error: Any error during bootstrap, nil on success
func bootstrapPkg(ctx context.Context, packages []*pkg.Package, cfg *config.Config,
	logger *log.Logger, buildDB *builddb.DB,
	registry *pkg.BuildStateRegistry) error {

	// Step 1: Find ports-mgmt/pkg in the package graph
	var pkgPkg *pkg.Package
	for _, p := range packages {
		if registry.HasFlags(p, pkg.PkgFPkgPkg) {
			pkgPkg = p
			break
		}
	}

	if pkgPkg == nil {
		// No pkg in dependency graph, nothing to bootstrap
		return nil
	}

	logger.Info("Bootstrap phase: checking ports-mgmt/pkg...")

	// Step 2: Compute CRC of pkg port directory
	portPath := filepath.Join(cfg.DPortsPath, pkgPkg.Category, pkgPkg.Name)
	currentCRC, err := builddb.ComputePortCRC(portPath)
	if err != nil {
		// CRC computation failed, rebuild to be safe
		logger.Warn("Failed to compute CRC for ports-mgmt/pkg: %v (will rebuild)", err)
	} else {
		// Step 3: Check if rebuild is needed
		needsBuild, err := buildDB.NeedsBuild(pkgPkg.PortDir, currentCRC)
		if err != nil {
			logger.Warn("Failed to check NeedsBuild for ports-mgmt/pkg: %v (will rebuild)", err)
		} else if !needsBuild {
			// CRC matches, pkg hasn't changed since last successful build
			registry.AddFlags(pkgPkg, pkg.PkgFSuccess|pkg.PkgFPackaged)
			logger.Success("ports-mgmt/pkg (CRC match, using cached package)")
			return nil
		}
	}

	// Step 4: Build pkg (CRC changed or first build)
	logger.Info("Building ports-mgmt/pkg (bootstrap)...")

	// Create temporary worker for bootstrap
	env, err := environment.New("bsd")
	if err != nil {
		return fmt.Errorf("bootstrap: failed to create environment: %w", err)
	}

	// Use slot 99 for bootstrap worker to avoid conflicts
	if err := env.Setup(99, cfg, logger); err != nil {
		env.Cleanup()
		return fmt.Errorf("bootstrap: environment setup failed: %w", err)
	}

	// Ensure cleanup happens
	defer func() {
		if err := env.Cleanup(); err != nil {
			logger.Error(fmt.Sprintf("Bootstrap cleanup failed: %v", err))
		}
	}()

	worker := &Worker{
		ID:     99,
		Env:    env,
		Status: "bootstrap-pkg",
	}

	// Create package logger
	pkgLogger := log.NewPackageLogger(cfg, pkgPkg.PortDir)
	defer pkgLogger.Close()

	// Step 5: Create build record with "running" status
	buildUUID := uuid.New().String()
	buildRecord := &builddb.BuildRecord{
		UUID:      buildUUID,
		PortDir:   pkgPkg.PortDir,
		Version:   pkgPkg.Version,
		Status:    "running",
		StartTime: time.Now(),
	}

	if err := buildDB.SaveRecord(buildRecord); err != nil {
		logger.Warn("Failed to save build record for bootstrap: %v", err)
	}

	// Step 6: Execute build phases
	registry.AddFlags(pkgPkg, pkg.PkgFRunning)

	phases := []string{
		"fetch",
		"checksum",
		"extract",
		"patch",
		"configure",
		"build",
		"stage",
		"package",
	}

	var buildErr error
	for _, phase := range phases {
		logger.Info("  [bootstrap] %s: %s", pkgPkg.PortDir, phase)
		if err := executePhase(ctx, worker, pkgPkg, phase, cfg, registry, pkgLogger); err != nil {
			buildErr = fmt.Errorf("phase %s failed: %w", phase, err)
			break
		}
	}

	registry.ClearFlags(pkgPkg, pkg.PkgFRunning)

	// Step 7: Update build record status
	buildRecord.EndTime = time.Now()
	if buildErr != nil {
		buildRecord.Status = "failed"
		if err := buildDB.UpdateRecordStatus(buildUUID, "failed", buildRecord.EndTime); err != nil {
			logger.Warn("Failed to update build record: %v", err)
		}
		registry.AddFlags(pkgPkg, pkg.PkgFFailed)
		return fmt.Errorf("bootstrap build failed: %w", buildErr)
	}

	buildRecord.Status = "success"
	if err := buildDB.UpdateRecordStatus(buildUUID, "success", buildRecord.EndTime); err != nil {
		logger.Warn("Failed to update build record: %v", err)
	}

	// Step 8: Update CRC and package index
	if err := buildDB.UpdateCRC(pkgPkg.PortDir, currentCRC); err != nil {
		logger.Warn("Failed to update CRC for ports-mgmt/pkg: %v", err)
	}

	if err := buildDB.UpdatePackageIndex(pkgPkg.PortDir, pkgPkg.Version, buildUUID); err != nil {
		logger.Warn("Failed to update package index for ports-mgmt/pkg: %v", err)
	}

	// Step 9: Mark success
	registry.AddFlags(pkgPkg, pkg.PkgFSuccess|pkg.PkgFPackaged)
	logger.Success("ports-mgmt/pkg (bootstrap build succeeded)")

	return nil
}

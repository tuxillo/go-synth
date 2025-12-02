package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"go-synth/builddb"
	"go-synth/config"
	"go-synth/environment"
	"go-synth/log"
	"go-synth/pkg"
)

// bootstrapPkg builds ports-mgmt/pkg before starting the worker pool.
// It returns the final status recorded for ports-mgmt/pkg within the build run.
func bootstrapPkg(ctx context.Context, packages []*pkg.Package, cfg *config.Config,
	logger *log.Logger, buildDB *builddb.DB,
	registry *pkg.BuildStateRegistry, onCleanupReady func(func()), runID string) (string, error) {

	const bootstrapWorkerID = 99

	// Locate ports-mgmt/pkg in the dependency graph
	var pkgPkg *pkg.Package
	for _, p := range packages {
		if p.PortDir == "ports-mgmt/pkg" {
			pkgPkg = p
			break
		}
	}
	if pkgPkg == nil {
		// No pkg in dependency graph, nothing to bootstrap
		return "", nil
	}

	recordPackage := func(status string, start, end time.Time, phase string) string {
		if runID == "" || status == "" {
			return status
		}
		rec := &builddb.RunPackageRecord{
			PortDir:   pkgPkg.PortDir,
			Version:   pkgPkg.Version,
			Status:    status,
			StartTime: start,
			EndTime:   end,
			WorkerID:  bootstrapWorkerID,
			LastPhase: phase,
		}
		if err := buildDB.PutRunPackage(runID, rec); err != nil {
			logger.Warn("Failed to record bootstrap package: %v", err)
		}
		return status
	}

	registry.AddFlags(pkgPkg, pkg.PkgFPkgPkg)
	logger.Info("Bootstrap phase: checking ports-mgmt/pkg...")

	templatePkg := filepath.Join(cfg.BuildBase, "Template/usr/local/sbin/pkg")
	pkgFilePath := filepath.Join(cfg.PackagesPath, "All", pkgPkg.PkgFile)

	// Template already contains pkg and package file exists
	if _, err := os.Stat(templatePkg); err == nil {
		// pkg binary exists in Template
		if _, err := os.Stat(pkgFilePath); err == nil {
			// Package file also exists
			registry.AddFlags(pkgPkg, pkg.PkgFSuccess|pkg.PkgFPackaged)
			logger.Success("ports-mgmt/pkg (already in Template, using existing)")
			now := time.Now()
			return recordPackage(builddb.RunStatusSkipped, now, now, ""), nil
		}
	}

	logger.Info("Bootstrap phase: pkg not in Template or package missing, will build...")

	portPath := filepath.Join(cfg.DPortsPath, pkgPkg.Category, pkgPkg.Name)
	currentCRC, err := builddb.ComputePortCRC(portPath)
	if err != nil {
		logger.Warn("Failed to compute CRC for ports-mgmt/pkg: %v (will rebuild)", err)
	} else {
		needsBuild, err := buildDB.NeedsBuild(pkgPkg.PortDir, currentCRC)
		if err != nil {
			logger.Warn("Failed to check NeedsBuild for ports-mgmt/pkg: %v (will rebuild)", err)
		} else if !needsBuild {
			// Reuse cached package, ensure Template has pkg installed
			registry.AddFlags(pkgPkg, pkg.PkgFSuccess|pkg.PkgFPackaged)
			logger.Success("ports-mgmt/pkg (CRC match, using cached package)")

			if _, err := os.Stat(templatePkg); err != nil {
				logger.Info("Installing cached pkg into Template...")
				cmd := exec.CommandContext(ctx, "tar",
					"--exclude", "+*",
					"--exclude", "*/man/*",
					"-xzpf", pkgFilePath,
					"-C", filepath.Join(cfg.BuildBase, "Template"))
				if output, err := cmd.CombinedOutput(); err != nil {
					return "", fmt.Errorf("bootstrap: failed to install cached pkg into Template: %w (output: %s)", err, string(output))
				}
				logger.Success("ports-mgmt/pkg installed into Template at /usr/local/sbin/pkg")
			}

			now := time.Now()
			return recordPackage(builddb.RunStatusSkipped, now, now, ""), nil
		}
	}

	// Need to build pkg
	logger.Info("Building ports-mgmt/pkg (bootstrap)...")

	env, err := environment.New("bsd")
	if err != nil {
		return "", fmt.Errorf("bootstrap: failed to create environment: %w", err)
	}

	cleanupEnv := func() {
		if err := env.Cleanup(); err != nil {
			logger.Error(fmt.Sprintf("Bootstrap cleanup failed: %v", err))
		}
	}

	if err := env.Setup(bootstrapWorkerID, cfg, logger); err != nil {
		cleanupEnv()
		if setupErr, ok := err.(*environment.ErrSetupFailed); ok && setupErr.Op == "template-copy" {
			templateDir := filepath.Join(cfg.BuildBase, "Template")
			return "", fmt.Errorf("bootstrap: Template directory missing (%s). Run 'go-synth init' to recreate it: %w", templateDir, err)
		}
		return "", fmt.Errorf("bootstrap: environment setup failed: %w", err)
	}

	if onCleanupReady != nil {
		onCleanupReady(func() {
			cleanupEnv()
		})
	}
	defer func() {
		if onCleanupReady != nil {
			onCleanupReady(nil)
		}
		cleanupEnv()
	}()

	worker := &Worker{
		ID:     bootstrapWorkerID,
		Env:    env,
		Status: "bootstrap-pkg",
	}

	pkgLogger := log.NewPackageLogger(cfg, pkgPkg.PortDir)
	defer pkgLogger.Close()

	registry.AddFlags(pkgPkg, pkg.PkgFRunning)

	startTime := time.Now()
	recordPackage(builddb.RunStatusRunning, startTime, time.Time{}, "")

	phases := []string{
		"install-pkgs",
		"check-sanity",
		"fetch-depends",
		"fetch",
		"checksum",
		"extract-depends",
		"extract",
		"patch-depends",
		"patch",
		"build-depends",
		"lib-depends",
		"configure",
		"build",
		"run-depends",
		"stage",
		"check-plist",
		"package",
	}

	var buildErr error
	var lastPhase string
	for _, phase := range phases {
		lastPhase = phase
		registry.SetLastPhase(pkgPkg, phase)
		logger.Info("  [bootstrap] %s: %s", pkgPkg.PortDir, phase)
		if err := executePhase(ctx, worker, pkgPkg, phase, cfg, registry, pkgLogger); err != nil {
			buildErr = fmt.Errorf("phase %s failed: %w", phase, err)
			break
		}
	}

	registry.ClearFlags(pkgPkg, pkg.PkgFRunning)

	if buildErr != nil {
		registry.AddFlags(pkgPkg, pkg.PkgFFailed)
		recordPackage(builddb.RunStatusFailed, startTime, time.Now(), lastPhase)
		return builddb.RunStatusFailed, fmt.Errorf("bootstrap build failed: %w", buildErr)
	}

	// Update CRC now that build succeeded
	if crc, err := builddb.ComputePortCRC(portPath); err != nil {
		logger.Warn("Failed to compute CRC for ports-mgmt/pkg: %v", err)
	} else if err := buildDB.UpdateCRC(pkgPkg.PortDir, crc); err != nil {
		logger.Warn("Failed to update CRC for ports-mgmt/pkg: %v", err)
	}

	registry.AddFlags(pkgPkg, pkg.PkgFSuccess|pkg.PkgFPackaged)
	logger.Success("ports-mgmt/pkg (bootstrap build succeeded)")

	// Install pkg into Template
	templateDir := filepath.Join(cfg.BuildBase, "Template")
	if _, err = os.Stat(templateDir); err != nil {
		return builddb.RunStatusFailed, fmt.Errorf("bootstrap: Template directory not found: %s (%w)", templateDir, err)
	}

	cmd := exec.CommandContext(ctx, "tar",
		"--exclude", "+*",
		"--exclude", "*/man/*",
		"-xzpf", pkgFilePath,
		"-C", templateDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		recordPackage(builddb.RunStatusFailed, startTime, time.Now(), "install-template")
		return builddb.RunStatusFailed, fmt.Errorf("bootstrap: failed to install pkg into Template: %w (output: %s)", err, string(output))
	}

	logger.Success("ports-mgmt/pkg installed into Template at /usr/local/sbin/pkg")
	return recordPackage(builddb.RunStatusSuccess, startTime, time.Now(), ""), nil
}

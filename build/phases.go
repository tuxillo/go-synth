package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dsynth/config"
	"dsynth/log"
	"dsynth/pkg"
)

// executePhase executes a single build phase
func executePhase(worker *Worker, p *pkg.Package, phase string, cfg *config.Config, registry *pkg.BuildStateRegistry, logger *log.PackageLogger) error {
	// Build the make command
	portPath := filepath.Join("/xports", p.Category, p.Name)

	args := []string{
		"-C", portPath,
	}

	// Add flavor if specified
	if p.Flavor != "" {
		args = append(args, "FLAVOR="+p.Flavor)
	}

	// Add common overrides - CRITICAL: Set PORTSDIR to where we mounted ports
	args = append(args,
		"PORTSDIR=/xports",
		"WRKDIRPREFIX=/construction",
		"DISTDIR=/distfiles",
		"PACKAGES=/packages",
		"PKG_DBDIR=/var/db/pkg",
	)

	// Phase-specific handling
	switch phase {
	case "install-pkgs":
		// Install dependency packages before building
		return installDependencyPackages(worker, p, cfg, registry, logger)

	case "fetch":
		args = append(args, "BATCH=yes", "fetch")

	case "checksum":
		args = append(args, "BATCH=yes", "checksum")

	case "extract":
		args = append(args, "BATCH=yes", "extract")

	case "patch":
		args = append(args, "BATCH=yes", "patch")

	case "configure":
		args = append(args, "BATCH=yes", "configure")

	case "build":
		args = append(args, "BATCH=yes", "build")

	case "stage":
		args = append(args, "BATCH=yes", "stage")

	case "package":
		args = append(args, "BATCH=yes", "package")

	case "check-plist":
		if cfg.CheckPlist {
			args = append(args, "check-plist")
		} else {
			// Skip check-plist if not enabled
			return nil
		}

	case "check-sanity":
		args = append(args, "check-sanity")

	case "fetch-depends", "extract-depends", "patch-depends", "build-depends", "lib-depends", "run-depends":
		// These are handled implicitly by the main phases
		return nil

	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}

	// Execute in chroot
	cmd := exec.Command("chroot", worker.Mount.BaseDir, "/usr/bin/make")
	cmd.Args = append([]string{"chroot", worker.Mount.BaseDir, "/usr/bin/make"}, args...)
	cmd.Dir = "/"

	// Capture output
	logger.WriteCommand(strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	logger.Write(output)

	if err != nil {
		return fmt.Errorf("phase failed: %w", err)
	}

	return nil
}

// installDependencyPackages installs required dependency packages
func installDependencyPackages(worker *Worker, p *pkg.Package, cfg *config.Config, registry *pkg.BuildStateRegistry, logger *log.PackageLogger) error {
	// Collect all dependency packages (just the filenames)
	depPkgs := make(map[string]bool)

	for _, link := range p.IDependOn {
		dep := link.Pkg

		// Only install packages that have been built
		if !registry.HasFlags(dep, pkg.PkgFSuccess) {
			continue
		}

		// Skip meta packages
		if registry.HasFlags(dep, pkg.PkgFMeta) {
			continue
		}

		// PkgFile should already be just the filename
		depPkgs[dep.PkgFile] = true
	}

	if len(depPkgs) == 0 {
		return nil
	}

	logger.WritePhase("Installing dependency packages")

	// Install each package from /packages/All/ (C dsynth convention)
	for pkgFile := range depPkgs {
		// Package path inside chroot - use /packages/All/ like C dsynth
		pkgPath := filepath.Join("/packages/All", pkgFile)

		cmd := exec.Command("chroot", worker.Mount.BaseDir, "pkg", "add", pkgPath)
		output, err := cmd.CombinedOutput()
		logger.Write(output)

		if err != nil {
			// Some failures are acceptable (already installed, etc)
			if !strings.Contains(string(output), "already installed") {
				logger.WriteWarning(fmt.Sprintf("Package install warning: %v", err))
			}
		}
	}

	return nil
}

// extractPackage extracts a successfully built package to the repository
func extractPackage(worker *Worker, p *pkg.Package, cfg *config.Config) error {
	// The package was built by make and should be in /packages/All already
	// (since we set PACKAGES=/packages which is mounted from the host)

	// Just verify the package exists
	pkgPath := filepath.Join(cfg.PackagesPath, "All", p.PkgFile)

	if _, err := os.Stat(pkgPath); err != nil {
		return fmt.Errorf("package file not found: %s (%w)", pkgPath, err)
	}

	return nil
}

// copyFile copies a file
func copyFile(src, dst string) error {
	cmd := exec.Command("cp", "-p", src, dst)
	return cmd.Run()
}

// cleanupWorkDir cleans up the work directory after build
func cleanupWorkDir(worker *Worker, p *pkg.Package) error {
	constructionPath := filepath.Join(worker.Mount.BaseDir, "construction")
	workDir := filepath.Join(constructionPath, p.Category, p.Name)

	// Remove work directory
	cmd := exec.Command("rm", "-rf", workDir)
	return cmd.Run()
}

// installMissingPackages installs packages that are missing but required
func installMissingPackages(worker *Worker, requiredPkgs []string, cfg *config.Config, logger *log.PackageLogger) error {
	for _, pkgFile := range requiredPkgs {
		// Use /packages/All/ like C dsynth
		pkgPath := filepath.Join("/packages/All", pkgFile)

		// Check if already installed
		checkCmd := exec.Command("chroot", worker.Mount.BaseDir, "pkg", "info", "-e", pkgFile)
		if err := checkCmd.Run(); err == nil {
			// Already installed
			continue
		}

		// Install package
		cmd := exec.Command("chroot", worker.Mount.BaseDir, "pkg", "add", pkgPath)
		output, err := cmd.CombinedOutput()

		if err != nil {
			logger.WriteWarning(fmt.Sprintf("Failed to install %s: %v\n%s", pkgFile, err, output))
			return fmt.Errorf("failed to install required package %s: %w", pkgFile, err)
		}
	}

	return nil
}

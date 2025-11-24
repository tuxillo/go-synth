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
func executePhase(worker *Worker, p *pkg.Package, phase string, cfg *config.Config, logger *log.PackageLogger) error {
	// Build the make command
	portPath := filepath.Join("/xports", p.Category, p.Name)

	args := []string{
		"-C", portPath,
	}

	// Add flavor if specified
	if p.Flavor != "" {
		args = append(args, "FLAVOR="+p.Flavor)
	}

	// Add common overrides
	args = append(args,
		"WRKDIRPREFIX=/construction",
		"DISTDIR=/distfiles",
		"PACKAGES=/packages",
		"PKG_DBDIR=/var/db/pkg",
	)

	// Phase-specific handling
	switch phase {
	case "install-pkgs":
		// Install dependency packages before building
		return installDependencyPackages(worker, p, cfg, logger)

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
	logger.WriteCommand(cmd.String())
	output, err := cmd.CombinedOutput()
	logger.Write(output)

	if err != nil {
		return fmt.Errorf("phase failed: %w", err)
	}

	return nil
}

// installDependencyPackages installs required dependency packages
func installDependencyPackages(worker *Worker, p *pkg.Package, cfg *config.Config, logger *log.PackageLogger) error {
	// Collect all dependency packages
	depPkgs := make(map[string]bool)

	for _, link := range p.IDependOn {
		dep := link.Pkg

		// Only install packages that have been built
		if dep.Flags&pkg.PkgFSuccess == 0 {
			continue
		}

		// Skip meta packages
		if dep.Flags&pkg.PkgFMeta != 0 {
			continue
		}

		depPkgs[dep.PkgFile] = true
	}

	if len(depPkgs) == 0 {
		return nil
	}

	logger.WritePhase("Installing dependency packages")

	// Install each package
	for pkgFile := range depPkgs {
		pkgPath := filepath.Join("/packages", pkgFile)

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
	// Package was built in /construction
	// Need to copy it to /packages (which is mounted from repository)

	constructionPath := filepath.Join(worker.Mount.BaseDir, "construction")
	workDir := filepath.Join(constructionPath, p.Category, p.Name)
	pkgPath := filepath.Join(workDir, p.PkgFile)

	// Check if package exists
	if _, err := os.Stat(pkgPath); err != nil {
		return fmt.Errorf("package file not found: %w", err)
	}

	// Copy to repository
	destPath := filepath.Join(cfg.RepositoryPath, p.PkgFile)
	if err := copyFile(pkgPath, destPath); err != nil {
		return fmt.Errorf("failed to copy package: %w", err)
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
		pkgPath := filepath.Join("/packages", pkgFile)

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

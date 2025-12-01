package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dsynth/config"
	"dsynth/environment"
	"dsynth/log"
	"dsynth/pkg"
)

// loggerWriter adapts log.PackageLogger to io.Writer interface
type loggerWriter struct {
	logger *log.PackageLogger
}

func (lw *loggerWriter) Write(p []byte) (n int, err error) {
	lw.logger.Write(p)
	return len(p), nil
}

// executePhase executes a single build phase
func executePhase(ctx context.Context, worker *Worker, p *pkg.Package, phase string, cfg *config.Config, registry *pkg.BuildStateRegistry, logger *log.PackageLogger) error {
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
		return installDependencyPackages(ctx, worker, p, cfg, registry, logger)

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

	// Build environment command
	// Create io.Writer adapter for logger
	logWriter := &loggerWriter{logger: logger}

	execCmd := &environment.ExecCommand{
		Command: "/usr/bin/make",
		Args:    args,
		Env: map[string]string{
			"PATH": "/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin",
		},
		Stdout: logWriter,
		Stderr: logWriter,
	}

	// Log command for debugging
	cmdStr := fmt.Sprintf("/usr/bin/make %s", strings.Join(args, " "))
	logger.WriteCommand(cmdStr)

	// Execute in isolated environment
	result, err := worker.Env.Execute(ctx, execCmd)
	if err != nil {
		// Execution failure (not non-zero exit code)
		return fmt.Errorf("phase execution failed: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("phase failed with exit code %d", result.ExitCode)
	}

	return nil
}

// installDependencyPackages installs required dependency packages
func installDependencyPackages(ctx context.Context, worker *Worker, p *pkg.Package, cfg *config.Config, registry *pkg.BuildStateRegistry, logger *log.PackageLogger) error {
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

		logWriter := &loggerWriter{logger: logger}
		execCmd := &environment.ExecCommand{
			Command: "/usr/sbin/pkg",
			Args:    []string{"add", pkgPath},
			Stdout:  logWriter,
			Stderr:  logWriter,
		}

		result, err := worker.Env.Execute(ctx, execCmd)
		if err != nil {
			return fmt.Errorf("failed to install dependency %s: %w", pkgFile, err)
		}

		if result.ExitCode != 0 {
			return fmt.Errorf("failed to install dependency %s: exit code %d", pkgFile, result.ExitCode)
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
	constructionPath := filepath.Join(worker.Env.GetBasePath(), "construction")
	workDir := filepath.Join(constructionPath, p.Category, p.Name)

	// Remove work directory
	cmd := exec.Command("rm", "-rf", workDir)
	return cmd.Run()
}

// installMissingPackages installs packages that are missing but required
func installMissingPackages(ctx context.Context, worker *Worker, requiredPkgs []string, cfg *config.Config, logger *log.PackageLogger) error {
	for _, pkgFile := range requiredPkgs {
		// Use /packages/All/ like C dsynth
		pkgPath := filepath.Join("/packages/All", pkgFile)

		// Check if already installed
		checkCmd := &environment.ExecCommand{
			Command: "/usr/sbin/pkg",
			Args:    []string{"info", "-e", pkgFile},
		}

		checkResult, err := worker.Env.Execute(ctx, checkCmd)
		if err == nil && checkResult.ExitCode == 0 {
			// Already installed
			continue
		}

		// Install package
		logWriter := &loggerWriter{logger: logger}
		installCmd := &environment.ExecCommand{
			Command: "/usr/sbin/pkg",
			Args:    []string{"add", pkgPath},
			Stdout:  logWriter,
			Stderr:  logWriter,
		}

		result, err := worker.Env.Execute(ctx, installCmd)
		if err != nil {
			logger.WriteWarning(fmt.Sprintf("Failed to install %s: execution error: %v", pkgFile, err))
			return fmt.Errorf("failed to install required package %s: %w", pkgFile, err)
		}

		if result.ExitCode != 0 {
			logger.WriteWarning(fmt.Sprintf("Failed to install %s: exit code %d", pkgFile, result.ExitCode))
			return fmt.Errorf("failed to install required package %s: exit code %d", pkgFile, result.ExitCode)
		}
	}

	return nil
}

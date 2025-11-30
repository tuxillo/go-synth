package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dsynth/builddb"
	"dsynth/migration"
)

// Initialize sets up the dsynth environment for the first time.
//
// This method creates all necessary directories, sets up the build template,
// initializes the build database, and optionally migrates legacy CRC data.
//
// The initialization process includes:
//  1. Creating required directory structure (build base, logs, packages, etc.)
//  2. Setting up the template directory with essential system files
//  3. Initializing the build database
//  4. Optionally migrating legacy CRC data
//  5. Verifying the ports directory
//
// This method handles all the business logic but does not interact with the user.
// The caller is responsible for:
//   - Displaying progress/status to the user
//   - Prompting for confirmations (e.g., migration)
//   - Handling errors and warnings
//
// Returns InitResult containing information about what was created and any warnings.
func (s *Service) Initialize(opts InitOptions) (*InitResult, error) {
	result := &InitResult{
		DirsCreated: make([]string, 0),
		Warnings:    make([]string, 0),
	}

	// 1. Create required directories
	dirs := map[string]string{
		"Build base":   s.cfg.BuildBase,
		"Logs":         s.cfg.LogsPath,
		"Ports":        s.cfg.DPortsPath,
		"Repository":   s.cfg.RepositoryPath,
		"Packages":     s.cfg.PackagesPath,
		"Distribution": s.cfg.DistFilesPath,
		"Options":      s.cfg.OptionsPath,
	}

	for label, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create %s directory (%s): %w", label, dir, err)
		}
		result.DirsCreated = append(result.DirsCreated, dir)
		s.logger.Info("Created %s: %s", label, dir)
	}

	// 2. Create template directory with essential files
	if err := s.createTemplate(result, opts.SkipSystemFiles); err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}
	result.TemplateCreated = true

	// 3. Initialize BuildDB (already initialized in NewService, but verify it works)
	if s.db != nil {
		result.DatabaseInitalized = true
		s.logger.Info("Database initialized: %s", s.cfg.Database.Path)
	} else {
		return nil, fmt.Errorf("database not initialized")
	}

	// 4. Check for legacy CRC migration
	if migration.DetectMigrationNeeded(s.cfg) {
		result.MigrationNeeded = true
		if opts.AutoMigrate {
			s.logger.Info("Migrating legacy CRC data...")
			if err := migration.MigrateLegacyCRC(s.cfg, s.db, s.logger); err != nil {
				return nil, fmt.Errorf("migration failed: %w", err)
			}
			result.MigrationPerformed = true
			s.logger.Info("Migration complete")
		}
	}

	// 5. Verify ports directory
	portsCount, err := s.verifyPortsDirectory()
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Ports directory verification failed: %v", err))
	} else {
		result.PortsFound = portsCount
		if portsCount == 0 {
			result.Warnings = append(result.Warnings, "Ports directory is empty")
		}
	}

	return result, nil
}

// createTemplate creates the build template directory with essential system files.
func (s *Service) createTemplate(result *InitResult, skipSystemFiles bool) error {
	templateDir := filepath.Join(s.cfg.BuildBase, "Template")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return fmt.Errorf("failed to create template directory: %w", err)
	}

	// Create necessary directory structure in template
	templateDirs := []string{
		"etc",
		"var/run",
		"var/db",
		"tmp",
	}
	for _, dir := range templateDirs {
		fullPath := filepath.Join(templateDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("failed to create template /%s directory: %w", dir, err)
		}
	}

	// Skip system file copying if requested (for testing)
	if skipSystemFiles {
		s.logger.Info("Created template: %s (system files skipped)", templateDir)
		return nil
	}

	// Copy ld-elf.so.hints from host (needed for dynamic linker)
	hintsSrc := "/var/run/ld-elf.so.hints"
	hintsDst := filepath.Join(templateDir, "var/run/ld-elf.so.hints")
	if err := exec.Command("cp", hintsSrc, hintsDst).Run(); err != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Failed to copy ld-elf.so.hints: %v (some ports may fail during installation)", err))
	}

	etcDir := filepath.Join(templateDir, "etc")

	// Copy essential /etc files for chroot functionality
	etcFiles := []string{
		"resolv.conf",   // DNS resolution
		"passwd",        // User database (needed for mtree, chown, etc)
		"group",         // Group database
		"master.passwd", // Password database
		"pwd.db",        // Password database (Berkeley DB format)
		"spwd.db",       // Secure password database
	}

	for _, file := range etcFiles {
		src := filepath.Join("/etc", file)
		dst := filepath.Join(etcDir, file)
		if err := exec.Command("cp", src, dst).Run(); err != nil {
			// Only warn for resolv.conf, fail for critical files
			if file == "resolv.conf" {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Failed to copy %s: %v (DNS resolution may not work in chroot)", file, err))
			} else {
				return fmt.Errorf("failed to copy /etc/%s (required for build operations): %w", file, err)
			}
		}
	}

	s.logger.Info("Created template: %s (with /etc files)", templateDir)
	return nil
}

// verifyPortsDirectory checks if the ports directory exists and has content.
func (s *Service) verifyPortsDirectory() (int, error) {
	if _, err := os.Stat(s.cfg.DPortsPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("ports directory does not exist: %s", s.cfg.DPortsPath)
	}

	entries, err := os.ReadDir(s.cfg.DPortsPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read ports directory: %w", err)
	}

	return len(entries), nil
}

// NeedsMigration checks if legacy CRC migration is needed without initializing anything.
func (s *Service) NeedsMigration() bool {
	return migration.DetectMigrationNeeded(s.cfg)
}

// GetLegacyCRCFile returns the path to the legacy CRC file if it exists.
func (s *Service) GetLegacyCRCFile() (string, error) {
	legacyFile := filepath.Join(s.cfg.BuildBase, "crc_index")
	if _, err := os.Stat(legacyFile); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return legacyFile, nil
}

// InitDatabase explicitly initializes just the database without full initialization.
// This is useful for commands that need the database but don't need full init.
func InitDatabase(dbPath string) (*builddb.DB, error) {
	return builddb.OpenDB(dbPath)
}

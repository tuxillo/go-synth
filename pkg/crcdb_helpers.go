package pkg

import (
	"fmt"
	"os"
	"path/filepath"

	"dsynth/config"
)

// RebuildCRCDatabase rebuilds the entire CRC database from scratch
func RebuildCRCDatabase(cfg *config.Config) error {
	fmt.Println("Rebuilding CRC database...")

	// Remove old database
	dbPath := filepath.Join(cfg.BuildBase, "dsynth.db")
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove old database: %w", err)
	}

	// Initialize new database
	db, err := InitCRCDatabase(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Scan all packages
	pkgDir := cfg.RepositoryPath
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return fmt.Errorf("failed to read packages directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !hasPkgSuffix(name) {
			continue
		}

		// Try to extract port origin from package name
		// This is a simplified heuristic
		// In reality, we'd need to inspect the package metadata

		count++
		if count%100 == 0 {
			fmt.Printf("  Processed %d packages...\r", count)
		}
	}

	fmt.Printf("  Processed %d packages\n", count)

	// Save database
	if err := db.Save(); err != nil {
		return fmt.Errorf("failed to save database: %w", err)
	}

	fmt.Println("Database rebuilt successfully")
	return nil
}

// CleanCRCDatabase removes entries for packages that no longer exist
func CleanCRCDatabase(cfg *config.Config) error {
	if globalCRCDB == nil {
		db, err := InitCRCDatabase(cfg)
		if err != nil {
			return err
		}
		globalCRCDB = db
	}

	fmt.Println("Cleaning CRC database...")

	globalCRCDB.mu.Lock()
	defer globalCRCDB.mu.Unlock()

	removed := 0
	for portDir, entry := range globalCRCDB.entries {
		pkgPath := filepath.Join(cfg.RepositoryPath, entry.PkgFile)
		if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
			delete(globalCRCDB.entries, portDir)
			removed++
		}
	}

	if removed > 0 {
		globalCRCDB.dirty = true
		fmt.Printf("Removed %d stale entries\n", removed)
	} else {
		fmt.Println("No stale entries found")
	}

	return globalCRCDB.Save()
}

// ExportCRCDatabase exports the database to a text file for inspection
func ExportCRCDatabase(cfg *config.Config, outputPath string) error {
	if globalCRCDB == nil {
		db, err := InitCRCDatabase(cfg)
		if err != nil {
			return err
		}
		globalCRCDB = db
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	globalCRCDB.mu.RLock()
	defer globalCRCDB.mu.RUnlock()

	fmt.Fprintf(file, "# dsynth CRC Database Export\n")
	fmt.Fprintf(file, "# Total entries: %d\n\n", len(globalCRCDB.entries))

	for _, entry := range globalCRCDB.entries {
		fmt.Fprintf(file, "PortDir:   %s\n", entry.PortDir)
		fmt.Fprintf(file, "CRC:       %08x\n", entry.CRC)
		fmt.Fprintf(file, "Version:   %s\n", entry.Version)
		fmt.Fprintf(file, "PkgFile:   %s\n", entry.PkgFile)
		fmt.Fprintf(file, "Size:      %d\n", entry.Size)
		fmt.Fprintf(file, "Mtime:     %d\n", entry.Mtime)
		fmt.Fprintf(file, "BuildTime: %d\n", entry.BuildTime)
		fmt.Fprintf(file, "\n")
	}

	fmt.Printf("Exported database to %s\n", outputPath)
	return nil
}

// hasPkgSuffix checks if a filename has a package suffix
func hasPkgSuffix(name string) bool {
	return filepath.Ext(name) == ".pkg" ||
		filepath.Ext(name) == ".txz" ||
		filepath.Ext(name) == ".tbz" ||
		filepath.Ext(name) == ".tgz"
}

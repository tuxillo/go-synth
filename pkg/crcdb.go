// Add to existing crcdb.go

// DeleteCRCEntry removes an entry from the CRC database
func DeleteCRCEntry(portDir string) {
	if globalCRCDB != nil {
		globalCRCDB.Delete(portDir)
	}
}

// GetCRCStats returns statistics about the CRC database
func GetCRCStats() (total int, modified int) {
	if globalCRCDB == nil {
		return 0, 0
	}
	return globalCRCDB.Stats()
}

// VerifyPackageIntegrity checks if packages match CRC database
func VerifyPackageIntegrity(cfg *config.Config) error {
	if globalCRCDB == nil {
		return fmt.Errorf("CRC database not initialized")
	}

	fmt.Println("Verifying package integrity...")

	globalCRCDB.mu.RLock()
	entries := make([]*CRCEntry, 0, len(globalCRCDB.entries))
	for _, entry := range globalCRCDB.entries {
		entries = append(entries, entry)
	}
	globalCRCDB.mu.RUnlock()

	missing := 0
	corrupted := 0

	for i, entry := range entries {
		if i%100 == 0 {
			fmt.Printf("  Checked %d/%d packages...\r", i, len(entries))
		}

		pkgPath := filepath.Join(cfg.RepositoryPath, entry.PkgFile)
		info, err := os.Stat(pkgPath)
		if os.IsNotExist(err) {
			missing++
			fmt.Printf("\n  Missing: %s\n", entry.PortDir)
			continue
		}

		if info.Size() != entry.Size {
			corrupted++
			fmt.Printf("\n  Size mismatch: %s (expected %d, got %d)\n",
				entry.PortDir, entry.Size, info.Size())
		}
	}

	fmt.Printf("  Checked %d packages\n", len(entries))
	if missing > 0 {
		fmt.Printf("  %d packages missing\n", missing)
	}
	if corrupted > 0 {
		fmt.Printf("  %d packages corrupted\n", corrupted)
	}

	if missing == 0 && corrupted == 0 {
		fmt.Println("  All packages verified successfully")
	}

	return nil
}
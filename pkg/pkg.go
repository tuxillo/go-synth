// Add to existing pkg.go

// MarkPackagesNeedingBuild analyzes which packages need rebuilding
func MarkPackagesNeedingBuild(head *Package, cfg *config.Config) (int, error) {
	// Initialize CRC database
	crcDB, err := InitCRCDatabase(cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to initialize CRC database: %w", err)
	}

	fmt.Println("\nChecking which packages need rebuilding...")

	needBuild := 0
	total := 0

	for pkg := head; pkg != nil; pkg = pkg.Next {
		total++

		// Skip packages marked with errors
		if pkg.Flags&(PkgFNotFound|PkgFCorrupt) != 0 {
			pkg.Flags |= PkgFNoBuildIgnore
			continue
		}

		// Skip meta packages
		if pkg.Flags&PkgFMeta != 0 {
			pkg.Flags |= PkgFSuccess // Don't build metaports
			continue
		}

		// Check if build is needed
		if crcDB.CheckNeedsBuild(pkg, cfg) {
			needBuild++
		} else {
			// Mark as already successful (no build needed)
			pkg.Flags |= PkgFSuccess | PkgFPackaged
			fmt.Printf("  %s: up-to-date\n", pkg.PortDir)
		}

		if total%100 == 0 {
			fmt.Printf("  Checked %d packages...\r", total)
		}
	}

	fmt.Printf("  Checked %d packages\n", total)
	fmt.Printf("  %d packages need building\n", needBuild)
	fmt.Printf("  %d packages are up-to-date\n", total-needBuild)

	return needBuild, nil
}

// SaveCRCDatabase saves the CRC database after builds
func SaveCRCDatabase() error {
	if globalCRCDB == nil {
		return nil
	}
	return globalCRCDB.Save()
}

// UpdateCRCAfterBuild updates CRC database for a successfully built package
func UpdateCRCAfterBuild(pkg *Package) {
	if globalCRCDB != nil {
		globalCRCDB.UpdateAfterBuild(pkg)
	}
}
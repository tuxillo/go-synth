// Add missing doCleanup function
func doCleanup(cfg *config.Config) {
	fmt.Println("Cleaning up build environment...")
	
	// Clean up worker directories
	for i := 0; i < cfg.MaxWorkers; i++ {
		workerDir := fmt.Sprintf("%s/SL%02d", cfg.BuildBase, i)
		if _, err := os.Stat(workerDir); err == nil {
			fmt.Printf("  Removing worker %d...\n", i)
			util.RemoveAll(workerDir)
		}
	}
	
	// Clean construction directories
	constructionPattern := filepath.Join(cfg.BuildBase, "construction.*")
	matches, _ := filepath.Glob(constructionPattern)
	for _, dir := range matches {
		fmt.Printf("  Removing %s...\n", filepath.Base(dir))
		util.RemoveAll(dir)
	}
	
	fmt.Println("Cleanup complete")
}

// Add missing doRebuildRepo function
func doRebuildRepo(cfg *config.Config) {
	fmt.Println("Rebuilding package repository...")
	
	// Use pkg repo to rebuild
	cmd := exec.Command("pkg", "repo", cfg.RepositoryPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: pkg repo failed: %v\n", err)
	} else {
		fmt.Println("Repository rebuilt successfully")
	}
}
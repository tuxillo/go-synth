package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"dsynth/build"
	"dsynth/config"
	"dsynth/log"
	"dsynth/pkg"
	"dsynth/util"
)

var Version = "2.0.0"

func main() {
	// Parse global flags
	var (
		debug        = flag.Bool("d", false, "Debug verbosity")
		force        = flag.Bool("f", false, "Force operations")
		yesAll       = flag.Bool("y", false, "Answer yes to all prompts")
		_            = flag.String("m", "", "Package dependency memory target (GB)") // Reserved for future use
		profile      = flag.String("p", "", "Override profile selection")
		slowStart    = flag.Int("s", 0, "Initial worker count (slow start)")
		configDir    = flag.String("C", "", "Config base directory")
		devMode      = flag.Bool("D", false, "Developer mode")
		checkPlist   = flag.Bool("P", false, "Check plist")
		disableUI    = flag.Bool("S", false, "Disable ncurses")
		niceValue    = flag.Int("N", 0, "Nice value")
	)

	flag.Parse()

	// Set nice value if specified
	if *niceValue != 0 {
		if err := util.SetNice(*niceValue); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set nice value: %v\n", err)
		}
	}

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	command := args[0]
	commandArgs := args[1:]

	// Load configuration
	cfg, err := config.LoadConfig(*configDir, *profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Apply command-line overrides
	if *debug {
		cfg.Debug = true
	}
	if *force {
		cfg.Force = true
	}
	if *yesAll {
		cfg.YesAll = true
	}
	if *slowStart > 0 {
		cfg.SlowStart = *slowStart
	}
	if *devMode {
		cfg.DevMode = true
	}
	if *checkPlist {
		cfg.CheckPlist = true
	}
	if *disableUI {
		cfg.DisableUI = true
	}

	// Execute command
	switch command {
	case "init":
		doInit(cfg)
	case "status":
		doStatus(cfg, commandArgs)
	case "cleanup":
		doCleanup(cfg)
	case "configure":
		doConfigure(cfg)
	case "upgrade-system":
		doUpgradeSystem(cfg)
	case "prepare-system":
		doPrepareSystem(cfg)
	case "rebuild-repository":
		doRebuildRepo(cfg)
	case "purge-distfiles":
		doPurgeDistfiles(cfg)
	case "reset-db":
		doResetDB(cfg)
	case "verify":
		doVerify(cfg)
	case "status-everything":
		doStatusEverything(cfg)
	case "everything":
		doEverything(cfg)
	case "version":
		fmt.Printf("dsynth version %s\n", Version)
	case "help":
		usage()
	case "build":
		doBuild(cfg, commandArgs, false, false)
	case "just-build":
		doBuild(cfg, commandArgs, true, false)
	case "force":
		cfg.Force = true
		doBuild(cfg, commandArgs, false, false)
	case "test":
		doBuild(cfg, commandArgs, false, true)
	case "fetch-only":
		doFetchOnly(cfg, commandArgs)
	case "logs":
		doLogs(cfg, commandArgs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Printf("dsynth version %s - DragonFly BSD ports build system\n\n", Version)
	fmt.Println("Usage: dsynth [options] command [args]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -d            Debug verbosity")
	fmt.Println("  -f            Force operations")
	fmt.Println("  -y            Answer yes to all prompts")
	fmt.Println("  -m GB         Package dependency memory target")
	fmt.Println("  -p profile    Override profile selection")
	fmt.Println("  -s N          Initial worker count (slow start)")
	fmt.Println("  -C dir        Config base directory")
	fmt.Println("  -D            Developer mode")
	fmt.Println("  -P            Check plist")
	fmt.Println("  -S            Disable ncurses")
	fmt.Println("  -N val        Nice value")
	fmt.Println()
	fmt.Println("Build Commands:")
	fmt.Println("  init                     Initialize configuration")
	fmt.Println("  build [ports...]         Build specified ports with dependencies")
	fmt.Println("  just-build [ports...]    Build without repo metadata update")
	fmt.Println("  everything               Build entire ports tree")
	fmt.Println("  upgrade-system           Build all installed packages")
	fmt.Println("  prepare-system           Build for system upgrade")
	fmt.Println("  force [ports...]         Force rebuild specified ports")
	fmt.Println("  fetch-only [ports...]    Download distfiles only")
	fmt.Println()
	fmt.Println("Management Commands:")
	fmt.Println("  status [ports...]        Show build status")
	fmt.Println("  cleanup                  Clean up build environment")
	fmt.Println("  reset-db                 Reset CRC database")
	fmt.Println("  verify                   Verify package integrity")
	fmt.Println("  purge-distfiles          Remove obsolete distfiles")
	fmt.Println("  logs [logfile]           View build logs")
	fmt.Println("  version                  Show version")
	fmt.Println("  help                     Show this help")
	fmt.Println()
	fmt.Println("Port Specification:")
	fmt.Println("  category/portname        Standard port")
	fmt.Println("  category/portname@flavor Port with flavor")
	fmt.Println("  /path/to/port           Absolute path")
	fmt.Println()
}

func doInit(cfg *config.Config) {
	fmt.Println("Initializing dsynth configuration...")

	// Create configuration directories
	configDirs := []string{
		cfg.ConfigPath,
		cfg.BuildBase,
		cfg.SystemPath,
		cfg.RepositoryPath,
		cfg.DistFilesPath,
		cfg.OptionsPath,
		cfg.LogsPath,
	}

	for _, dir := range configDirs {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", dir, err)
			continue
		}
		fmt.Printf("  Created: %s\n", dir)
	}

	// Create template directory
	templateDir := cfg.BuildBase + "/Template"
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating template directory: %v\n", err)
	} else {
		fmt.Printf("  Created: %s\n", templateDir)
	}

	// Write default configuration if it doesn't exist
	configFile := cfg.ConfigPath + "/dsynth.ini"
	if !util.FileExists(configFile) {
		if err := config.WriteDefaultConfig(configFile, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
			return
		}
		fmt.Printf("  Created: %s\n", configFile)
	} else {
		fmt.Printf("  Exists: %s\n", configFile)
	}

	fmt.Println()
	fmt.Println("Configuration initialized successfully!")
	fmt.Println()
	fmt.Printf("Edit %s to customize settings\n", configFile)
}

func doStatus(cfg *config.Config, args []string) {
	fmt.Println("Status check not yet implemented")
	// TODO: Implement status checking
}

func doCleanup(cfg *config.Config) {
	fmt.Println("Cleaning up build environment...")

	// Clean up worker directories
	cleaned := 0
	for i := 0; i < cfg.MaxWorkers; i++ {
		workerDir := fmt.Sprintf("%s/SL%02d", cfg.BuildBase, i)
		if _, err := os.Stat(workerDir); err == nil {
			fmt.Printf("  Removing worker %d...\n", i)
			if err := util.RemoveAll(workerDir); err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: %v\n", err)
			} else {
				cleaned++
			}
		}
	}

	// Clean construction directories
	constructionPattern := cfg.BuildBase + "/construction.*"
	matches, _ := util.Glob(constructionPattern)
	for _, dir := range matches {
		fmt.Printf("  Removing %s...\n", dir)
		if err := util.RemoveAll(dir); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: %v\n", err)
		} else {
			cleaned++
		}
	}

	if cleaned > 0 {
		fmt.Printf("\nCleaned up %d directories\n", cleaned)
	} else {
		fmt.Println("\nNothing to clean")
	}
	fmt.Println("Cleanup complete")
}

func doConfigure(cfg *config.Config) {
	fmt.Println("Interactive configuration not yet implemented")
	fmt.Printf("Edit %s/dsynth.ini to configure\n", cfg.ConfigPath)
}

func doUpgradeSystem(cfg *config.Config) {
	fmt.Println("Detecting installed packages...")

	// Get list of installed packages
	pkgs, err := pkg.GetInstalledPackages(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting installed packages: %v\n", err)
		os.Exit(1)
	}

	if len(pkgs) == 0 {
		fmt.Println("No packages installed")
		return
	}

	fmt.Printf("Found %d installed packages\n", len(pkgs))

	// Build the packages
	doBuild(cfg, pkgs, false, false)
}

func doPrepareSystem(cfg *config.Config) {
	fmt.Println("Prepare system not yet implemented")
	// TODO: Implement prepare-system
}

func doRebuildRepo(cfg *config.Config) {
	fmt.Println("Rebuilding package repository...")

	// Use pkg repo to rebuild
	if err := util.RunCommand("pkg", "repo", cfg.RepositoryPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: pkg repo failed: %v\n", err)
	} else {
		fmt.Println("Repository rebuilt successfully")
	}
}

func doPurgeDistfiles(cfg *config.Config) {
	fmt.Println("Purge distfiles not yet implemented")
	// TODO: Implement purge-distfiles
}

func doResetDB(cfg *config.Config) {
	fmt.Println("Resetting CRC database...")

	dbPath := cfg.BuildBase + "/dsynth.db"
	if util.FileExists(dbPath) {
		if err := os.Remove(dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing database: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Database reset successfully")
	} else {
		fmt.Println("No database found")
	}
}

func doVerify(cfg *config.Config) {
	fmt.Println("Verifying packages...")
	if err := pkg.VerifyPackageIntegrity(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error verifying packages: %v\n", err)
		os.Exit(1)
	}
}

func doStatusEverything(cfg *config.Config) {
	fmt.Println("Status everything not yet implemented")
	// TODO: Implement status-everything
}

func doEverything(cfg *config.Config) {
	fmt.Println("Building everything...")

	// Get all ports from the ports tree
	portList, err := pkg.GetAllPorts(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting ports list: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d ports in tree\n", len(portList))

	// Build all ports
	doBuild(cfg, portList, false, false)
}

func doBuild(cfg *config.Config, portList []string, justBuild bool, testMode bool) {
	if len(portList) == 0 {
		fmt.Println("No ports specified")
		return
	}

	// Initialize logger
	logger, err := log.NewLogger(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	fmt.Printf("Building %d port(s)...\n", len(portList))

	// Parse port specifications into package list
	head, err := pkg.ParsePortList(portList, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing port list: %v\n", err)
		os.Exit(1)
	}

	// Resolve all dependencies
	if err := pkg.ResolveDependencies(head, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving dependencies: %v\n", err)
		os.Exit(1)
	}

	// Check which packages need building (CRC-based)
	needBuild, err := pkg.MarkPackagesNeedingBuild(head, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking build status: %v\n", err)
		os.Exit(1)
	}

	if needBuild == 0 {
		fmt.Println("All packages are up-to-date!")
		return
	}

	// Confirm build
	if !cfg.YesAll {
		if !util.AskYN(fmt.Sprintf("Build %d packages?", needBuild), true) {
			fmt.Println("Build cancelled")
			return
		}
	}

	// Execute build
	stats, err := build.DoBuild(head, cfg, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Build error: %v\n", err)
		os.Exit(1)
	}

	// Print statistics
	fmt.Println()
	fmt.Println("Build Statistics:")
	fmt.Printf("  Total packages: %d\n", stats.Total)
	fmt.Printf("  Success: %d\n", stats.Success)
	fmt.Printf("  Failed: %d\n", stats.Failed)
	fmt.Printf("  Skipped: %d\n", stats.Skipped)
	fmt.Printf("  Ignored: %d\n", stats.Ignored)
	fmt.Printf("  Duration: %s\n", stats.Duration)

	// Update CRC database
	if err := pkg.SaveCRCDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save CRC database: %v\n", err)
	}

	// Rebuild repository metadata unless just-build
	if !justBuild && stats.Success > 0 {
		doRebuildRepo(cfg)
	}

	if stats.Failed > 0 {
		os.Exit(1)
	}
}

func doFetchOnly(cfg *config.Config, portList []string) {
	if len(portList) == 0 {
		fmt.Println("No ports specified")
		return
	}

	fmt.Printf("Fetching distfiles for %d port(s)...\n", len(portList))

	// Parse port specifications
	head, err := pkg.ParsePortList(portList, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing port list: %v\n", err)
		os.Exit(1)
	}

	// Resolve dependencies
	if err := pkg.ResolveDependencies(head, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving dependencies: %v\n", err)
		os.Exit(1)
	}

	// Execute fetch
	stats, err := build.DoFetchOnly(head, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fetch error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Fetched %d distfile(s)\n", stats.Success)
	if stats.Failed > 0 {
		fmt.Printf("Failed %d distfile(s)\n", stats.Failed)
		os.Exit(1)
	}
}

func doLogs(cfg *config.Config, args []string) {
	if len(args) == 0 {
		// List available logs
		log.ListLogs(cfg)
		return
	}

	logName := args[0]

	// Handle special log names
	switch {
	case strings.Contains(logName, "/"):
		// Package log (category/portname)
		log.ViewPackageLog(cfg, logName)
	case logName == "results" || logName == "00":
		log.ViewLog(cfg, "00_last_results.log")
	case logName == "success" || logName == "01":
		log.ViewLog(cfg, "01_success_list.log")
	case logName == "failure" || logName == "02":
		log.ViewLog(cfg, "02_failure_list.log")
	case logName == "ignored" || logName == "03":
		log.ViewLog(cfg, "03_ignored_list.log")
	case logName == "skipped" || logName == "04":
		log.ViewLog(cfg, "04_skipped_list.log")
	case logName == "abnormal" || logName == "05":
		log.ViewLog(cfg, "05_abnormal_command_output.log")
	case logName == "obsolete" || logName == "06":
		log.ViewLog(cfg, "06_obsolete_packages.log")
	case logName == "debug" || logName == "07":
		log.ViewLog(cfg, "07_debug.log")
	default:
		log.ViewLog(cfg, logName)
	}
}

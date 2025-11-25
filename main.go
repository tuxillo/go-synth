package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"dsynth/build"
	"dsynth/config"
	"dsynth/log"
	"dsynth/pkg"
	"dsynth/util"
)

var Version = "dev"

func main() {
	// Command-line flags
	debug := flag.Bool("d", false, "Debug verbosity")
	force := flag.Bool("f", false, "Force operations")
	yesAll := flag.Bool("y", false, "Answer yes to all prompts")
	memTarget := flag.Int("m", 0, "Package dependency memory target in GB")
	profile := flag.String("p", "default", "Profile to use")
	slowStart := flag.Int("s", 0, "Initial worker count (slow start)")
	configDir := flag.String("C", "", "Config base directory")
	devMode := flag.Bool("D", false, "Developer mode")
	checkPlist := flag.Bool("P", false, "Check plist")
	disableUI := flag.Bool("S", false, "Disable ncurses UI")
	niceVal := flag.Int("N", 10, "Nice value for builds")

	flag.Usage = usage
	flag.Parse()

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
	// Skip unsupported config options for now
	_ = memTarget
	_ = niceVal

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
	fmt.Println("  -m GB         Package dependency memory target (reserved)")
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
	fmt.Println("Maintenance Commands:")
	fmt.Println("  status [ports...]        Show port build status")
	fmt.Println("  status-everything        Status of entire ports tree")
	fmt.Println("  cleanup                  Clean up stale mounts and logs")
	fmt.Println("  configure                Configure dsynth")
	fmt.Println("  rebuild-repository       Rebuild package repository")
	fmt.Println("  purge-distfiles          Remove obsolete distfiles")
	fmt.Println("  reset-db                 Reset CRC database")
	fmt.Println("  verify                   Verify package integrity")
	fmt.Println("  logs [port]              View build logs")
	fmt.Println()
}

func doInit(cfg *config.Config) {
	fmt.Println("Initializing dsynth configuration...")

	// Create required directories
	dirs := []string{
		cfg.BuildBase,
		cfg.LogsPath,
		cfg.DPortsPath,
		cfg.RepositoryPath,
		cfg.PackagesPath,
		cfg.DistFilesPath,
		cfg.OptionsPath,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	// Create template directory
	templateDir := filepath.Join(cfg.BuildBase, "Template")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating template directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Configuration initialized successfully")
}

func doStatus(cfg *config.Config, portList []string) {
	if len(portList) == 0 {
		fmt.Println("No ports specified")
		return
	}

	fmt.Println("Status not yet implemented")
	// TODO: Implement status checking
}

func doCleanup(cfg *config.Config) {
	fmt.Println("Cleaning up...")

	// TODO: Clean up any stale mounts
	// TODO: Clean up old logs

	fmt.Println("Cleanup complete")
}

func doConfigure(cfg *config.Config) {
	fmt.Println("Configure not yet implemented")
	// TODO: Implement interactive configuration
}

func doUpgradeSystem(cfg *config.Config) {
	fmt.Println("Upgrading system packages...")

	// Get list of installed packages
	installed, err := pkg.GetInstalledPackages(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting installed packages: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d installed packages\n", len(installed))

	// Build the installed packages
	doBuild(cfg, installed, false, false)
}

func doPrepareSystem(cfg *config.Config) {
	fmt.Println("Preparing system...")
	doUpgradeSystem(cfg)
}

func doRebuildRepo(cfg *config.Config) {
	fmt.Println("Rebuilding repository metadata...")

	// TODO: Implement pkg repo rebuild
	fmt.Println("Repository rebuild not yet implemented")
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
	// TODO: Implement package verification
	fmt.Println("Package verification not yet implemented")
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

	// DEBUG: Print config values
	fmt.Printf("\nDEBUG: Config loaded:\n")
	fmt.Printf("  Profile: %s\n", cfg.Profile)
	fmt.Printf("  BuildBase: %s\n", cfg.BuildBase)
	fmt.Printf("  OptionsPath: %s\n", cfg.OptionsPath)
	fmt.Printf("  PackagesPath: %s\n", cfg.PackagesPath)
	fmt.Printf("  RepositoryPath: %s\n", cfg.RepositoryPath)
	fmt.Printf("  DistFilesPath: %s\n", cfg.DistFilesPath)
	fmt.Printf("  DPortsPath: %s\n", cfg.DPortsPath)
	fmt.Println()

	// Setup signal handler for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	var buildCleanup func()

	// Goroutine to handle signals
	go func() {
		sig := <-sigChan
		fmt.Fprintf(os.Stderr, "\nReceived signal %v, cleaning up...\n", sig)

		if buildCleanup != nil {
			buildCleanup()
		}

		os.Exit(1)
	}()

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

	// Create build state registry and populate from Package.Flags
	// (transitional - eventually MarkPackagesNeedingBuild will use registry)
	registry := pkg.NewBuildStateRegistry()
	for p := head; p != nil; p = p.Next {
		if p.Flags != 0 {
			registry.SetFlags(p, p.Flags)
		}
		if p.IgnoreReason != "" {
			registry.SetIgnoreReason(p, p.IgnoreReason)
		}
	}

	// DEBUG: Print what packages are marked for build vs skipped
	fmt.Println("\nDEBUG: Package status (first 10):")
	count := 0
	for p := head; p != nil && count < 10; p = p.Next {
		fmt.Printf("  %s: Flags=%08x PkgFile=%s\n", p.PortDir, registry.GetFlags(p), p.PkgFile)

		// Check if package file actually exists
		pkgPath := filepath.Join(cfg.PackagesPath, "All", p.PkgFile)
		if _, err := os.Stat(pkgPath); err == nil {
			fmt.Printf("    -> Package EXISTS at %s\n", pkgPath)
		} else {
			fmt.Printf("    -> Package NOT FOUND at %s (err: %v)\n", pkgPath, err)
		}
		count++
	}
	fmt.Println()

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

	// Execute build - NOW WITH 3 RETURN VALUES
	stats, cleanup, err := build.DoBuild(head, cfg, logger)
	buildCleanup = cleanup // Store cleanup function for signal handler

	if err != nil {
		fmt.Fprintf(os.Stderr, "Build error: %v\n", err)
		if cleanup != nil {
			cleanup()
		}
		os.Exit(1)
	}

	// Cleanup workers after successful build
	if cleanup != nil {
		cleanup()
	}

	// Save CRC database
	if err := pkg.SaveCRCDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save CRC database: %v\n", err)
	}

	// Print statistics
	fmt.Println()
	fmt.Println("Build Statistics:")
	fmt.Printf("  Total packages: %d\n", stats.Total)
	fmt.Printf("  Success: %d\n", stats.Success)
	fmt.Printf("  Failed: %d\n", stats.Failed)
	fmt.Printf("  Skipped: %d\n", stats.Skipped)
	fmt.Printf("  Ignored: %d\n", stats.Ignored)
	fmt.Printf("  Duration: %s\n\n", stats.Duration)

	// Also update repo if not just-build mode
	if !justBuild {
		doRebuildRepo(cfg)
	}

	if stats.Failed > 0 {
		os.Exit(1)
	}
}

func doFetchOnly(cfg *config.Config, portList []string) {
	fmt.Println("Fetch-only not yet implemented")
	// TODO: Implement fetch-only
}

func doLogs(cfg *config.Config, portList []string) {
	if len(portList) == 0 {
		fmt.Println("No port specified")
		return
	}

	port := portList[0]
	logFile := filepath.Join(cfg.LogsPath, strings.ReplaceAll(port, "/", "___")+".log")

	if !util.FileExists(logFile) {
		fmt.Printf("Log file not found: %s\n", logFile)
		return
	}

	// Display the log file
	content, err := os.ReadFile(logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading log: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(string(content))
}

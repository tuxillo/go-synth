package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go-synth/config"
	_ "go-synth/environment/bsd" // Register BSD backend
	"go-synth/pkg"
	"go-synth/service"
	"go-synth/util"
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
		fmt.Printf("go-synth version %s\n", Version)
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

// askYN prompts the user for yes/no confirmation.
// This is a CLI-specific function and should not be moved to a library package.
func askYN(prompt string, defaultYes bool) bool {
	if defaultYes {
		fmt.Printf("%s [Y/n]: ", prompt)
	} else {
		fmt.Printf("%s [y/N]: ", prompt)
	}

	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	if response == "" {
		return defaultYes
	}

	return response == "y" || response == "yes"
}

func usage() {
	fmt.Printf("go-synth version %s - DragonFly BSD ports build system\n\n", Version)
	fmt.Println("Usage: go-synth [options] command [args]")
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
	fmt.Println("  configure                Configure go-synth")
	fmt.Println("  rebuild-repository       Rebuild package repository")
	fmt.Println("  purge-distfiles          Remove obsolete distfiles")
	fmt.Println("  reset-db                 Reset CRC database")
	fmt.Println("  verify                   Verify package integrity")
	fmt.Println("  logs [port]              View build logs")
	fmt.Println()
}

func doInit(cfg *config.Config) {
	fmt.Println("Initializing go-synth environment...")
	fmt.Println()

	// Create service
	svc, err := service.NewService(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Failed to initialize service: %v\n", err)
		os.Exit(1)
	}
	defer svc.Close()

	// Check for migration and prompt user if needed
	autoMigrate := cfg.Migration.AutoMigrate
	if svc.NeedsMigration() && !cfg.YesAll && !autoMigrate {
		legacyFile, _ := svc.GetLegacyCRCFile()
		fmt.Println("⚠️  Legacy CRC data detected!")
		fmt.Printf("Found: %s\n", legacyFile)
		fmt.Print("Migrate legacy data now? [Y/n]: ")
		var response string
		fmt.Scanln(&response)
		if response == "" || strings.EqualFold(response, "y") || strings.EqualFold(response, "yes") {
			autoMigrate = true
		}
	}

	// Initialize environment using service layer
	result, err := svc.Initialize(service.InitOptions{
		AutoMigrate: autoMigrate || cfg.YesAll,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Initialization failed: %v\n", err)
		os.Exit(1)
	}

	// Display results to user
	fmt.Println("Setting up directories:")
	for _, dir := range result.DirsCreated {
		fmt.Printf("  ✓ %s\n", dir)
	}

	if result.TemplateCreated {
		templateDir := filepath.Join(cfg.BuildBase, "Template")
		fmt.Printf("  ✓ Template: %s (with /etc files)\n", templateDir)
	}

	if result.DatabaseInitalized {
		fmt.Println("\nInitializing build database:")
		fmt.Printf("  ✓ Database: %s\n", cfg.Database.Path)
	}

	if result.MigrationNeeded {
		fmt.Println()
		if result.MigrationPerformed {
			fmt.Println("  ✓ Legacy data migrated successfully")
		} else {
			fmt.Println("  - Migration skipped (can migrate later with first build)")
		}
	}

	// Display warnings if any
	if len(result.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, warning := range result.Warnings {
			fmt.Printf("  ⚠  %s\n", warning)
		}
	}

	// Verify ports directory
	fmt.Println("\nVerifying environment:")
	if result.PortsFound == 0 {
		fmt.Printf("  ⚠  Ports directory is empty: %s\n", cfg.DPortsPath)
		fmt.Println("     You'll need to populate it before building")
	} else {
		fmt.Printf("  ✓ Ports directory: %s (%d entries)\n", cfg.DPortsPath, result.PortsFound)
	}

	configPath := cfg.ConfigPath
	if configPath == "" {
		configPath = "/etc/dsynth/dsynth.ini"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("\nWriting configuration file:")
		if err := config.SaveConfig(configPath, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠  Failed to write %s: %v\n", configPath, err)
			fmt.Println("     Please run go-synth as root or create the config manually.")
		} else {
			fmt.Printf("  ✓ Config file created: %s\n", configPath)
		}
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "\n⚠  Unable to check config file: %v\n", err)
	} else {
		fmt.Printf("\nConfiguration file already exists: %s (not modified)\n", configPath)
	}

	// Success summary
	fmt.Println("\n✓ Initialization complete!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Verify configuration file (if needed)")
	fmt.Println("  2. Ensure ports tree is populated")
	fmt.Println("  3. Run: go-synth build <package>")
	fmt.Println()
}

func doStatus(cfg *config.Config, portList []string) {
	// Create service
	svc, err := service.NewService(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize service: %v\n", err)
		fmt.Println("No build history available. Run a build first.")
		return
	}
	defer svc.Close()

	// Get status from service
	result, err := svc.GetStatus(service.StatusOptions{
		PortList: portList,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get status: %v\n", err)
		os.Exit(1)
	}

	if len(portList) == 0 {
		// Show overall database statistics
		fmt.Println("=== Build Database Status ===")
		fmt.Printf("Database:      %s\n", result.Stats.DatabasePath)
		fmt.Printf("Size:          %s\n", formatBytes(result.Stats.DatabaseSize))
		fmt.Printf("Total builds:  %d\n", result.Stats.TotalBuilds)
		fmt.Printf("Unique ports:  %d\n", result.Stats.TotalPorts)
		fmt.Printf("CRC entries:   %d\n", result.Stats.TotalCRCs)
		return
	}

	// Show status for specific ports
	fmt.Println("=== Port Build Status ===")
	for _, portStatus := range result.Ports {
		if portStatus.LastBuild == nil {
			fmt.Printf("\n%s: never built\n", portStatus.PortDir)
			continue
		}

		rec := portStatus.LastBuild
		fmt.Printf("\n%s:\n", portStatus.PortDir)
		fmt.Printf("  Status:      %s\n", rec.Status)
		fmt.Printf("  UUID:        %s\n", rec.UUID[:8]) // Short UUID
		if rec.Version != "" {
			fmt.Printf("  Version:     %s\n", rec.Version)
		}
		fmt.Printf("  Started:     %s\n", rec.StartTime.Format("2006-01-02 15:04:05"))
		if !rec.EndTime.IsZero() {
			fmt.Printf("  Ended:       %s\n", rec.EndTime.Format("2006-01-02 15:04:05"))
			duration := rec.EndTime.Sub(rec.StartTime)
			fmt.Printf("  Duration:    %s\n", duration.Round(time.Second))
		}

		// Show CRC if available
		if portStatus.CRC != 0 {
			fmt.Printf("  CRC:         %08x\n", portStatus.CRC)
		}
	}
}

// formatBytes formats bytes as human-readable string (e.g., "1.5 MiB")
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func doCleanup(cfg *config.Config) {
	fmt.Println("Cleaning up stale worker environments...")

	// Create service
	svc, err := service.NewService(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize service: %v\n", err)
		os.Exit(1)
	}
	defer svc.Close()

	// Cleanup stale workers using service layer
	// This handles orphaned worker directories from crashed builds
	result, err := svc.CleanupStaleWorkers(service.CleanupOptions{
		Force: cfg.Force,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cleanup failed: %v\n", err)
		os.Exit(1)
	}

	// Display results
	if result.WorkersCleaned == 0 {
		fmt.Println("No stale worker directories found")
	} else {
		// Display any errors that occurred
		for _, cleanupErr := range result.Errors {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", cleanupErr)
		}

		fmt.Printf("\nCleaned up %d stale worker directories\n", result.WorkersCleaned)
	}

	// Clear stale build locks from crashed/interrupted builds
	fmt.Println("\nClearing stale build locks...")
	cleared, err := svc.Database().ClearActiveLocks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to clear locks: %v\n", err)
	} else if cleared > 0 {
		fmt.Printf("Cleared %d stale build lock(s)\n", cleared)
	} else {
		fmt.Println("No stale build locks found")
	}

	// Also cleanup old logs (optional)
	fmt.Println("\nCleaning up old logs...")
	logsPath := cfg.LogsPath
	if logsPath != "" && util.FileExists(logsPath) {
		// Remove logs older than 7 days (optional, could be configurable)
		fmt.Println("  (Log cleanup not yet implemented)")
	}

	fmt.Println("\n✓ Cleanup complete")
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
	// Create service
	svc, err := service.NewService(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize service: %v\n", err)
		os.Exit(1)
	}
	defer svc.Close()

	// Check if database exists
	if !svc.DatabaseExists() {
		fmt.Println("No database found")
		return
	}

	// Confirm destructive operation (unless -y flag)
	if !cfg.YesAll {
		fmt.Printf("⚠️  WARNING: This will delete the build database\n")
		fmt.Printf("Database: %s\n", svc.GetDatabasePath())
		fmt.Print("\nAre you sure? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
			fmt.Println("Cancelled")
			return
		}
	}

	// Reset database using service layer
	result, err := svc.ResetDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to reset database: %v\n", err)
		os.Exit(1)
	}

	// Display results
	if result.DatabaseRemoved {
		fmt.Println("✓ Build database reset successfully")
	}

	// Show all files that were removed
	for _, file := range result.FilesRemoved {
		if strings.Contains(file, "crc_index.bak") {
			fmt.Println("✓ Legacy CRC backup also removed")
		} else if strings.Contains(file, "crc_index") {
			fmt.Println("✓ Legacy CRC file also removed")
		}
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

	// Create service
	svc, err := service.NewService(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing service: %v\n", err)
		os.Exit(1)
	}
	defer svc.Close()

	// Check for migration and prompt user if needed (unless auto-migrate is on)
	if !cfg.Migration.AutoMigrate {
		migStatus, err := svc.CheckMigrationStatus()
		if err == nil && migStatus.Needed && !cfg.YesAll {
			fmt.Println("\n⚠️  Legacy CRC data detected!")
			fmt.Printf("Found legacy CRC file: %s\n", migStatus.LegacyFile)
			fmt.Println("This data will be imported into the new BuildDB.")
			fmt.Print("Migrate legacy data now? [Y/n]: ")
			var response string
			fmt.Scanln(&response)
			if !strings.EqualFold(response, "n") && !strings.EqualFold(response, "no") {
				if err := svc.PerformMigration(); err != nil {
					fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
					os.Exit(1)
				}
			} else {
				fmt.Println("Skipping migration. Note: CRC skip functionality requires migration.")
			}
		}
	}

	// Get build plan to show user what will be built
	fmt.Printf("Analyzing %d port(s)...\n", len(portList))
	plan, err := svc.GetBuildPlan(portList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error analyzing build: %v\n", err)
		os.Exit(1)
	}

	// Display build plan summary
	fmt.Println("\nBuild Plan:")
	fmt.Printf("  Total packages: %d\n", plan.TotalPackages)
	fmt.Printf("  To build: %d\n", plan.NeedBuild)
	fmt.Printf("  To skip: %d\n", len(plan.ToSkip))

	if plan.NeedBuild > 0 {
		fmt.Println("\nPackages to build:")
		for i, portDir := range plan.ToBuild {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(plan.ToBuild)-10)
				break
			}
			fmt.Printf("  - %s\n", portDir)
		}
	}
	fmt.Println()

	if plan.NeedBuild == 0 {
		fmt.Println("All packages are up-to-date!")
		return
	}

	// Confirm build
	if !cfg.YesAll {
		if !askYN(fmt.Sprintf("Build %d packages?", plan.NeedBuild), true) {
			fmt.Println("Build cancelled")
			return
		}
	}

	// Setup signal handler for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Goroutine to handle signals
	go func() {
		sig := <-sigChan
		fmt.Fprintf(os.Stderr, "\nReceived signal %v, cleaning up...\n", sig)

		// Get the active build's cleanup function from service
		// This is set immediately when workers are created
		cleanup := svc.GetActiveCleanup()
		if cleanup != nil {
			fmt.Fprintf(os.Stderr, "Cleaning up active build workers...\n")
			cleanup()
			fmt.Fprintf(os.Stderr, "Worker cleanup complete\n")
		} else {
			fmt.Fprintf(os.Stderr, "No active cleanup function found (build may not have started workers yet)\n")
		}

		// Close service (DB, logger, etc.)
		_ = svc.Close()

		os.Exit(1)
	}()

	// Execute build using service layer
	result, err := svc.Build(service.BuildOptions{
		PortList:  portList,
		Force:     cfg.Force,
		JustBuild: justBuild,
		TestMode:  testMode,
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Build error: %v\n", err)
		os.Exit(1)
	}

	// Print statistics
	fmt.Println()
	fmt.Println("Build Complete!")
	fmt.Println("================")
	fmt.Printf("  Total packages:  %d\n", result.Stats.Total)
	fmt.Printf("  ✓ Success:       %d\n", result.Stats.Success)
	fmt.Printf("  ✗ Failed:        %d\n", result.Stats.Failed)
	fmt.Printf("  - Skipped:       %d\n", result.Stats.Skipped)
	fmt.Printf("  - Ignored:       %d\n", result.Stats.Ignored)
	fmt.Printf("  Duration:        %s\n\n", result.Stats.Duration)

	// Also update repo if not just-build mode
	if !justBuild {
		doRebuildRepo(cfg)
	}

	if result.Stats.Failed > 0 {
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

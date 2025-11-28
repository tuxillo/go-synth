package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"dsynth/build"
	"dsynth/builddb"
	"dsynth/config"
	"dsynth/log"
	"dsynth/migration"
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
	fmt.Println("Initializing dsynth environment...")
	fmt.Println()

	// 1. Create required directories
	fmt.Println("Setting up directories:")
	dirs := map[string]string{
		"Build base":   cfg.BuildBase,
		"Logs":         cfg.LogsPath,
		"Ports":        cfg.DPortsPath,
		"Repository":   cfg.RepositoryPath,
		"Packages":     cfg.PackagesPath,
		"Distribution": cfg.DistFilesPath,
		"Options":      cfg.OptionsPath,
	}

	for label, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Failed to create %s directory: %v\n", label, err)
			os.Exit(1)
		}
		fmt.Printf("  ✓ %s: %s\n", label, dir)
	}

	// Create template directory
	templateDir := filepath.Join(cfg.BuildBase, "Template")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Failed to create template directory: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  ✓ Template: %s\n", templateDir)

	// 2. Initialize BuildDB
	fmt.Println("\nInitializing build database:")
	dbPath := cfg.Database.Path
	db, err := builddb.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Printf("  ✓ Database: %s\n", dbPath)

	// 3. Check for legacy CRC migration
	if cfg.Migration.AutoMigrate && migration.DetectMigrationNeeded(cfg) {
		fmt.Println("\n⚠️  Legacy CRC data detected!")
		legacyFile := filepath.Join(cfg.BuildBase, "crc_index")
		fmt.Printf("Found: %s\n", legacyFile)

		if cfg.YesAll {
			fmt.Println("Migrating automatically (-y flag)...")
			if err := migration.MigrateLegacyCRC(cfg, db); err != nil {
				fmt.Fprintf(os.Stderr, "✗ Migration failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("  ✓ Legacy data migrated successfully")
		} else {
			fmt.Print("Migrate legacy data now? [Y/n]: ")
			var response string
			fmt.Scanln(&response)
			if response == "" || strings.EqualFold(response, "y") || strings.EqualFold(response, "yes") {
				if err := migration.MigrateLegacyCRC(cfg, db); err != nil {
					fmt.Fprintf(os.Stderr, "✗ Migration failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("  ✓ Legacy data migrated successfully")
			} else {
				fmt.Println("  - Migration skipped (can migrate later with first build)")
			}
		}
	}

	// 4. Verify ports directory exists and has content
	fmt.Println("\nVerifying environment:")
	if _, err := os.Stat(cfg.DPortsPath); os.IsNotExist(err) {
		fmt.Printf("  ⚠  Ports directory is empty: %s\n", cfg.DPortsPath)
		fmt.Println("     You'll need to populate it before building")
	} else {
		// Quick check if it has any content
		entries, _ := os.ReadDir(cfg.DPortsPath)
		if len(entries) == 0 {
			fmt.Printf("  ⚠  Ports directory is empty: %s\n", cfg.DPortsPath)
		} else {
			fmt.Printf("  ✓ Ports directory: %s (%d entries)\n", cfg.DPortsPath, len(entries))
		}
	}

	// 5. Success summary
	fmt.Println("\n✓ Initialization complete!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Verify configuration file (if needed)")
	fmt.Println("  2. Ensure ports tree is populated")
	fmt.Println("  3. Run: dsynth build <package>")
	fmt.Println()
}

func doStatus(cfg *config.Config, portList []string) {
	// Open database
	dbPath := cfg.Database.Path
	db, err := builddb.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		fmt.Println("No build history available. Run a build first.")
		return
	}
	defer db.Close()

	if len(portList) == 0 {
		// Show overall database statistics
		stats, err := db.Stats()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get database stats: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("=== Build Database Status ===")
		fmt.Printf("Database:      %s\n", stats.DatabasePath)
		fmt.Printf("Size:          %s\n", formatBytes(stats.DatabaseSize))
		fmt.Printf("Total builds:  %d\n", stats.TotalBuilds)
		fmt.Printf("Unique ports:  %d\n", stats.TotalPorts)
		fmt.Printf("CRC entries:   %d\n", stats.TotalCRCs)
		return
	}

	// Show status for specific ports
	fmt.Println("=== Port Build Status ===")
	for _, portDir := range portList {
		rec, err := db.LatestFor(portDir, "")
		if err != nil {
			fmt.Printf("\n%s: never built\n", portDir)
			continue
		}

		fmt.Printf("\n%s:\n", portDir)
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
		if crc, exists, err := db.GetCRC(portDir); err == nil && exists {
			fmt.Printf("  CRC:         %08x\n", crc)
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

	// Look for worker directories in BuildBase
	baseDir := cfg.BuildBase
	workersFound := 0
	workersCleanedUp := 0

	// Scan for SL.* directories (worker directories)
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read build directory: %v\n", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Worker directories match pattern "SL.*"
		if strings.HasPrefix(entry.Name(), "SL.") {
			workersFound++
			workerPath := filepath.Join(baseDir, entry.Name())

			// Try to cleanup mounts for this worker
			if err := cleanupWorkerMounts(workerPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to cleanup %s: %v\n", entry.Name(), err)
				continue
			}

			workersCleanedUp++
			fmt.Printf("  ✓ Cleaned up %s\n", entry.Name())
		}
	}

	if workersFound == 0 {
		fmt.Println("No worker directories found")
	} else {
		fmt.Printf("\nCleaned up %d/%d worker directories\n", workersCleanedUp, workersFound)
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

// cleanupWorkerMounts attempts to unmount and remove a worker directory
func cleanupWorkerMounts(workerPath string) error {
	// This is a simplified version that just tries to unmount common mount points
	// In a full implementation, we'd scan /proc/mounts or use mount(8) to find all mounts

	commonMounts := []string{
		"dev",
		"proc",
		"distfiles",
		"packages",
		"ccache",
		"logs",
		"options",
		"construction",
	}

	// Try to unmount in reverse order
	for i := len(commonMounts) - 1; i >= 0; i-- {
		mountPoint := filepath.Join(workerPath, commonMounts[i])
		// Ignore errors - mount might not exist
		exec.Command("umount", "-f", mountPoint).Run()
	}

	// Try to remove the directory
	return os.RemoveAll(workerPath)
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
	dbPath := cfg.Database.Path

	// Check if database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Println("No database found")
		return
	}

	// Confirm destructive operation (unless -y flag)
	if !cfg.YesAll {
		fmt.Printf("⚠️  WARNING: This will delete the build database\n")
		fmt.Printf("Database: %s\n", dbPath)
		fmt.Print("\nAre you sure? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
			fmt.Println("Cancelled")
			return
		}
	}

	// Remove database file
	if err := os.Remove(dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to remove database: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Build database reset successfully")

	// Also remove legacy CRC file if present (optional cleanup)
	legacyFile := filepath.Join(cfg.BuildBase, "crc_index")
	if _, err := os.Stat(legacyFile); err == nil {
		os.Remove(legacyFile)
		fmt.Println("✓ Legacy CRC file also removed")
	}

	// Also remove backup if present
	backupFile := legacyFile + ".bak"
	if _, err := os.Stat(backupFile); err == nil {
		os.Remove(backupFile)
		fmt.Println("✓ Legacy CRC backup also removed")
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

	// Open BuildDB once for the entire workflow
	dbPath := cfg.Database.Path
	buildDB, err := builddb.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening build database: %v\n", err)
		os.Exit(1)
	}
	defer buildDB.Close()

	// Check for legacy CRC migration (if enabled in config)
	if cfg.Migration.AutoMigrate && migration.DetectMigrationNeeded(cfg) {
		fmt.Println("\n⚠️  Legacy CRC data detected!")
		fmt.Printf("Found legacy CRC file: %s/crc_index\n", cfg.BuildBase)
		fmt.Println("This data will be imported into the new BuildDB.")

		if !cfg.YesAll {
			fmt.Print("Migrate legacy data now? [Y/n]: ")
			var response string
			fmt.Scanln(&response)
			if strings.EqualFold(response, "n") || strings.EqualFold(response, "no") {
				fmt.Println("Skipping migration. Note: CRC skip functionality requires migration.")
			} else {
				if err := migration.MigrateLegacyCRC(cfg, buildDB); err != nil {
					fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
					os.Exit(1)
				}
			}
		} else {
			// Auto-migrate with -y flag
			fmt.Println("Auto-migrating legacy CRC data (-y flag)...")
			if err := migration.MigrateLegacyCRC(cfg, buildDB); err != nil {
				fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
				os.Exit(1)
			}
		}
	}

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

		// Close buildDB on signal
		if buildDB != nil {
			buildDB.Close()
		}

		os.Exit(1)
	}()

	fmt.Printf("Building %d port(s)...\n", len(portList))

	// Create build state registry
	registry := pkg.NewBuildStateRegistry()

	// Create package registry
	pkgRegistry := pkg.NewPackageRegistry()

	// Parse port specifications into package list
	packages, err := pkg.ParsePortList(portList, cfg, registry, pkgRegistry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing port list: %v\n", err)
		os.Exit(1)
	}

	// Resolve all dependencies
	if err := pkg.ResolveDependencies(packages, cfg, registry, pkgRegistry); err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving dependencies: %v\n", err)
		os.Exit(1)
	}

	// Check which packages need building (CRC-based)
	needBuild, err := pkg.MarkPackagesNeedingBuild(packages, cfg, registry, buildDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking build status: %v\n", err)
		os.Exit(1)
	}

	// Display build plan summary
	totalPkgs := len(packages)
	skipCount := totalPkgs - needBuild

	fmt.Println("\nBuild Plan:")
	fmt.Printf("  Total packages: %d\n", totalPkgs)
	fmt.Printf("  To build: %d\n", needBuild)
	fmt.Printf("  To skip: %d\n", skipCount)

	if needBuild > 0 {
		fmt.Println("\nPackages to build:")
		count := 0
		for _, p := range packages {
			flags := registry.GetFlags(p)
			// Show packages that are NOT already packaged
			if !flags.Has(pkg.PkgFPackaged) {
				fmt.Printf("  - %s\n", p.PortDir)
				count++
				if count >= 10 {
					if needBuild > 10 {
						fmt.Printf("  ... and %d more\n", needBuild-10)
					}
					break
				}
			}
		}
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
	stats, cleanup, err := build.DoBuild(packages, cfg, logger, buildDB)
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

	// Print statistics
	fmt.Println()
	fmt.Println("Build Complete!")
	fmt.Println("================")
	fmt.Printf("  Total packages:  %d\n", stats.Total)
	fmt.Printf("  ✓ Success:       %d\n", stats.Success)
	if stats.Failed > 0 {
		fmt.Printf("  ✗ Failed:        %d\n", stats.Failed)
	} else {
		fmt.Printf("  ✗ Failed:        %d\n", stats.Failed)
	}
	fmt.Printf("  - Skipped:       %d\n", stats.Skipped)
	fmt.Printf("  - Ignored:       %d\n", stats.Ignored)
	fmt.Printf("  Duration:        %s\n\n", stats.Duration)

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

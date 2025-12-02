package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"go-synth/build"
	"go-synth/builddb"
	"go-synth/config"
	"go-synth/log"
	"go-synth/pkg"

	"github.com/spf13/cobra"
)

// TODO: Phase 3 - This is skeleton code for future cobra CLI
// For now, main.go handles CLI directly
var buildCmd = &cobra.Command{
	Use:   "build [ports...]",
	Short: "Build specified ports",
	Long:  `Build the specified ports and their dependencies`,
	Run:   runBuild,
}

// Commented out until root.go is created in Phase 3
// func init() {
// 	rootCmd.AddCommand(buildCmd)
// }

func runBuild(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: no ports specified")
		os.Exit(1)
	}

	cfg := config.GetConfig()
	logger, _ := log.NewLogger(cfg) // TODO: handle error in Phase 3

	// Open BuildDB once for the entire workflow
	dbPath := filepath.Join(cfg.BuildBase, "builds.db")
	buildDB, err := builddb.OpenDB(dbPath)
	if err != nil {
		fmt.Printf("Error opening build database: %v\n", err)
		os.Exit(1)
	}
	defer buildDB.Close()

	// Setup signal handler for cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Track build context for cleanup
	var buildCleanup func()

	// Goroutine to handle signals
	go func() {
		sig := <-sigChan
		fmt.Fprintf(os.Stderr, "\nReceived signal %v, cleaning up...\n", sig)

		// Call cleanup if available
		if buildCleanup != nil {
			buildCleanup()
		}

		// Close buildDB on signal
		if buildDB != nil {
			buildDB.Close()
		}

		os.Exit(1)
	}()

	// Create build state registry
	registry := pkg.NewBuildStateRegistry()

	// Create package registry
	pkgRegistry := pkg.NewPackageRegistry()

	// Parse port list
	head, err := pkg.ParsePortList(args, cfg, registry, pkgRegistry, logger)
	if err != nil {
		fmt.Printf("Error parsing port list: %v\n", err)
		os.Exit(1)
	}

	// Resolve dependencies
	fmt.Println("Resolving dependencies...")
	if err := pkg.ResolveDependencies(head, cfg, registry, pkgRegistry, logger); err != nil {
		fmt.Printf("Error resolving dependencies: %v\n", err)
		os.Exit(1)
	}

	// Mark packages needing build
	needBuild, err := pkg.MarkPackagesNeedingBuild(head, cfg, registry, buildDB, logger)
	if err != nil {
		fmt.Printf("Error checking packages: %v\n", err)
		os.Exit(1)
	}

	if needBuild == 0 {
		fmt.Println("\nAll packages are up to date!")
		return
	}

	// Confirm build
	fmt.Printf("\nBuild %d packages? [Y/n]: ", needBuild)
	var response string
	fmt.Scanln(&response)
	if response != "" && response != "y" && response != "Y" {
		fmt.Println("Build cancelled")
		return
	}

	// Execute build with cleanup function
	stats, cleanupFunc, err := build.DoBuild(head, cfg, logger, buildDB, nil, "")
	buildCleanup = cleanupFunc

	if err != nil {
		fmt.Printf("Build error: %v\n", err)
		if cleanupFunc != nil {
			cleanupFunc()
		}
		os.Exit(1)
	}

	// Print statistics
	fmt.Printf("\nBuild Statistics:\n")
	fmt.Printf("  Total packages: %d\n", stats.Total)
	fmt.Printf("  Success: %d\n", stats.Success)
	fmt.Printf("  Failed: %d\n", stats.Failed)
	fmt.Printf("  Already built (skipped): %d\n", stats.SkippedPre)
	fmt.Printf("  Dependency skipped: %d\n", stats.Skipped)
	fmt.Printf("  Ignored: %d\n", stats.Ignored)
	fmt.Printf("  Duration: %s\n\n", stats.Duration)

	if stats.Failed > 0 {
		os.Exit(1)
	}
}

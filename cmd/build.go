package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"dsynth/build"
	"dsynth/config"
	"dsynth/log"
	"dsynth/pkg"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build [ports...]",
	Short: "Build specified ports",
	Long:  `Build the specified ports and their dependencies`,
	Run:   runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: no ports specified")
		os.Exit(1)
	}

	cfg := config.GetConfig()
	logger := log.NewLogger(cfg)

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
		
		os.Exit(1)
	}()

	// Parse port list
	head, err := pkg.ParsePortList(args, cfg)
	if err != nil {
		fmt.Printf("Error parsing port list: %v\n", err)
		os.Exit(1)
	}

	// Resolve dependencies
	fmt.Println("Resolving dependencies...")
	if err := pkg.ResolveDependencies(head, cfg); err != nil {
		fmt.Printf("Error resolving dependencies: %v\n", err)
		os.Exit(1)
	}

	// Mark packages needing build
	needBuild, err := pkg.MarkPackagesNeedingBuild(head, cfg)
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
	stats, cleanupFunc, err := build.DoBuild(head, cfg, logger)
	buildCleanup = cleanupFunc
	
	if err != nil {
		fmt.Printf("Build error: %v\n", err)
		if cleanupFunc != nil {
			cleanupFunc()
		}
		os.Exit(1)
	}

	// Save CRC database
	if err := pkg.SaveCRCDatabase(); err != nil {
		fmt.Printf("Warning: failed to save CRC database: %v\n", err)
	}

	// Print statistics
	fmt.Printf("\nBuild Statistics:\n")
	fmt.Printf("  Total packages: %d\n", stats.Total)
	fmt.Printf("  Success: %d\n", stats.Success)
	fmt.Printf("  Failed: %d\n", stats.Failed)
	fmt.Printf("  Skipped: %d\n", stats.Skipped)
	fmt.Printf("  Ignored: %d\n", stats.Ignored)
	fmt.Printf("  Duration: %s\n\n", stats.Duration)

	if stats.Failed > 0 {
		os.Exit(1)
	}
}
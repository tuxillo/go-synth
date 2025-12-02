package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go-synth/builddb"
	"go-synth/config"
	"go-synth/stats"
)

// DoMonitor implements the `go-synth monitor` command for real-time build monitoring.
// It polls the BuildDB for active build stats and displays them in real-time.
//
// Usage:
//
//	go-synth monitor              # Watch active build from BuildDB (default)
//	go-synth monitor --file PATH  # Watch legacy monitor.dat file
//	go-synth monitor export PATH  # Export current snapshot to file
func DoMonitor(cfg *config.Config, args []string) error {
	// Parse subcommand
	if len(args) > 0 && args[0] == "export" {
		if len(args) < 2 {
			return fmt.Errorf("export requires a file path argument")
		}
		return doMonitorExport(cfg, args[1])
	}

	// Check for --file flag
	if len(args) > 0 && (args[0] == "--file" || args[0] == "-f") {
		if len(args) < 2 {
			return fmt.Errorf("--file requires a path argument")
		}
		return doMonitorFile(args[1])
	}

	// Default: watch BuildDB
	return doMonitorBuildDB(cfg)
}

// doMonitorBuildDB polls BuildDB's ActiveRunSnapshot() every second and displays stats
func doMonitorBuildDB(cfg *config.Config) error {
	// Open BuildDB
	dbPath := cfg.Database.Path
	if dbPath == "" {
		// Default path if not configured
		dbPath = filepath.Join(cfg.BuildBase, "builds.db")
	}

	db, err := builddb.OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open builddb: %w", err)
	}
	defer db.Close()

	fmt.Println("Monitoring active build (press Ctrl+C to exit)...")
	fmt.Println()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Track if we've seen an active build
	lastRunID := ""
	noActiveBuildCount := 0

	for {
		// TODO: Replace with db.ActiveRunSnapshot() once Phase 3 BuildDB implementation is complete
		// For now, use ActiveRun() and return placeholder data
		runID, rec, err := db.ActiveRun()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading active run: %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}

		var snapshot *stats.TopInfo
		if rec != nil {
			// Create placeholder TopInfo from RunRecord
			// In Phase 3, this will be read from rec.LiveSnapshot JSON field
			snapshot = &stats.TopInfo{
				ActiveWorkers: 0, // Unknown without stats collector
				MaxWorkers:    8, // Placeholder
				DynMaxWorkers: 8,
				Elapsed:       time.Since(rec.StartTime),
				StartTime:     rec.StartTime,
				Built:         rec.Stats.Success,
				Failed:        rec.Stats.Failed,
				Ignored:       rec.Stats.Ignored,
				Skipped:       rec.Stats.Skipped,
				Queued:        rec.Stats.Total,
				Remaining:     rec.Stats.Total - (rec.Stats.Success + rec.Stats.Failed + rec.Stats.Ignored),
			}
		}

		// No active build
		if snapshot == nil {
			noActiveBuildCount++
			if noActiveBuildCount == 1 || noActiveBuildCount%5 == 0 {
				fmt.Printf("\r%-100s\r", "") // Clear line
				fmt.Printf("No active build... (checked %d times)\r", noActiveBuildCount)
			}
			lastRunID = ""
			<-ticker.C
			continue
		}

		// Active build found - reset counter
		noActiveBuildCount = 0

		// Print header if new build detected
		if runID != lastRunID {
			fmt.Printf("\n\n")
			fmt.Printf("═══════════════════════════════════════════════════════════════════════\n")
			fmt.Printf(" Build Run: %s\n", runID[:8])
			fmt.Printf("═══════════════════════════════════════════════════════════════════════\n")
			lastRunID = runID
		}

		// Clear screen for cleaner display (optional - comment out if annoying)
		// fmt.Print("\033[2J\033[H")

		// Display current stats
		displaySnapshot(*snapshot)

		<-ticker.C
	}
}

// displaySnapshot formats and prints a TopInfo snapshot to stdout
func displaySnapshot(info stats.TopInfo) {
	fmt.Printf("\r%-100s\r", "") // Clear line

	// Line 1: Workers and system metrics
	fmt.Printf("Workers: %2d/%2d", info.ActiveWorkers, info.MaxWorkers)
	if info.DynMaxWorkers < info.MaxWorkers {
		fmt.Printf("  [DynMax: %2d - THROTTLED]", info.DynMaxWorkers)
	} else {
		fmt.Printf("  [DynMax: %2d]", info.DynMaxWorkers)
	}
	fmt.Printf("  Load: %4.2f  Swap: %2d%%", info.Load, info.SwapPct)
	if info.NoSwap {
		fmt.Printf(" (no swap)")
	}
	fmt.Println()

	// Line 2: Build rate and timing
	fmt.Printf("Elapsed: %s  Rate: %s pkg/hr  Impulse: %.0f\n",
		stats.FormatDuration(info.Elapsed), stats.FormatRate(info.Rate), info.Impulse)

	// Line 3: Build totals
	fmt.Printf("Queued: %d  Built: %d  Failed: %d  Ignored: %d  Skipped: %d  Remaining: %d\n",
		info.Queued, info.Built, info.Failed, info.Ignored, info.Skipped, info.Remaining)

	// Throttle warning
	if info.DynMaxWorkers < info.MaxWorkers {
		reason := stats.ThrottleReason(info)
		fmt.Printf("\n⚠️  Workers throttled due to: %s\n", reason)
	}

	fmt.Println()
}

// doMonitorFile polls a legacy monitor.dat file and displays it
func doMonitorFile(path string) error {
	fmt.Printf("Monitoring file: %s (press Ctrl+C to exit)\n", path)
	fmt.Println()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("\rError reading file: %v%-50s\r", err, "")
			<-ticker.C
			continue
		}

		// Clear screen and display file contents
		fmt.Print("\033[2J\033[H")
		fmt.Printf("═══════════════════════════════════════════════════════════════════════\n")
		fmt.Printf(" Monitor File: %s\n", path)
		fmt.Printf("═══════════════════════════════════════════════════════════════════════\n\n")
		fmt.Print(string(data))
		fmt.Println()

		<-ticker.C
	}
}

// doMonitorExport exports the current active build snapshot to a dsynth-compatible file
func doMonitorExport(cfg *config.Config, exportPath string) error {
	// Open BuildDB
	dbPath := cfg.Database.Path
	if dbPath == "" {
		dbPath = filepath.Join(cfg.BuildBase, "builds.db")
	}

	db, err := builddb.OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open builddb: %w", err)
	}
	defer db.Close()

	// TODO: Replace with db.ActiveRunSnapshot() once Phase 3 BuildDB implementation is complete
	runID, rec, err := db.ActiveRun()
	if err != nil {
		return fmt.Errorf("failed to read active run: %w", err)
	}

	if rec == nil {
		return fmt.Errorf("no active build to export")
	}

	// Create placeholder TopInfo from RunRecord
	// In Phase 3, this will be read from rec.LiveSnapshot JSON field
	elapsed := time.Since(rec.StartTime)

	// Format as dsynth-compatible monitor.dat
	content := fmt.Sprintf(`Load=0.00
Swap=0
Workers=0/8
DynMax=8
Rate=0.0
Impulse=0
Elapsed=%d
Queued=%d
Built=%d
Failed=%d
Ignored=%d
Skipped=%d
`,
		int(elapsed.Seconds()),
		rec.Stats.Total,
		rec.Stats.Success,
		rec.Stats.Failed,
		rec.Stats.Ignored,
		rec.Stats.Skipped,
	)

	// Write to file
	if err := os.WriteFile(exportPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Exported snapshot from build %s to %s\n", runID[:8], exportPath)
	return nil
}

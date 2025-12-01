package log

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go-synth/config"
)

// ListLogs lists all available log files
func ListLogs(cfg *config.Config) {
	fmt.Println("Available log files:")
	fmt.Println()
	fmt.Println("Summary logs:")
	fmt.Println("  00 or results  - 00_last_results.log")
	fmt.Println("  01 or success  - 01_success_list.log")
	fmt.Println("  02 or failure  - 02_failure_list.log")
	fmt.Println("  03 or ignored  - 03_ignored_list.log")
	fmt.Println("  04 or skipped  - 04_skipped_list.log")
	fmt.Println("  05 or abnormal - 05_abnormal_command_output.log")
	fmt.Println("  06 or obsolete - 06_obsolete_packages.log")
	fmt.Println("  07 or debug    - 07_debug.log")
	fmt.Println()
	fmt.Println("Package logs:")
	fmt.Println("  Use category/portname to view package-specific log")
	fmt.Println()

	// List some recent package logs
	logsDir := filepath.Join(cfg.LogsPath, "logs")
	if _, err := os.Stat(logsDir); err == nil {
		fmt.Println("Recent package logs:")
		filepath.Walk(logsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.HasSuffix(path, ".log") {
				relPath, _ := filepath.Rel(logsDir, path)
				relPath = strings.TrimSuffix(relPath, ".log")
				fmt.Printf("  %s\n", relPath)
			}
			return nil
		})
	}
}

// ViewLog views a specific log file
func ViewLog(cfg *config.Config, logName string) {
	logPath := filepath.Join(cfg.LogsPath, logName)

	file, err := os.Open(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		return
	}
	defer file.Close()

	// Use a pager if available, otherwise print directly
	if usePager() {
		viewWithPager(logPath)
	} else {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}
}

// ViewPackageLog views a package-specific log
func ViewPackageLog(cfg *config.Config, portDir string) {
	logPath := filepath.Join(cfg.LogsPath, "logs", portDir+".log")

	file, err := os.Open(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening package log: %v\n", err)
		fmt.Fprintf(os.Stderr, "Log file: %s\n", logPath)
		return
	}
	defer file.Close()

	if usePager() {
		viewWithPager(logPath)
	} else {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}
}

// usePager checks if a pager is available
func usePager() bool {
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}

	// Check if pager exists
	_, err := os.Stat("/usr/bin/" + pager)
	return err == nil
}

// viewWithPager views a file using a pager
func viewWithPager(filepath string) {
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}

	// Use exec.Command instead of os.System
	cmd := exec.Command(pager, filepath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// TailLog shows the last N lines of a log file
func TailLog(cfg *config.Config, logName string, lines int) {
	logPath := filepath.Join(cfg.LogsPath, logName)

	file, err := os.Open(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		return
	}
	defer file.Close()

	// Read all lines
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	// Print last N lines
	start := len(allLines) - lines
	if start < 0 {
		start = 0
	}

	for i := start; i < len(allLines); i++ {
		fmt.Println(allLines[i])
	}
}

// GrepLog searches for a pattern in a log file
func GrepLog(cfg *config.Config, logName, pattern string) {
	logPath := filepath.Join(cfg.LogsPath, logName)

	file, err := os.Open(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(line, pattern) {
			fmt.Printf("%d: %s\n", lineNum, line)
		}
	}
}

// GetLogSummary returns a summary of build results from logs
func GetLogSummary(cfg *config.Config) map[string]int {
	summary := make(map[string]int)

	// Count successes
	successPath := filepath.Join(cfg.LogsPath, "01_success_list.log")
	if lines, err := countLines(successPath); err == nil {
		summary["success"] = lines
	}

	// Count failures
	failurePath := filepath.Join(cfg.LogsPath, "02_failure_list.log")
	if lines, err := countLines(failurePath); err == nil {
		summary["failed"] = lines
	}

	// Count ignored
	ignoredPath := filepath.Join(cfg.LogsPath, "03_ignored_list.log")
	if lines, err := countLines(ignoredPath); err == nil {
		summary["ignored"] = lines
	}

	// Count skipped
	skippedPath := filepath.Join(cfg.LogsPath, "04_skipped_list.log")
	if lines, err := countLines(skippedPath); err == nil {
		summary["skipped"] = lines
	}

	return summary
}

// countLines counts the number of lines in a file
func countLines(filepath string) (int, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			count++
		}
	}

	return count, scanner.Err()
}

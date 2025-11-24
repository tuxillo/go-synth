package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// AskYN prompts the user for yes/no confirmation
func AskYN(prompt string, defaultYes bool) bool {
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

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	cmd := exec.Command("cp", "-p", src, dst)
	return cmd.Run()
}

// CopyDir recursively copies a directory
func CopyDir(src, dst string) error {
	cmd := exec.Command("cp", "-Rp", src, dst)
	return cmd.Run()
}

// RemoveAll removes a directory tree with retries
func RemoveAll(path string) error {
	// Try regular remove first
	if err := os.RemoveAll(path); err == nil {
		return nil
	}

	// If that fails, use rm -rf
	cmd := exec.Command("rm", "-rf", path)
	return cmd.Run()
}

// Glob is a wrapper around filepath.Glob
func Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

// GetSwapUsage returns swap usage percentage
func GetSwapUsage() (float64, bool) {
	// This would need platform-specific implementation
	// For now, return 0
	return 0.0, false
}

// SetNice sets the nice value for current process
func SetNice(nice int) error {
	return syscall.Setpriority(syscall.PRIO_PROCESS, 0, nice)
}

// Repeat repeats a string n times
func Repeat(s string, n int) string {
	return strings.Repeat(s, n)
}

// RunCommand runs a command and returns error if it fails
func RunCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunCommandQuiet runs a command without output
func RunCommandQuiet(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

// RunCommandOutput runs a command and returns output
func RunCommandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// MkdirAll creates a directory and all parents
func MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// WriteFile writes data to a file
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

// ReadFile reads data from a file
func ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// Contains checks if a string slice contains a value
func Contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max returns the maximum of two integers
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// FormatBytes formats bytes as human-readable string
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
		if exp >= 5 { // Limit to PB
			break
		}
	}
	units := "KMGTPE"
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), units[exp])
}

// FormatDuration formats a duration as human-readable string
func FormatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	seconds = seconds % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	hours := minutes / 60
	minutes = minutes % 60
	return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
}

// EnsureDir ensures a directory exists, creating it if needed
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// Chdir changes the current directory
func Chdir(dir string) error {
	return os.Chdir(dir)
}

// Getwd gets the current working directory
func Getwd() (string, error) {
	return os.Getwd()
}

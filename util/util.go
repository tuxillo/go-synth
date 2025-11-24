package util

import (
	"fmt"
	"os"
	"os/exec"
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

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	cmd := exec.Command("cp", "-p", src, dst)
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
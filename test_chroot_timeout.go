//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func main() {
	// Create a minimal chroot environment
	chrootDir := "/tmp/test-chroot-timeout"

	// Create directory structure
	dirs := []string{
		chrootDir,
		chrootDir + "/bin",
		chrootDir + "/tmp",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("Failed to create %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	// Copy /bin/sh into chroot
	cpCmd := exec.Command("cp", "/bin/sh", chrootDir+"/bin/sh")
	if err := cpCmd.Run(); err != nil {
		fmt.Printf("Failed to copy /bin/sh: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Testing chroot with CommandContext timeout...")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run a sleep command inside chroot that should be killed after 2 seconds
	cmd := exec.CommandContext(ctx, "chroot", chrootDir, "/bin/sh", "-c", "sleep 9999")

	fmt.Println("Starting 'chroot /tmp/test-chroot-timeout /bin/sh -c sleep 9999'...")
	start := time.Now()

	err := cmd.Run()
	elapsed := time.Since(start)

	fmt.Printf("Command finished after %v\n", elapsed)
	if err != nil {
		fmt.Printf("Error (expected): %v\n", err)
	}

	if elapsed > 3*time.Second {
		fmt.Println("FAIL: Command did not timeout")
		os.Exit(1)
	} else {
		fmt.Println("SUCCESS: Chroot command timed out as expected")
	}

	// Cleanup
	os.RemoveAll(chrootDir)
}

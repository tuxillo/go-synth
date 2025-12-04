//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

func main() {
	fmt.Println("Testing CommandContext timeout...")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run a sleep command that should be killed after 2 seconds
	cmd := exec.CommandContext(ctx, "sleep", "9999")

	fmt.Println("Starting sleep 9999...")
	start := time.Now()

	err := cmd.Run()
	elapsed := time.Since(start)

	fmt.Printf("Command finished after %v\n", elapsed)
	if err != nil {
		fmt.Printf("Error (expected): %v\n", err)
	}

	if elapsed > 3*time.Second {
		fmt.Println("FAIL: Command did not timeout")
	} else {
		fmt.Println("SUCCESS: Command timed out as expected")
	}
}

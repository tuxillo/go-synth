//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"go-synth/config"
	"go-synth/environment"
	"go-synth/environment/bsd"
	"go-synth/log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	cfg := &config.Config{
		BuildBase:     "/tmp/test-sleep-env/build",
		SystemPath:    "/",
		DistFilesPath: "/tmp/test-sleep-env/distfiles",
		PackagesPath:  "/tmp/test-sleep-env/packages",
		DPortsPath:    "/tmp/test-sleep-env/dports",
		OptionsPath:   "/tmp/test-sleep-env/options",
	}

	for _, dir := range []string{cfg.DistFilesPath, cfg.PackagesPath, cfg.DPortsPath, cfg.OptionsPath} {
		os.MkdirAll(dir, 0755)
	}

	templateDir := filepath.Join(cfg.BuildBase, "Template/etc")
	os.MkdirAll(templateDir, 0755)
	os.WriteFile(filepath.Join(templateDir, "passwd"), []byte("root:*:0:0::0:0:root:/root:/bin/sh\n"), 0644)

	env := bsd.NewBSDEnvironment()
	defer env.Cleanup()

	fmt.Println("Setting up environment...")
	if err := env.Setup(1, cfg, log.NoOpLogger{}); err != nil {
		fmt.Printf("Setup failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Setup complete")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var stdout strings.Builder
	fmt.Println("Executing: sleep 10 (should timeout after 3s)...")
	start := time.Now()
	result, err := env.Execute(ctx, &environment.ExecCommand{
		Command: "/bin/sleep",
		Args:    []string{"10"},
		Stdout:  &stdout,
		Stderr:  &stdout,
	})
	elapsed := time.Since(start)

	fmt.Printf("Execute returned after %v\n", elapsed)
	if err != nil {
		fmt.Printf("Error (expected): %v\n", err)
	}
	if result != nil {
		fmt.Printf("Exit code: %d\n", result.ExitCode)
	}
	fmt.Printf("Output: %s\n", stdout.String())

	if elapsed > 4*time.Second {
		fmt.Println("FAIL: Did not timeout")
		os.Exit(1)
	}
	fmt.Println("SUCCESS: Timed out as expected")
}

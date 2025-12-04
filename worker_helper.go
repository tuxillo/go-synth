//go:build dragonfly || freebsd
// +build dragonfly freebsd

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"go-synth/environment/bsd"
)

// workerHelperArgs holds the arguments passed to worker helper mode
type workerHelperArgs struct {
	chrootPath string
	workDir    string
	command    string
	args       []string
	timeout    time.Duration
}

// parseWorkerHelperArgs parses command-line arguments for worker helper mode.
//
// Expected format:
//
//	go-synth --worker-helper --chroot=/path --workdir=/dir --timeout=5m -- /usr/bin/make arg1 arg2
//
// Everything after -- is the actual command to execute.
func parseWorkerHelperArgs() (*workerHelperArgs, error) {
	fs := flag.NewFlagSet("worker-helper", flag.ExitOnError)

	chrootPath := fs.String("chroot", "", "Chroot path (required)")
	workDir := fs.String("workdir", "", "Working directory inside chroot")
	timeout := fs.Duration("timeout", 0, "Command timeout (0 = no timeout)")

	// Parse flags up to --
	args := os.Args[1:] // Skip program name

	// Skip the --worker-helper flag itself (already processed in main.go)
	if len(args) > 0 && args[0] == "--worker-helper" {
		args = args[1:]
	}

	dashDashIdx := -1
	for i, arg := range args {
		if arg == "--" {
			dashDashIdx = i
			break
		}
	}

	if dashDashIdx == -1 {
		return nil, fmt.Errorf("missing -- separator before command")
	}

	// Parse flags before --
	if err := fs.Parse(args[:dashDashIdx]); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	// Everything after -- is the command
	commandArgs := args[dashDashIdx+1:]
	if len(commandArgs) == 0 {
		return nil, fmt.Errorf("no command specified after --")
	}

	if *chrootPath == "" {
		return nil, fmt.Errorf("--chroot is required")
	}

	return &workerHelperArgs{
		chrootPath: *chrootPath,
		workDir:    *workDir,
		command:    commandArgs[0],
		args:       commandArgs[1:],
		timeout:    *timeout,
	}, nil
}

// runWorkerHelper executes the worker helper mode.
//
// Lifecycle:
//  1. Parse arguments
//  2. Acquire reaper status (PROC_REAP_ACQUIRE)
//  3. Enter chroot
//  4. Execute the phase command
//  5. On exit, kill all descendants (PROC_REAP_KILL)
//  6. Return with same exit code as phase command
func runWorkerHelper() int {
	args, err := parseWorkerHelperArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "worker-helper: argument error: %v\n", err)
		return 1
	}

	// Step 1: Become a reaper
	if err := bsd.BecomeReaper(); err != nil {
		// Non-fatal: continue without reaper status but log warning
		fmt.Fprintf(os.Stderr, "worker-helper: warning: failed to acquire reaper status: %v\n", err)
		fmt.Fprintf(os.Stderr, "worker-helper: continuing without reaper (orphan cleanup will be limited)\n")
	}

	// Open /dev/null BEFORE chroot for stdin redirection
	// (chroot environments may not have /dev mounted)
	devNull, err := os.Open("/dev/null")
	if err != nil {
		fmt.Fprintf(os.Stderr, "worker-helper: failed to open /dev/null: %v\n", err)
		return 1
	}
	defer devNull.Close()

	// Step 2: Enter chroot
	if err := syscall.Chroot(args.chrootPath); err != nil {
		fmt.Fprintf(os.Stderr, "worker-helper: failed to chroot to %s: %v\n", args.chrootPath, err)
		return 1
	}

	// Change to working directory inside chroot
	workDir := args.workDir
	if workDir == "" {
		workDir = "/"
	}
	if err := os.Chdir(workDir); err != nil {
		fmt.Fprintf(os.Stderr, "worker-helper: failed to chdir to %s: %v\n", workDir, err)
		return 1
	}

	// Step 3: Execute the phase command
	ctx := context.Background()
	if args.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, args.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, args.command, args.args...)
	// Use /dev/null for stdin (opened before chroot)
	cmd.Stdin = devNull
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group for signal isolation
	}

	if err := cmd.Run(); err != nil {
		// Step 4: Kill all descendants before returning error
		if killErr := bsd.ReapAll(); killErr != nil {
			fmt.Fprintf(os.Stderr, "worker-helper: warning: failed to kill descendants: %v\n", killErr)
		}

		// Return the command's exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}

		// Other errors (couldn't start, etc.)
		fmt.Fprintf(os.Stderr, "worker-helper: command execution error: %v\n", err)
		return 1
	}

	// Step 5: Success - still kill descendants before exit (in case any background processes)
	if killErr := bsd.ReapAll(); killErr != nil {
		fmt.Fprintf(os.Stderr, "worker-helper: warning: failed to kill descendants: %v\n", killErr)
	}

	return 0
}

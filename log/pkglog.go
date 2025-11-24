package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Update the repeat function reference
func (pl *PackageLogger) WriteHeader() {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("=", 70))
	fmt.Fprintf(pl.file, "Build Log: %s\n", pl.portDir)
	fmt.Fprintf(pl.file, "Started: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(pl.file, "%s\n\n", strings.Repeat("=", 70))
	pl.file.Sync()
}

func (pl *PackageLogger) WritePhase(phase string) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	fmt.Fprintf(pl.file, "\n")
	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("=", 70))
	fmt.Fprintf(pl.file, "Phase: %s\n", phase)
	fmt.Fprintf(pl.file, "Time: %s\n", time.Now().Format("15:04:05"))
	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("=", 70))
	pl.file.Sync()
}

func (pl *PackageLogger) WriteSuccess(duration time.Duration) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	fmt.Fprintf(pl.file, "\n")
	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("=", 70))
	fmt.Fprintf(pl.file, "BUILD SUCCESS\n")
	fmt.Fprintf(pl.file, "Completed: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(pl.file, "Duration: %s\n", duration)
	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("=", 70))
	pl.file.Sync()
}

func (pl *PackageLogger) WriteFailure(duration time.Duration, reason string) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	fmt.Fprintf(pl.file, "\n")
	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("=", 70))
	fmt.Fprintf(pl.file, "BUILD FAILED\n")
	fmt.Fprintf(pl.file, "Reason: %s\n", reason)
	fmt.Fprintf(pl.file, "Completed: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(pl.file, "Duration: %s\n", duration)
	fmt.Fprintf(pl.file, "%s\n", strings.Repeat("=", 70))
	pl.file.Sync()
}
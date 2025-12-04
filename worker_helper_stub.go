//go:build !dragonfly && !freebsd
// +build !dragonfly,!freebsd

package main

import (
	"fmt"
	"os"
)

// runWorkerHelper is a stub for non-BSD platforms.
// Worker helper mode is only supported on DragonFly BSD and FreeBSD.
func runWorkerHelper() int {
	fmt.Fprintln(os.Stderr, "worker-helper mode is only supported on DragonFly BSD and FreeBSD")
	return 1
}

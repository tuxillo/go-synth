//go:build !dragonfly && !freebsd

package stats

// getAdjustedLoad returns the 1-minute load average.
// This is a stub for non-BSD systems.
func getAdjustedLoad() (float64, error) {
	// TODO: Implement for Linux using /proc/loadavg
	return 0.0, nil
}

// getSwapUsage returns swap usage as a percentage (0-100).
// This is a stub for non-BSD systems.
func getSwapUsage() (int, error) {
	// TODO: Implement for Linux using /proc/meminfo
	return 0, nil
}

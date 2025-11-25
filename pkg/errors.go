package pkg

import "fmt"

// Sentinel errors - simple error constants that can be checked with errors.Is()
var (
	// ErrCycleDetected is returned when a circular dependency is found
	// during topological sorting of the dependency graph.
	ErrCycleDetected = fmt.Errorf("circular dependency detected")

	// ErrInvalidSpec is returned when a port specification is malformed
	// or cannot be parsed correctly.
	ErrInvalidSpec = fmt.Errorf("invalid port specification")

	// ErrPortNotFound is returned when a port doesn't exist in the ports tree.
	// This is the base error for PortNotFoundError.
	ErrPortNotFound = fmt.Errorf("port not found in ports tree")

	// ErrNoValidPorts is returned when no valid ports are found in the input
	// specification list after parsing.
	ErrNoValidPorts = fmt.Errorf("no valid ports found")

	// ErrEmptySpec is returned when an empty port specification list is provided.
	ErrEmptySpec = fmt.Errorf("empty port specification list")
)

// PortNotFoundError wraps port-specific not found errors with detailed context
// about which port was not found and where it was expected to be.
//
// This error type allows callers to extract the specific port that failed
// and the path that was checked.
type PortNotFoundError struct {
	// PortSpec is the port specification that was not found (e.g., "editors/vim")
	PortSpec string

	// Path is the filesystem path where the port was expected to be
	Path string
}

// Error implements the error interface
func (e *PortNotFoundError) Error() string {
	return fmt.Sprintf("port not found: %s (path: %s)", e.PortSpec, e.Path)
}

// Unwrap allows errors.Is(err, ErrPortNotFound) to work correctly
func (e *PortNotFoundError) Unwrap() error {
	return ErrPortNotFound
}

// CycleError wraps cycle detection errors with detailed information about
// the circular dependency that was found.
//
// This error includes the total number of packages in the graph, how many
// were successfully ordered before the cycle was detected, and optionally
// the specific packages involved in the cycle.
type CycleError struct {
	// TotalPackages is the total number of packages in the dependency graph
	TotalPackages int

	// OrderedPackages is the number of packages that were successfully ordered
	// before the cycle was detected (will be less than TotalPackages)
	OrderedPackages int

	// CyclePackages contains the packages involved in the cycle, if identified.
	// This field may be nil if the specific cycle packages were not tracked.
	CyclePackages []*Package
}

// Error implements the error interface
func (e *CycleError) Error() string {
	return fmt.Sprintf("cycle detected: only %d of %d packages ordered",
		e.OrderedPackages, e.TotalPackages)
}

// Unwrap allows errors.Is(err, ErrCycleDetected) to work correctly
func (e *CycleError) Unwrap() error {
	return ErrCycleDetected
}

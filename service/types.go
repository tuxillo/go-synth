package service

import (
	"time"

	"go-synth/build"
	"go-synth/builddb"
	"go-synth/pkg"
)

// BuildOptions contains options for the Build service.
type BuildOptions struct {
	PortList  []string // List of ports to build
	Force     bool     // Force rebuild even if up-to-date
	JustBuild bool     // Skip pre-build checks
	TestMode  bool     // Enable test mode
}

// BuildResult contains the results of a build operation.
type BuildResult struct {
	Stats     *build.BuildStats // Build statistics
	Packages  []*pkg.Package    // All packages (including dependencies)
	NeedBuild int               // Number of packages that need building
	Duration  time.Duration     // Total build duration
	Cleanup   func()            // Cleanup function for caller to manage worker environments
}

// InitOptions contains options for the Initialize service.
type InitOptions struct {
	AutoMigrate     bool // Automatically migrate legacy CRC data if found
	SkipSystemFiles bool // Skip copying system files (for testing)
}

// InitResult contains the results of an initialization operation.
type InitResult struct {
	DirsCreated        []string // List of directories created
	TemplateCreated    bool     // Whether template directory was created
	DatabaseInitalized bool     // Whether database was initialized
	MigrationNeeded    bool     // Whether legacy CRC migration is needed
	MigrationPerformed bool     // Whether migration was performed
	PortsFound         int      // Number of entries found in ports directory
	Warnings           []string // Non-fatal warnings
}

// StatusOptions contains options for the GetStatus service.
type StatusOptions struct {
	PortList []string // List of ports to check status for (empty = all)
}

// StatusResult contains the results of a status query.
type StatusResult struct {
	Ports        []PortStatus     // Status of individual ports
	DatabaseSize int64            // Size of BuildDB in bytes
	Stats        *builddb.DBStats // Database statistics
}

// PortStatus contains status information for a single port.
type PortStatus struct {
	PortDir    string               // Port directory (e.g., "editors/vim")
	Version    string               // Port version
	LastBuild  *builddb.BuildRecord // Most recent build record (nil if never built)
	NeedsBuild bool                 // Whether port needs rebuilding
	CRC        uint32               // Current CRC value
}

// CleanupOptions contains options for the Cleanup service.
type CleanupOptions struct {
	Force bool // Force cleanup even if mounts are in use
}

// CleanupResult contains the results of a cleanup operation.
type CleanupResult struct {
	WorkersCleaned int     // Number of workers cleaned up
	Errors         []error // Non-fatal errors encountered
}

// DatabaseOptions contains options for database operations.
type DatabaseOptions struct {
	Backup bool // Create backup before operation
	Force  bool // Force operation without confirmation
}

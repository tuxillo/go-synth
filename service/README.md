# Service Layer

The service layer provides a clean, reusable API for go-synth's core functionality. It separates business logic from CLI presentation, making it suitable for use in REST APIs, GUIs, or other frontends.

## Architecture

```
┌─────────────────────────────────────┐
│   CLI Layer (main.go - 655 lines)  │
│   • User interaction only           │
│   • Flag parsing & display          │
│   • Signal handling (Ctrl+C)        │
└──────────────┬──────────────────────┘
               │ Calls service methods
               ↓
┌─────────────────────────────────────┐
│  Service Layer (service/ - 969 lines)│
│   • All business logic              │
│   • 64.3% test coverage             │
│   • 47 unit tests (1,845 lines)     │
│   • REST API-ready                  │
└──────────────┬──────────────────────┘
               │ Calls library functions
               ↓
┌─────────────────────────────────────┐
│ Library Layer (pkg/, build/, etc.)  │
│   • Core functionality              │
│   • Package parsing & resolution    │
│   • Build execution                 │
└─────────────────────────────────────┘
```

## Design Principles

1. **No User Interaction** - Service methods never prompt the user or read from stdin
2. **Structured Results** - All methods return structured data (not formatted strings)
3. **Error Propagation** - Errors are returned, not printed to stderr
4. **Stateless Operations** - Each method is independent (except for service lifecycle)
5. **Testable** - All functionality covered by unit tests

## Package Structure

```
service/
├── service.go       (120 lines) - Core service lifecycle
├── init.go          (198 lines) - Initialization logic
├── build.go         (242 lines) - Build orchestration
├── status.go        (111 lines) - Status queries
├── cleanup.go       (108 lines) - Worker cleanup
├── database.go      (110 lines) - Database operations
├── types.go         (80 lines)  - Type definitions
└── *_test.go        (1,845 lines) - Comprehensive tests
```

## Quick Start

### Basic Usage

```go
import (
    "go-synth/config"
    "go-synth/service"
)

// 1. Load configuration
cfg, err := config.Load("/etc/dsynth/dsynth.ini")
if err != nil {
    return err
}

// 2. Create service
svc, err := service.NewService(cfg)
if err != nil {
    return err
}
defer svc.Close()

// 3. Use service methods
result, err := svc.GetStatus(service.StatusOptions{})
if err != nil {
    return err
}

// 4. Process results (no formatting in service layer)
for _, port := range result.Ports {
    fmt.Printf("%s: %s\n", port.PortDir, port.Version)
}
```

## Service API

### Service Lifecycle

#### NewService(cfg *config.Config) (*Service, error)
Creates a new service instance with initialized logger and database.

```go
svc, err := service.NewService(cfg)
if err != nil {
    return fmt.Errorf("failed to create service: %w", err)
}
defer svc.Close()
```

#### Close() error
Closes the service and cleans up resources (database connections, log files).

```go
defer svc.Close()
```

### Initialization

#### Initialize(opts InitOptions) (*InitResult, error)
Sets up the go-synth environment for the first time.

**Features:**
- Creates directory structure (build base, logs, packages, etc.)
- Sets up template directory with system files
- Initializes build database
- Optionally migrates legacy CRC data
- Verifies ports directory

**Options:**
- `AutoMigrate` - Automatically migrate legacy CRC data if found
- `SkipSystemFiles` - Skip copying system files (for testing)

**Example:**
```go
result, err := svc.Initialize(service.InitOptions{
    AutoMigrate: true,
})
if err != nil {
    return err
}

// Check what was created
fmt.Printf("Created %d directories\n", len(result.DirsCreated))
if result.MigrationPerformed {
    fmt.Println("Legacy CRC data migrated")
}
if result.PortsFound == 0 {
    fmt.Println("Warning: No ports found in tree")
}
```

#### NeedsMigration() bool
Checks if legacy CRC migration is needed without initializing anything.

```go
if svc.NeedsMigration() {
    fmt.Println("Legacy CRC data detected")
}
```

#### GetLegacyCRCFile() (string, error)
Returns the path to the legacy CRC file if it exists.

```go
legacyFile, err := svc.GetLegacyCRCFile()
if err != nil {
    return err
}
if legacyFile != "" {
    fmt.Printf("Found legacy file: %s\n", legacyFile)
}
```

### Status Queries

#### GetStatus(opts StatusOptions) (*StatusResult, error)
Queries build status for ports.

**Options:**
- `PortList` - List of specific ports to query (empty = all ports with build records)

**Example:**
```go
// Get overall status
result, err := svc.GetStatus(service.StatusOptions{})
if err != nil {
    return err
}

fmt.Printf("Database size: %d bytes\n", result.DatabaseSize)
fmt.Printf("Total builds: %d\n", result.Stats.TotalBuilds)

// Get status for specific ports
result, err = svc.GetStatus(service.StatusOptions{
    PortList: []string{"editors/vim", "shells/bash"},
})
for _, port := range result.Ports {
    if port.LastBuild != nil {
        fmt.Printf("%s: built at %s\n", port.PortDir, port.LastBuild.BuildTime)
    } else {
        fmt.Printf("%s: never built\n", port.PortDir)
    }
}
```

#### GetDatabaseStats() (*builddb.DBStats, error)
Returns detailed database statistics.

```go
stats, err := svc.GetDatabaseStats()
if err != nil {
    return err
}
fmt.Printf("Total builds: %d\n", stats.TotalBuilds)
fmt.Printf("Unique ports: %d\n", stats.UniquePackages)
```

#### GetPortStatus(portDir string) (*PortStatus, error)
Gets status for a single port.

```go
status, err := svc.GetPortStatus("editors/vim")
if err != nil {
    return err
}
if status.NeedsBuild {
    fmt.Println("Port needs rebuilding")
}
```

### Build Operations

#### Build(opts BuildOptions) (*BuildResult, error)
Orchestrates the complete build workflow.

**Options:**
- `PortList` - List of ports to build
- `Force` - Force rebuild even if up-to-date
- `JustBuild` - Skip pre-build checks
- `TestMode` - Enable test mode

**Features:**
- Automatic migration of legacy CRC data (if configured)
- Package parsing and dependency resolution
- CRC-based incremental builds
- Worker orchestration
- Build environment cleanup

**Example:**
```go
result, err := svc.Build(service.BuildOptions{
    PortList: []string{"editors/vim", "shells/bash"},
    Force:    false,
})
if err != nil {
    return err
}

fmt.Printf("Total: %d\n", result.Stats.Total)
fmt.Printf("Success: %d\n", result.Stats.Success)
fmt.Printf("Failed: %d\n", result.Stats.Failed)
fmt.Printf("Duration: %s\n", result.Duration)
```

#### GetBuildPlan(portList []string) (*BuildPlan, error)
Returns information about what would be built without actually building.

**Example:**
```go
plan, err := svc.GetBuildPlan([]string{"editors/vim"})
if err != nil {
    return err
}

fmt.Printf("Total packages: %d\n", plan.TotalPackages)
fmt.Printf("To build: %d\n", plan.NeedBuild)
fmt.Printf("Already built: %d\n", len(plan.ToSkip))

for _, port := range plan.ToBuild {
    fmt.Printf("  - %s (needs building)\n", port)
}
```

#### CheckMigrationStatus() (*MigrationStatus, error)
Checks if legacy CRC migration is needed.

```go
status, err := svc.CheckMigrationStatus()
if err != nil {
    return err
}
if status.Needed {
    fmt.Printf("Migration needed: %s\n", status.LegacyFile)
}
```

#### PerformMigration() error
Manually triggers legacy CRC migration.

```go
if err := svc.PerformMigration(); err != nil {
    return fmt.Errorf("migration failed: %w", err)
}
```

### Cleanup

#### Cleanup(opts CleanupOptions) (*CleanupResult, error)
Cleans up stale worker environments.

**Options:**
- `Force` - Force cleanup even if workers appear active

**Example:**
```go
result, err := svc.Cleanup(service.CleanupOptions{})
if err != nil {
    return err
}

fmt.Printf("Cleaned up %d workers\n", result.WorkersCleaned)
for _, cleanupErr := range result.Errors {
    fmt.Fprintf(os.Stderr, "Warning: %v\n", cleanupErr)
}
```

#### GetWorkerDirectories() ([]string, error)
Returns list of worker directories that exist.

```go
workers, err := svc.GetWorkerDirectories()
if err != nil {
    return err
}
fmt.Printf("Found %d worker directories\n", len(workers))
```

### Database Operations

#### DatabaseExists() bool
Checks if the build database file exists.

```go
if !svc.DatabaseExists() {
    fmt.Println("No database found - run 'init' first")
    return
}
```

#### GetDatabasePath() string
Returns the path to the build database.

```go
fmt.Printf("Database: %s\n", svc.GetDatabasePath())
```

#### BackupDatabase() (string, error)
Creates a backup of the database.

```go
backupPath, err := svc.BackupDatabase()
if err != nil {
    return err
}
fmt.Printf("Backup created: %s\n", backupPath)
```

#### ResetDatabase() (*DatabaseResetResult, error)
Deletes the build database and legacy files.

**Example:**
```go
result, err := svc.ResetDatabase()
if err != nil {
    return err
}

if result.DatabaseRemoved {
    fmt.Println("Database reset")
}
for _, file := range result.FilesRemoved {
    fmt.Printf("Removed: %s\n", file)
}
```

### Configuration Access

#### Config() *config.Config
Returns the service's configuration.

```go
cfg := svc.Config()
fmt.Printf("Build base: %s\n", cfg.BuildBase)
```

#### Logger() log.LibraryLogger
Returns the service's logger.

```go
logger := svc.Logger()
logger.Info("Custom log message")
```

#### Database() *builddb.DB
Returns the service's database instance.

```go
db := svc.Database()
// Use database directly if needed
```

## Error Handling

All service methods return errors that should be checked and handled appropriately:

```go
result, err := svc.Initialize(service.InitOptions{})
if err != nil {
    // Error occurred - display to user and exit
    fmt.Fprintf(os.Stderr, "Initialization failed: %v\n", err)
    os.Exit(1)
}

// Success - process result
if len(result.Warnings) > 0 {
    for _, warning := range result.Warnings {
        fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
    }
}
```

## Testing

The service layer has comprehensive unit tests with 64.3% coverage:

```bash
# Run all service tests
go test ./service

# Run with coverage
go test ./service -cover

# Run specific test
go test ./service -run TestInitialize

# Verbose output
go test ./service -v
```

### Test Structure

Each service file has a corresponding test file:
- `service_test.go` - Service lifecycle tests
- `init_test.go` - Initialization tests
- `build_test.go` - Build orchestration tests
- `status_test.go` - Status query tests
- `cleanup_test.go` - Cleanup tests
- `database_test.go` - Database operation tests

### Writing Tests

Tests use temporary directories and don't require actual ports:

```go
func TestMyFeature(t *testing.T) {
    tmpDir := t.TempDir()
    
    cfg := &config.Config{
        BuildBase:  tmpDir,
        LogsPath:   filepath.Join(tmpDir, "logs"),
        // ... other required paths
    }
    cfg.Database.Path = filepath.Join(tmpDir, "build.db")
    
    // Create logs directory (required)
    os.MkdirAll(cfg.LogsPath, 0755)
    
    svc, err := service.NewService(cfg)
    if err != nil {
        t.Fatalf("NewService() failed: %v", err)
    }
    defer svc.Close()
    
    // Test your feature
    result, err := svc.SomeMethod()
    if err != nil {
        t.Fatalf("SomeMethod() failed: %v", err)
    }
    
    // Assertions
    if result.Expected != "value" {
        t.Errorf("got %v, want %v", result.Expected, "value")
    }
}
```

## Future Enhancements

### Planned Features
- [ ] Async build with progress callbacks
- [ ] Concurrent cleanup operations
- [ ] Database corruption recovery
- [ ] Build result streaming
- [ ] Port dependency visualization

### Integration with REST API (Phase 5)

The service layer is designed for easy integration with REST APIs:

```go
// Example HTTP handler
func handleBuild(w http.ResponseWriter, r *http.Request) {
    var req BuildRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    svc, err := service.NewService(cfg)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer svc.Close()
    
    result, err := svc.Build(service.BuildOptions{
        PortList: req.Ports,
        Force:    req.Force,
    })
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(result)
}
```

## Migration Guide

### From Direct Library Calls

**Before (direct library usage):**
```go
// CLI code mixed with business logic
packages, err := pkg.ParsePortList(portList, cfg, registry, pkgRegistry, logger)
if err != nil {
    fmt.Fprintf(os.Stderr, "Parse failed: %v\n", err)
    os.Exit(1)
}

err = pkg.ResolveDependencies(packages, cfg, registry, pkgRegistry, logger)
if err != nil {
    fmt.Fprintf(os.Stderr, "Resolve failed: %v\n", err)
    os.Exit(1)
}

needBuild, err := pkg.MarkPackagesNeedingBuild(packages, cfg, registry, db, logger)
// ... many more lines of orchestration
```

**After (service layer):**
```go
// Clean separation of concerns
svc, err := service.NewService(cfg)
if err != nil {
    fmt.Fprintf(os.Stderr, "Service init failed: %v\n", err)
    os.Exit(1)
}
defer svc.Close()

result, err := svc.Build(service.BuildOptions{
    PortList: portList,
})
if err != nil {
    fmt.Fprintf(os.Stderr, "Build failed: %v\n", err)
    os.Exit(1)
}

// Display results
fmt.Printf("Success: %d, Failed: %d\n", result.Stats.Success, result.Stats.Failed)
```

## See Also

- [DEVELOPMENT.md](../DEVELOPMENT.md) - Overall development guide
- [PHASE_1_DEVELOPER_GUIDE.md](../PHASE_1_DEVELOPER_GUIDE.md) - pkg library usage
- [TESTING.md](../TESTING.md) - Testing guidelines
- [examples/](../examples/) - Usage examples for pkg library

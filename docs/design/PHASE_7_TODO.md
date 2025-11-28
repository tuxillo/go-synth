# Phase 7: Integration & Migration - CLI Integration

**Phase**: 7 of 7 (Final)  
**Status**: ⚪ Planned  
**Dependencies**: Phases 1-6 complete  
**Estimated Effort**: ~12 hours  
**Priority**: Critical (Completes MVP)

## Overview

Phase 7 is the **final phase** that wires all new components (`pkg`, `builddb`, `build`, 
`environment`) into the existing CLI, provides migration from legacy CRC files to BuildDB, 
and ensures backward compatibility. This completes the go-synth MVP.

### Goals

- **Wire New Pipeline**: Integrate pkg → builddb → build → environment into CLI
- **Migration Path**: Smooth transition from legacy file-based CRC to BuildDB
- **Backward Compatibility**: Existing workflows continue to work
- **Command Mapping**: Map `dsynth` commands to new pipeline
- **Logging Integration**: Maintain existing log format, add UUID tracking

### Non-Goals (MVP)

- ❌ Ncurses UI (Phase 1.5 or later)
- ❌ Advanced migration tooling (simple is sufficient)
- ❌ Configuration migration (config stays compatible)
- ❌ Performance tuning (functional first, optimize later)

---

## Implementation Tasks

### Task 1: Create Migration Package ✅ COMPLETE

**Actual Time**: 2 hours  
**Priority**: High  
**Dependencies**: None  
**Completed**: 2025-11-28  
**Commit**: dbde074

#### Description

Created migration utilities to import legacy CRC data from file-based format into BuildDB. 
This ensures users can upgrade without losing build history.

**Implementation delivered**:
- MigrateLegacyCRC() - imports CRC records from ${BuildBase}/crc_index
- readLegacyCRCFile() - parses legacy format (portdir:crc32_hex)
- DetectMigrationNeeded() - checks if migration required
- 7 comprehensive test functions (all passing)
- Coverage: 87.0%, race detector clean

#### Implementation Steps

1. **Create `migration/migration.go`**:
   ```go
   // Package migration provides utilities for migrating from legacy
   // file-based CRC storage to the new BuildDB format.
   package migration
   
   import (
       "bufio"
       "fmt"
       "os"
       "path/filepath"
       "strings"
       
       "dsynth/builddb"
       "dsynth/config"
   )
   
   // CRCRecord represents a legacy CRC entry from file
   type CRCRecord struct {
       PortDir string
       CRC     uint32
   }
   
   // MigrateLegacyCRC imports CRC data from legacy file format into BuildDB
   func MigrateLegacyCRC(cfg *config.Config, db *builddb.DB) error {
       legacyFile := filepath.Join(cfg.System.BuildBase, "crc_index")
       
       // Check if legacy file exists
       if _, err := os.Stat(legacyFile); os.IsNotExist(err) {
           // No legacy data, nothing to migrate
           return nil
       }
       
       fmt.Printf("Found legacy CRC file: %s\n", legacyFile)
       
       // Read legacy file
       records, err := readLegacyCRCFile(legacyFile)
       if err != nil {
           return fmt.Errorf("failed to read legacy CRC file: %w", err)
       }
       
       fmt.Printf("Migrating %d CRC records...\n", len(records))
       
       // Import into BuildDB
       migrated := 0
       for _, rec := range records {
           if err := db.UpdateCRC(rec.PortDir, rec.CRC); err != nil {
               fmt.Fprintf(os.Stderr, "Warning: failed to migrate %s: %v\n",
                   rec.PortDir, err)
               continue
           }
           migrated++
       }
       
       fmt.Printf("Successfully migrated %d/%d records\n", migrated, len(records))
       
       // Backup legacy file
       backupFile := legacyFile + ".bak"
       if err := os.Rename(legacyFile, backupFile); err != nil {
           fmt.Fprintf(os.Stderr, "Warning: failed to backup legacy file: %v\n", err)
       } else {
           fmt.Printf("Legacy file backed up to: %s\n", backupFile)
       }
       
       return nil
   }
   
   // readLegacyCRCFile parses the legacy CRC file format
   func readLegacyCRCFile(path string) ([]CRCRecord, error) {
       file, err := os.Open(path)
       if err != nil {
           return nil, err
       }
       defer file.Close()
       
       var records []CRCRecord
       scanner := bufio.NewScanner(file)
       
       for scanner.Scan() {
           line := strings.TrimSpace(scanner.Text())
           if line == "" || strings.HasPrefix(line, "#") {
               continue
           }
           
           // Expected format: "portdir:crc32_hex"
           parts := strings.SplitN(line, ":", 2)
           if len(parts) != 2 {
               continue
           }
           
           portDir := parts[0]
           var crc uint32
           if _, err := fmt.Sscanf(parts[1], "%x", &crc); err != nil {
               fmt.Fprintf(os.Stderr, "Warning: invalid CRC for %s: %v\n",
                   portDir, err)
               continue
           }
           
           records = append(records, CRCRecord{
               PortDir: portDir,
               CRC:     crc,
           })
       }
       
       if err := scanner.Err(); err != nil {
           return nil, err
       }
       
       return records, nil
   }
   
   // DetectMigrationNeeded checks if migration is required
   func DetectMigrationNeeded(cfg *config.Config) bool {
       legacyFile := filepath.Join(cfg.System.BuildBase, "crc_index")
       _, err := os.Stat(legacyFile)
       return err == nil
   }
   ```

2. **Add migration test** in `migration/migration_test.go`:
   ```go
   package migration_test
   
   import (
       "os"
       "path/filepath"
       "testing"
       
       "dsynth/builddb"
       "dsynth/config"
       "dsynth/migration"
   )
   
   func TestMigrateLegacyCRC(t *testing.T) {
       tmpDir := t.TempDir()
       
       // Create legacy CRC file
       legacyFile := filepath.Join(tmpDir, "crc_index")
       legacyData := `# Legacy CRC index
   editors/vim:deadbeef
   devel/git:cafebabe
   `
       os.WriteFile(legacyFile, []byte(legacyData), 0644)
       
       // Setup config and database
       cfg := &config.Config{
           System: config.SystemConfig{
               BuildBase: tmpDir,
           },
       }
       
       db, _ := builddb.OpenDB(filepath.Join(tmpDir, "builds.db"))
       defer db.Close()
       
       // Run migration
       err := migration.MigrateLegacyCRC(cfg, db)
       if err != nil {
           t.Fatalf("MigrateLegacyCRC() failed: %v", err)
       }
       
       // Verify CRCs imported
       crc1, _ := db.GetCRC("editors/vim")
       if crc1 != 0xdeadbeef {
           t.Errorf("CRC mismatch for editors/vim: got %x, want deadbeef", crc1)
       }
       
       crc2, _ := db.GetCRC("devel/git")
       if crc2 != 0xcafebabe {
           t.Errorf("CRC mismatch for devel/git: got %x, want cafebabe", crc2)
       }
       
       // Verify legacy file backed up
       backupFile := legacyFile + ".bak"
       if _, err := os.Stat(backupFile); os.IsNotExist(err) {
           t.Error("Expected backup file to exist")
       }
   }
   ```

#### Testing Checklist

- [ ] Migration reads legacy CRC file
- [ ] CRC records imported into BuildDB
- [ ] Invalid lines skipped gracefully
- [ ] Legacy file backed up after migration
- [ ] Migration idempotent (safe to run multiple times)
- [ ] Empty legacy file handled correctly
- [ ] Missing legacy file returns no error

---

### Task 2: Wire CLI Build Commands ⚪

**Estimated Time**: 3 hours  
**Priority**: Critical  
**Dependencies**: Task 1

#### Description

Update CLI build commands (`build`, `just-build`, `force`) to use the new pipeline 
(pkg → builddb → build → environment).

#### Implementation Steps

1. **Update `main.go` build command handling**:
   ```go
   case "build":
       doBuild(cfg, commandArgs, false)
   case "just-build":
       doBuild(cfg, commandArgs, false)
   case "force":
       cfg.Force = true
       doBuild(cfg, commandArgs, false)
   case "rebuild":
       cfg.Force = true
       doBuild(cfg, commandArgs, true) // true = rebuild (skip deps)
   ```

2. **Implement `doBuild()` function**:
   ```go
   func doBuild(cfg *config.Config, portList []string, skipDeps bool) {
       // 1. Initialize logger
       logger, err := log.NewLogger(cfg)
       if err != nil {
           fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
           os.Exit(1)
       }
       defer logger.Close()
       
       // 2. Open BuildDB
       dbPath := filepath.Join(cfg.System.BuildBase, "builds.db")
       db, err := builddb.OpenDB(dbPath)
       if err != nil {
           fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
           os.Exit(1)
       }
       defer db.Close()
       
       // 3. Check for legacy CRC migration
       if migration.DetectMigrationNeeded(cfg) {
           fmt.Println("Detecting legacy CRC data...")
           if err := migration.MigrateLegacyCRC(cfg, db); err != nil {
               fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
               os.Exit(1)
           }
       }
       
       // 4. Parse port specifications
       logger.Info("Parsing port specifications...")
       pkgRegistry := pkg.NewPackageRegistry()
       stateRegistry := pkg.NewBuildStateRegistry()
       
       packages, err := pkg.ParsePortList(portList, cfg, stateRegistry, pkgRegistry)
       if err != nil {
           fmt.Fprintf(os.Stderr, "Failed to parse ports: %v\n", err)
           os.Exit(1)
       }
       
       // 5. Resolve dependencies (unless skipDeps)
       if !skipDeps {
           logger.Info("Resolving dependencies...")
           if err := pkg.ResolveDependencies(packages, cfg, stateRegistry, pkgRegistry); err != nil {
               fmt.Fprintf(os.Stderr, "Failed to resolve dependencies: %v\n", err)
               os.Exit(1)
           }
       }
       
       // 6. Display build plan
       buildOrder := pkg.GetBuildOrder(packages)
       fmt.Printf("\nBuild plan: %d packages\n", len(buildOrder))
       if len(buildOrder) <= 20 {
           for i, p := range buildOrder {
               fmt.Printf("  %d. %s\n", i+1, p.PortDir)
           }
       } else {
           fmt.Printf("  (showing first 10 of %d)\n", len(buildOrder))
           for i := 0; i < 10; i++ {
               fmt.Printf("  %d. %s\n", i+1, buildOrder[i].PortDir)
           }
       }
       
       // 7. Prompt for confirmation (unless -y)
       if !cfg.YesAll {
           fmt.Print("\nProceed with build? [y/N]: ")
           var response string
           fmt.Scanln(&response)
           if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
               fmt.Println("Build cancelled")
               os.Exit(0)
           }
       }
       
       // 8. Execute build
       fmt.Println("\nStarting build...")
       logger.Info("Build started")
       
       stats, cleanup, err := build.DoBuild(packages, cfg, logger, db)
       defer cleanup()
       
       if err != nil {
           fmt.Fprintf(os.Stderr, "\nBuild failed: %v\n", err)
           logger.Error("Build failed: %v", err)
           os.Exit(1)
       }
       
       // 9. Display results
       fmt.Printf("\n=== Build Complete ===\n")
       fmt.Printf("Total:   %d\n", stats.Total)
       fmt.Printf("Success: %d\n", stats.Success)
       fmt.Printf("Failed:  %d\n", stats.Failed)
       fmt.Printf("Skipped: %d\n", stats.Skipped)
       fmt.Printf("Ignored: %d\n", stats.Ignored)
       fmt.Printf("Duration: %s\n", stats.Duration)
       
       logger.Info("Build complete: %d success, %d failed, %d skipped",
           stats.Success, stats.Failed, stats.Skipped)
       
       if stats.Failed > 0 {
           os.Exit(1)
       }
   }
   ```

3. **Add signal handling** for graceful shutdown:
   ```go
   // In doBuild, before build.DoBuild
   sigChan := make(chan os.Signal, 1)
   signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
   
   go func() {
       <-sigChan
       fmt.Println("\nReceived interrupt, cleaning up...")
       cleanup()
       os.Exit(130) // 128 + SIGINT
   }()
   ```

#### Testing Checklist

- [ ] `dsynth build editors/vim` works end-to-end
- [ ] `dsynth force editors/vim` bypasses CRC check
- [ ] `-y` flag skips confirmation prompt
- [ ] `-d` flag enables debug logging
- [ ] Build plan displays correctly
- [ ] Statistics shown after build
- [ ] Exit code 0 on success, 1 on failure
- [ ] Ctrl-C gracefully cleans up workers
- [ ] Migration prompt appears on first run

---

### Task 3: Wire Other CLI Commands ⚪

**Estimated Time**: 2 hours  
**Priority**: Medium  
**Dependencies**: Task 2

#### Description

Update supporting commands (`status`, `reset-db`, `cleanup`) to work with new database.

#### Implementation Steps

1. **Update `doStatus()`**:
   ```go
   func doStatus(cfg *config.Config, portList []string) {
       dbPath := filepath.Join(cfg.System.BuildBase, "builds.db")
       db, err := builddb.OpenDB(dbPath)
       if err != nil {
           fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
           os.Exit(1)
       }
       defer db.Close()
       
       if len(portList) == 0 {
           // Show overall status
           fmt.Println("=== Build Database Status ===")
           // TODO: Add db.Stats() method to builddb
           fmt.Printf("Database: %s\n", dbPath)
       } else {
           // Show status for specific ports
           for _, portDir := range portList {
               rec, err := db.LatestFor(portDir, "")
               if err != nil {
                   fmt.Printf("%s: never built\n", portDir)
                   continue
               }
               
               fmt.Printf("%s:\n", portDir)
               fmt.Printf("  Status: %s\n", rec.Status)
               fmt.Printf("  UUID: %s\n", rec.UUID)
               fmt.Printf("  Last build: %s\n", rec.StartTime.Format("2006-01-02 15:04:05"))
           }
       }
   }
   ```

2. **Update `doResetDB()`**:
   ```go
   func doResetDB(cfg *config.Config) {
       dbPath := filepath.Join(cfg.System.BuildBase, "builds.db")
       
       // Confirm destructive operation
       if !cfg.YesAll {
           fmt.Printf("This will delete the build database: %s\n", dbPath)
           fmt.Print("Are you sure? [y/N]: ")
           var response string
           fmt.Scanln(&response)
           if !strings.EqualFold(response, "y") {
               fmt.Println("Cancelled")
               return
           }
       }
       
       // Remove database file
       if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
           fmt.Fprintf(os.Stderr, "Failed to remove database: %v\n", err)
           os.Exit(1)
       }
       
       fmt.Println("Build database reset successfully")
       
       // Also remove legacy CRC file if present
       legacyFile := filepath.Join(cfg.System.BuildBase, "crc_index")
       os.Remove(legacyFile) // Ignore errors
   }
   ```

3. **Update `doCleanup()`**:
   ```go
   func doCleanup(cfg *config.Config) {
       fmt.Println("Cleaning up worker mounts...")
       
       // TODO: Add mount.CleanupAllWorkers() function
       // This should:
       // 1. Scan for any leftover worker mounts
       // 2. Forcefully unmount them
       // 3. Remove empty worker directories
       
       fmt.Println("Cleanup complete")
   }
   ```

#### Testing Checklist

- [ ] `dsynth status` shows database info
- [ ] `dsynth status editors/vim` shows port status
- [ ] `dsynth reset-db` removes database
- [ ] `-y` bypasses reset-db confirmation
- [ ] `dsynth cleanup` removes stale mounts
- [ ] Commands work with missing database

---

### Task 4: Add UUID Tracking to Logs ⚪

**Estimated Time**: 1.5 hours  
**Priority**: Medium  
**Dependencies**: Task 2

#### Description

Enhance logging to include build UUIDs, making it easy to correlate log entries with 
database records.

#### Implementation Steps

1. **Update `log.Logger` to support context**:
   ```go
   // In log/log.go
   
   type Context struct {
       BuildID  string
       PortDir  string
       WorkerID int
   }
   
   func (l *Logger) WithContext(ctx Context) *ContextLogger {
       return &ContextLogger{
           logger: l,
           ctx:    ctx,
       }
   }
   
   type ContextLogger struct {
       logger *Logger
       ctx    Context
   }
   
   func (cl *ContextLogger) Info(format string, args ...interface{}) {
       prefix := fmt.Sprintf("[%s] [W%d] %s: ",
           cl.ctx.BuildID[:8], // Short UUID
           cl.ctx.WorkerID,
           cl.ctx.PortDir)
       cl.logger.Info(prefix+format, args...)
   }
   
   // Similarly for Debug, Warn, Error
   ```

2. **Update `build.go` to use context logging**:
   ```go
   // In worker goroutine
   ctxLogger := logger.WithContext(log.Context{
       BuildID:  buildUUID,
       PortDir:  p.PortDir,
       WorkerID: worker.ID,
   })
   
   ctxLogger.Info("Starting build")
   // ... build phases ...
   ctxLogger.Info("Build completed successfully")
   ```

3. **Add UUID to log file names**:
   ```go
   // In log.NewLogger
   logFileName := fmt.Sprintf("build_%s_%s.log",
       time.Now().Format("20060102_150405"),
       buildUUID[:8])
   ```

#### Testing Checklist

- [ ] Log entries include short UUID prefix
- [ ] Worker ID visible in logs
- [ ] Port directory in each log line
- [ ] Log file names include UUID
- [ ] Context logging doesn't affect performance
- [ ] Legacy logging still works

---

### Task 5: Update Configuration ⚪

**Estimated Time**: 1 hour  
**Priority**: Low  
**Dependencies**: None

#### Description

Add Phase 7-related configuration options and ensure backward compatibility.

#### Implementation Steps

1. **Add to `config/config.go`**:
   ```go
   type Config struct {
       // ... existing fields ...
       
       Migration struct {
           AutoMigrate bool `json:"auto_migrate"` // Default: true
           BackupLegacy bool `json:"backup_legacy"` // Default: true
       } `json:"migration"`
       
       Database struct {
           Path string `json:"path"` // Default: "${BuildBase}/builds.db"
           AutoVacuum bool `json:"auto_vacuum"` // Default: true
       } `json:"database"`
   }
   ```

2. **Add defaults**:
   ```go
   func ApplyDefaults(cfg *Config) {
       // ... existing defaults ...
       
       if cfg.Migration.AutoMigrate {
           cfg.Migration.AutoMigrate = true
       }
       if cfg.Migration.BackupLegacy {
           cfg.Migration.BackupLegacy = true
       }
       
       if cfg.Database.Path == "" {
           cfg.Database.Path = filepath.Join(cfg.System.BuildBase, "builds.db")
       }
       cfg.Database.AutoVacuum = true
   }
   ```

3. **Update example config** in `config.example.json`:
   ```json
   {
     "profiles": {
       "default": {
         "num_workers": 4,
         "packages_dir": "/usr/dports",
         "distfiles_dir": "/usr/distfiles",
         "build_base": "/build"
       }
     },
     "migration": {
       "auto_migrate": true,
       "backup_legacy": true
     },
     "database": {
       "path": "/build/builds.db",
       "auto_vacuum": true
     }
   }
   ```

#### Testing Checklist

- [ ] New config options parse correctly
- [ ] Defaults applied when missing
- [ ] Backward compatibility maintained
- [ ] Example config is valid
- [ ] Migration config respected

---

### Task 6: Create Initialization Command ⚪

**Estimated Time**: 1 hour  
**Priority**: Medium  
**Dependencies**: Task 5

#### Description

Enhance `dsynth init` to set up the new BuildDB and perform initial migration.

#### Implementation Steps

1. **Update `doInit()`**:
   ```go
   func doInit(cfg *config.Config) {
       fmt.Println("Initializing dsynth environment...")
       
       // 1. Create build base directory
       buildBase := cfg.System.BuildBase
       if err := os.MkdirAll(buildBase, 0755); err != nil {
           fmt.Fprintf(os.Stderr, "Failed to create build base: %v\n", err)
           os.Exit(1)
       }
       fmt.Printf("✓ Created build base: %s\n", buildBase)
       
       // 2. Create log directory
       logDir := cfg.System.LogDir
       if logDir == "" {
           logDir = filepath.Join(buildBase, "logs")
       }
       if err := os.MkdirAll(logDir, 0755); err != nil {
           fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
           os.Exit(1)
       }
       fmt.Printf("✓ Created log directory: %s\n", logDir)
       
       // 3. Initialize BuildDB
       dbPath := filepath.Join(buildBase, "builds.db")
       db, err := builddb.OpenDB(dbPath)
       if err != nil {
           fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
           os.Exit(1)
       }
       db.Close()
       fmt.Printf("✓ Initialized build database: %s\n", dbPath)
       
       // 4. Check for legacy data
       if migration.DetectMigrationNeeded(cfg) {
           fmt.Println("\nLegacy CRC data detected.")
           if cfg.YesAll {
               fmt.Println("Migrating automatically...")
               db, _ := builddb.OpenDB(dbPath)
               migration.MigrateLegacyCRC(cfg, db)
               db.Close()
           } else {
               fmt.Print("Migrate now? [Y/n]: ")
               var response string
               fmt.Scanln(&response)
               if response == "" || strings.EqualFold(response, "y") {
                   db, _ := builddb.OpenDB(dbPath)
                   migration.MigrateLegacyCRC(cfg, db)
                   db.Close()
               }
           }
       }
       
       // 5. Verify packages directory
       if _, err := os.Stat(cfg.System.PackagesDir); os.IsNotExist(err) {
           fmt.Printf("⚠ Packages directory not found: %s\n", cfg.System.PackagesDir)
           fmt.Println("  Please create it or update config.json")
       } else {
           fmt.Printf("✓ Packages directory: %s\n", cfg.System.PackagesDir)
       }
       
       fmt.Println("\n✓ Initialization complete")
       fmt.Println("\nNext steps:")
       fmt.Println("  1. Verify config.json settings")
       fmt.Println("  2. Run: dsynth build <package>")
   }
   ```

#### Testing Checklist

- [ ] `dsynth init` creates build base
- [ ] Database initialized correctly
- [ ] Log directory created
- [ ] Legacy data migrated if present
- [ ] Helpful next steps displayed
- [ ] Idempotent (safe to run multiple times)

---

### Task 7: End-to-End Integration Tests ⚪

**Estimated Time**: 2 hours  
**Priority**: High  
**Dependencies**: Tasks 1-6

#### Description

Create comprehensive end-to-end tests that validate the entire pipeline from CLI to 
database to build completion.

#### Implementation Steps

1. **Create `integration_e2e_test.go`**:
   ```go
   //go:build integration
   // +build integration
   
   package main_test
   
   import (
       "os"
       "os/exec"
       "path/filepath"
       "testing"
   )
   
   func TestE2EBuildFlow(t *testing.T) {
       if os.Getuid() != 0 {
           t.Skip("requires root for mount operations")
       }
       
       tmpDir := t.TempDir()
       
       // 1. Create minimal config
       configPath := filepath.Join(tmpDir, "config.json")
       configData := `{
           "profiles": {
               "default": {
                   "num_workers": 1,
                   "packages_dir": "/usr/dports",
                   "build_base": "` + tmpDir + `"
               }
           }
       }`
       os.WriteFile(configPath, []byte(configData), 0644)
       
       // 2. Run init
       cmd := exec.Command("./dsynth", "-C", tmpDir, "-y", "init")
       if output, err := cmd.CombinedOutput(); err != nil {
           t.Fatalf("init failed: %v\n%s", err, output)
       }
       
       // 3. Build a small package (twice to test CRC skip)
       for i := 0; i < 2; i++ {
           cmd = exec.Command("./dsynth", "-C", tmpDir, "-y", "build", "misc/hello")
           output, err := cmd.CombinedOutput()
           if err != nil {
               t.Fatalf("build %d failed: %v\n%s", i+1, err, output)
           }
           
           // On second build, verify CRC skip
           if i == 1 && !bytes.Contains(output, []byte("Skipped: 1")) {
               t.Error("Expected CRC skip on second build")
           }
       }
       
       // 4. Verify database state
       dbPath := filepath.Join(tmpDir, "builds.db")
       db, _ := builddb.OpenDB(dbPath)
       defer db.Close()
       
       rec, err := db.LatestFor("misc/hello", "")
       if err != nil {
           t.Fatalf("Failed to query database: %v", err)
       }
       
       if rec.Status != "success" {
           t.Errorf("Expected status=success, got %s", rec.Status)
       }
   }
   
   func TestE2EMigration(t *testing.T) {
       // Create legacy CRC file
       // Run dsynth init
       // Verify migration occurred
       // Run build
       // Verify CRC skip works with migrated data
   }
   
   func TestE2EForceRebuild(t *testing.T) {
       // Build once
       // Force rebuild
       // Verify rebuilt despite CRC match
   }
   ```

#### Testing Checklist

- [ ] E2E test builds package successfully
- [ ] Second build skips via CRC
- [ ] Database records created correctly
- [ ] Migration test validates legacy import
- [ ] Force rebuild bypasses CRC
- [ ] Tests run in CI (if root available)

---

### Task 8: Update Documentation ⚪

**Estimated Time**: 1.5 hours  
**Priority**: Medium  
**Dependencies**: Tasks 1-7

#### Description

Update user-facing documentation to reflect new functionality and migration process.

#### Implementation Steps

1. **Update `README.md`**:
   ````markdown
   # go-synth
   
   Modern port building system for DragonFly BSD, rewritten in Go.
   
   ## Features
   
   - **Incremental Builds**: CRC-based change detection skips unchanged ports
   - **Parallel Building**: Multi-worker builds with dependency ordering
   - **Build Tracking**: SQLite database tracks all build attempts
   - **Clean Architecture**: Modular design with pkg, builddb, build packages
   
   ## Quick Start
   
   ```bash
   # Initialize environment
   dsynth init
   
   # Build a package
   dsynth build editors/vim
   
   # Force rebuild (bypass CRC check)
   dsynth force editors/vim
   
   # Check status
   dsynth status editors/vim
   ```
   
   ## Migration from Legacy dsynth
   
   go-synth automatically detects and migrates legacy CRC data on first run:
   
   ```bash
   # Your existing CRC data will be preserved
   dsynth init
   ```
   
   The legacy `crc_index` file is backed up to `crc_index.bak`.
   
   ## Configuration
   
   Default location: `~/.config/dsynth/config.json`
   
   See [docs/configuration.md](docs/configuration.md) for details.
   ````

2. **Create `docs/MIGRATION.md`**:
   ```markdown
   # Migration Guide
   
   ## Migrating from Legacy dsynth
   
   go-synth is designed for seamless migration from C-based dsynth.
   
   ### Automatic Migration
   
   Run `dsynth init` and migration happens automatically:
   
   ```bash
   $ dsynth init
   Initializing dsynth environment...
   ✓ Created build base: /build
   ✓ Initialized build database: /build/builds.db
   
   Legacy CRC data detected.
   Migrating 142 CRC records...
   Successfully migrated 142/142 records
   Legacy file backed up to: /build/crc_index.bak
   ```
   
   ### What Gets Migrated
   
   - **CRC Index**: Port checksums for incremental builds
   - **Build History**: NOT migrated (fresh start)
   
   ### Backward Compatibility
   
   - Configuration files remain compatible
   - Command-line flags unchanged
   - Log format mostly preserved (UUIDs added)
   
   ### Manual Migration
   
   If automatic migration fails:
   
   ```bash
   # Backup your data
   cp /build/crc_index /tmp/crc_index.bak
   
   # Reset and reinitialize
   dsynth reset-db
   dsynth init
   ```
   
   ### Rollback
   
   To revert to legacy dsynth:
   
   ```bash
   # Restore legacy CRC file
   mv /build/crc_index.bak /build/crc_index
   
   # Remove go-synth database
   rm /build/builds.db
   
   # Use legacy dsynth binary
   /usr/local/bin/dsynth-legacy build editors/vim
   ```
   ```

3. **Update `DEVELOPMENT.md` Phase 7 section** (see Task 9)

#### Testing Checklist

- [ ] README quick start works
- [ ] Migration guide is clear
- [ ] Examples are correct
- [ ] Links work
- [ ] Configuration docs updated

---

### Task 9: Update DEVELOPMENT.md ⚪

**Estimated Time**: 0.5 hours  
**Priority**: Low  
**Dependencies**: Tasks 1-8

#### Description

Mark Phase 7 as complete and update project status.

#### Implementation Steps

1. **Update Phase 7 section**:
   ```markdown
   ## Phase 7: Integration & Migration ✅
   
   **Status**: ✅ Complete  
   **Timeline**: Started YYYY-MM-DD | Completed YYYY-MM-DD  
   **Dependencies**: Phases 1-6 complete
   
   ### Task Breakdown
   
   - [x] 1. Create migration package (2h)
   - [x] 2. Wire CLI build commands (3h)
   - [x] 3. Wire other CLI commands (2h)
   - [x] 4. Add UUID tracking to logs (1.5h)
   - [x] 5. Update configuration (1h)
   - [x] 6. Create initialization command (1h)
   - [x] 7. End-to-end integration tests (2h)
   - [x] 8. Update documentation (1.5h)
   - [x] 9. Update DEVELOPMENT.md (0.5h)
   
   **Total**: 12/12 hours (100%)
   
   ### Exit Criteria
   
   - [x] End-to-end build via CLI works
   - [x] CRC skip validated across two runs
   - [x] Migration from file-based CRC completes
   - [x] All CLI commands functional
   - [x] UUID tracking in logs
   - [x] Documentation complete
   ```

2. **Update project status**:
   ```markdown
   ## Project Status
   
   ### Completed Phases ✅
   
   - **Phase 1**: Library (pkg) - 100%
   - **Phase 2**: BuildDB - 100%
   - **Phase 3**: Builder - 100%
   - **Phase 4**: Environment - 100%
   - **Phase 5**: REST API - 100% (Optional)
   - **Phase 6**: Testing - 100%
   - **Phase 7**: Integration - 100%
   
   ### MVP Complete 🎉
   
   go-synth MVP is **feature complete** and ready for production use.
   
   Next steps:
   - Performance tuning
   - Ncurses UI (Phase 1.5)
   - Additional backends (jails, containers)
   ```

#### Testing Checklist

- [ ] DEVELOPMENT.md updated
- [ ] Phase 7 marked complete
- [ ] Project status reflects completion
- [ ] Next steps identified

---

## Summary

### Estimated Time Breakdown

| Task | Estimated | Critical Path |
|------|-----------|---------------|
| 1. Migration Package | 2h | ✅ |
| 2. Wire CLI Build | 3h | ✅ |
| 3. Wire Other Commands | 2h | ✅ |
| 4. UUID Logging | 1.5h | |
| 5. Update Config | 1h | |
| 6. Init Command | 1h | ✅ |
| 7. E2E Tests | 2h | ✅ |
| 8. Documentation | 1.5h | |
| 9. DEVELOPMENT.md | 0.5h | |
| **Total** | **14.5h** | **10.5h critical** |

### Exit Criteria

- [ ] End-to-end build via CLI works correctly
- [ ] CRC skip validated across two consecutive runs
- [ ] Migration from file-based CRC completes successfully
- [ ] All existing CLI commands remain functional
- [ ] UUID tracking visible in log files
- [ ] `dsynth init` sets up new environment
- [ ] Documentation complete and accurate
- [ ] E2E tests pass

### Dependencies

**Requires**:
- ✅ Phase 1: pkg package (parsing, resolution, ordering)
- ✅ Phase 2: builddb package (CRC tracking, build records)
- ✅ Phase 3: build package (DoBuild, worker management)
- 🔄 Phase 4: environment package (Setup, Execute, Cleanup)
- ✅ Phase 6: Testing (ensures quality)

**Blocks**:
- Nothing (final phase, completes MVP)

### Code Impact

| Package | New Lines | Changes |
|---------|-----------|---------|
| `migration/` (new) | ~300 | Create package |
| `main.go` | +200 | Wire commands |
| `log/` | +100 | Add context logging |
| `config/` | +30 | Migration/DB config |
| `docs/` | +500 | User documentation |
| **Total** | **~1,130** | **CLI integration** |

---

## Notes

### Design Decisions

1. **Automatic Migration**: Detect and migrate legacy data automatically (user-friendly)
2. **Backup Legacy Data**: Always backup before migration (safety first)
3. **Graceful Degradation**: Commands work without database if possible
4. **Minimal Breaking Changes**: Preserve existing CLI interface
5. **UUID in Logs**: Short UUID (8 chars) for readability

### Command Mapping

| Command | Legacy Behavior | go-synth Behavior |
|---------|----------------|-------------------|
| `build` | Build with deps | Same + BuildDB tracking |
| `just-build` | Build without deps | Same (alias for build) |
| `force` | Force rebuild | Same + bypass CRC |
| `status` | Show build status | Query BuildDB |
| `init` | Setup environment | Setup + migration |
| `reset-db` | Remove CRC file | Remove BuildDB |
| `cleanup` | Unmount workers | Same + cleanup |

### Migration Strategy

```
First Run:
  1. dsynth init
  2. Detect /build/crc_index
  3. Prompt for migration (or auto with -y)
  4. Import CRCs into BuildDB
  5. Backup legacy file
  6. Ready to build

Second Run:
  1. dsynth build editors/vim
  2. Parse + resolve
  3. Check CRC in BuildDB
  4. Skip if match, build if changed
  5. Update CRC on success
```

### Initialization Sequence

```go
// In doBuild()
1. Load config from ~/.config/dsynth/config.json
2. Open logger (/build/logs/build_*.log)
3. Open BuildDB (/build/builds.db)
4. Detect migration needed (check /build/crc_index)
5. Auto-migrate if present
6. Parse ports (pkg.ParsePortList)
7. Resolve dependencies (pkg.ResolveDependencies)
8. Display build plan
9. Prompt confirmation (unless -y)
10. Execute build (build.DoBuild)
11. Display statistics
12. Exit with appropriate code
```

### Testing Strategy

- **Unit Tests**: Migration package, config updates
- **Integration Tests**: CLI command execution, database interaction
- **E2E Tests**: Full pipeline from `dsynth build` to package built
- **Manual Tests**: Migration from legacy installation

### Future Enhancements (Post-MVP)

- Web UI for build monitoring
- Distributed builds across multiple machines
- Build queue management
- Notification system (email, webhooks)
- Build artifact caching
- Automatic dependency updates
- Build performance profiling

---

**Project Complete**: This is the final phase. After Phase 7, go-synth MVP is **complete** 
and ready for production use! 🎉

// Package service provides reusable business logic for dsynth operations.
//
// The service layer sits between the CLI (main.go) and library packages (pkg, build, builddb, etc.),
// providing a clean separation of concerns:
//
//   - CLI layer (main.go): handles user interaction, prompts, formatting, arg parsing
//   - Service layer (service/): orchestrates business logic, coordinates between libraries
//   - Library layer (pkg, build, etc.): provides core functionality with no I/O coupling
//
// This design enables the service layer to be reused in different contexts:
//   - CLI tool (current usage)
//   - REST API (Phase 5)
//   - GUI applications
//   - Test harnesses
//
// All service methods use the LibraryLogger interface for output, ensuring they can be
// used in any context without terminal coupling.
package service

import (
	"fmt"
	"sync"

	"dsynth/builddb"
	"dsynth/config"
	"dsynth/log"
)

// Service coordinates business logic across dsynth subsystems.
//
// It manages lifecycle of shared resources (logger, database) and provides
// high-level operations for build orchestration, status queries, and maintenance.
//
// Usage:
//
//	cfg, _ := config.LoadConfig("", "default")
//	svc, err := service.NewService(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer svc.Close()
//
//	result, err := svc.Build(service.BuildOptions{
//	    PortList: []string{"editors/vim"},
//	    Force:    false,
//	})
type Service struct {
	cfg           *config.Config
	logger        *log.Logger
	db            *builddb.DB
	activeCleanup func() // Cleanup function for active build (set immediately when workers created)
	cleanupMu     sync.Mutex
}

// NewService creates a new Service instance with the given configuration.
//
// It initializes the logger and opens the build database. The caller is responsible
// for calling Close() to release resources (typically via defer).
//
// Returns an error if logger initialization or database opening fails.
func NewService(cfg *config.Config) (*Service, error) {
	// Initialize logger
	logger, err := log.NewLogger(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Open build database
	db, err := builddb.OpenDB(cfg.Database.Path)
	if err != nil {
		logger.Close()
		return nil, fmt.Errorf("failed to open build database: %w", err)
	}

	return &Service{
		cfg:    cfg,
		logger: logger,
		db:     db,
	}, nil
}

// Close releases resources held by the service (logger, database).
//
// This method should be called when the service is no longer needed,
// typically via defer immediately after NewService:
//
//	svc, err := service.NewService(cfg)
//	if err != nil { ... }
//	defer svc.Close()
//
// Note: This does NOT call cleanup for active builds. The caller is responsible
// for calling the cleanup function returned in BuildResult.
func (s *Service) Close() error {
	var errs []error

	// Close database and logger
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("database close: %w", err))
		}
	}

	if s.logger != nil {
		s.logger.Close()
	}

	if len(errs) > 0 {
		return fmt.Errorf("service close errors: %v", errs)
	}

	return nil
}

// Config returns the service's configuration.
func (s *Service) Config() *config.Config {
	return s.cfg
}

// Logger returns the service's logger.
func (s *Service) Logger() *log.Logger {
	return s.logger
}

// Database returns the service's build database.
func (s *Service) Database() *builddb.DB {
	return s.db
}

// SetActiveCleanup stores the cleanup function for the active build.
// This is called by Build() as soon as workers are created, allowing
// signal handlers to access the cleanup function immediately.
func (s *Service) SetActiveCleanup(cleanup func()) {
	s.cleanupMu.Lock()
	s.activeCleanup = cleanup
	s.cleanupMu.Unlock()
}

// GetActiveCleanup returns the cleanup function for the active build.
// Returns nil if no build is active.
// This is called by signal handlers to cleanup workers on interruption.
func (s *Service) GetActiveCleanup() func() {
	s.cleanupMu.Lock()
	defer s.cleanupMu.Unlock()
	return s.activeCleanup
}

// ClearActiveCleanup removes the stored cleanup function.
// This is called after cleanup completes.
func (s *Service) ClearActiveCleanup() {
	s.cleanupMu.Lock()
	s.activeCleanup = nil
	s.cleanupMu.Unlock()
}

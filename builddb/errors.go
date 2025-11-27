// Package builddb defines structured error types for build database operations.
//
// This file provides comprehensive error handling with two categories:
//
//  1. Sentinel Errors: Simple error constants for common conditions (use errors.Is)
//  2. Structured Errors: Rich error types with context (use errors.As)
//
// All structured error types implement Unwrap() for compatibility with Go's
// error wrapping and inspection functions (errors.Is, errors.As, errors.Unwrap).
package builddb

import (
	"errors"
	"fmt"
)

// ==================== Sentinel Errors ====================
// These are simple error constants that can be checked with errors.Is()

var (
	// ErrDatabaseNotOpen is returned when attempting operations on a closed database
	ErrDatabaseNotOpen = fmt.Errorf("database not open")

	// ErrDatabaseClosed is returned when attempting to close an already closed database
	ErrDatabaseClosed = fmt.Errorf("database already closed")

	// ErrEmptyUUID is returned when a UUID parameter is empty or missing
	ErrEmptyUUID = fmt.Errorf("UUID cannot be empty")

	// ErrInvalidUUID is returned when a UUID format is invalid
	ErrInvalidUUID = fmt.Errorf("invalid UUID format")

	// ErrEmptyPortDir is returned when a port directory parameter is empty
	ErrEmptyPortDir = fmt.Errorf("port directory cannot be empty")

	// ErrRecordNotFound is returned when a build record doesn't exist in the database
	ErrRecordNotFound = fmt.Errorf("build record not found")

	// ErrBucketNotFound is returned when a required database bucket doesn't exist
	ErrBucketNotFound = fmt.Errorf("database bucket not found")

	// ErrCorruptedData is returned when database data cannot be parsed or is invalid
	ErrCorruptedData = fmt.Errorf("corrupted database data")

	// ErrOrphanedRecord is returned when a record references non-existent data
	ErrOrphanedRecord = fmt.Errorf("orphaned record reference")
)

// ==================== Structured Error Types ====================

// DatabaseError wraps database operation errors with context about the operation
// and bucket involved. This provides detailed information for debugging database
// issues while maintaining error chain compatibility.
//
// Use errors.As to extract DatabaseError from error chains:
//
//	var dbErr *DatabaseError
//	if errors.As(err, &dbErr) {
//	    log.Printf("Database operation '%s' failed on bucket '%s'", dbErr.Op, dbErr.Bucket)
//	}
type DatabaseError struct {
	// Op is the operation that failed (e.g., "open", "create bucket", "transaction")
	Op string

	// Bucket is the bucket name involved in the operation (empty if not applicable)
	Bucket string

	// Err is the underlying error that caused the failure
	Err error
}

// Error implements the error interface
func (e *DatabaseError) Error() string {
	if e.Bucket != "" {
		return fmt.Sprintf("database %s [bucket: %s]: %v", e.Op, e.Bucket, e.Err)
	}
	return fmt.Sprintf("database %s: %v", e.Op, e.Err)
}

// Unwrap allows errors.Is() and errors.As() to work with wrapped errors
func (e *DatabaseError) Unwrap() error {
	return e.Err
}

// RecordError wraps build record operation errors with context about which
// record was involved and what operation failed.
//
// Use errors.As to extract RecordError from error chains:
//
//	var recErr *RecordError
//	if errors.As(err, &recErr) {
//	    log.Printf("Record operation '%s' failed for UUID '%s'", recErr.Op, recErr.UUID)
//	}
type RecordError struct {
	// Op is the operation that failed (e.g., "save", "get", "update", "delete")
	Op string

	// UUID is the build record UUID involved in the operation
	UUID string

	// Err is the underlying error that caused the failure
	Err error
}

// Error implements the error interface
func (e *RecordError) Error() string {
	return fmt.Sprintf("build record %s [uuid: %s]: %v", e.Op, e.UUID, e.Err)
}

// Unwrap allows errors.Is() and errors.As() to work with wrapped errors
func (e *RecordError) Unwrap() error {
	return e.Err
}

// PackageIndexError wraps package index operation errors with context about
// which package and version were involved.
//
// Use errors.As to extract PackageIndexError from error chains:
//
//	var pkgErr *PackageIndexError
//	if errors.As(err, &pkgErr) {
//	    log.Printf("Package index '%s' failed for %s@%s", pkgErr.Op, pkgErr.PortDir, pkgErr.Version)
//	}
type PackageIndexError struct {
	// Op is the operation that failed (e.g., "update", "lookup", "validate")
	Op string

	// PortDir is the port directory (e.g., "editors/vim")
	PortDir string

	// Version is the port version (e.g., "default", "9.0.1")
	Version string

	// Err is the underlying error that caused the failure
	Err error
}

// Error implements the error interface
func (e *PackageIndexError) Error() string {
	return fmt.Sprintf("package index %s [%s@%s]: %v",
		e.Op, e.PortDir, e.Version, e.Err)
}

// Unwrap allows errors.Is() and errors.As() to work with wrapped errors
func (e *PackageIndexError) Unwrap() error {
	return e.Err
}

// CRCError wraps CRC operation errors with context about which port directory
// was involved and what operation failed.
//
// Use errors.As to extract CRCError from error chains:
//
//	var crcErr *CRCError
//	if errors.As(err, &crcErr) {
//	    log.Printf("CRC operation '%s' failed for port '%s'", crcErr.Op, crcErr.PortDir)
//	}
type CRCError struct {
	// Op is the operation that failed (e.g., "compute", "update", "get")
	Op string

	// PortDir is the port directory (e.g., "editors/vim")
	PortDir string

	// Err is the underlying error that caused the failure
	Err error
}

// Error implements the error interface
func (e *CRCError) Error() string {
	return fmt.Sprintf("CRC %s [%s]: %v", e.Op, e.PortDir, e.Err)
}

// Unwrap allows errors.Is() and errors.As() to work with wrapped errors
func (e *CRCError) Unwrap() error {
	return e.Err
}

// ValidationError wraps input validation errors with context about which
// field failed validation and what the invalid value was.
//
// Use errors.As to extract ValidationError from error chains:
//
//	var valErr *ValidationError
//	if errors.As(err, &valErr) {
//	    log.Printf("Validation failed for field '%s': %v", valErr.Field, valErr.Err)
//	}
type ValidationError struct {
	// Field is the name of the field that failed validation
	Field string

	// Value is the invalid value (truncated if too long for readability)
	Value string

	// Err is the underlying sentinel error (e.g., ErrEmptyUUID)
	Err error
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("validation failed [%s=%s]: %v", e.Field, e.Value, e.Err)
	}
	return fmt.Sprintf("validation failed [%s]: %v", e.Field, e.Err)
}

// Unwrap allows errors.Is() and errors.As() to work with wrapped errors
func (e *ValidationError) Unwrap() error {
	return e.Err
}

// ==================== Error Inspection Helpers ====================

// IsValidationError checks if the error (or any error in its chain) is a ValidationError.
// This is useful for distinguishing between user input errors and system errors.
//
// Returns true if err wraps a ValidationError, false otherwise.
//
// Example:
//
//	if IsValidationError(err) {
//	    fmt.Println("Invalid input provided")
//	}
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// IsDatabaseError checks if the error (or any error in its chain) is a DatabaseError.
// This helps identify infrastructure-level failures.
//
// Returns true if err wraps a DatabaseError, false otherwise.
//
// Example:
//
//	if IsDatabaseError(err) {
//	    log.Error("Database infrastructure failure")
//	}
func IsDatabaseError(err error) bool {
	var de *DatabaseError
	return errors.As(err, &de)
}

// IsRecordNotFound checks if the error (or any error in its chain) is ErrRecordNotFound.
// This is useful for implementing "create if not exists" patterns.
//
// Returns true if err is or wraps ErrRecordNotFound, false otherwise.
//
// Example:
//
//	rec, err := db.GetRecord(uuid)
//	if IsRecordNotFound(err) {
//	    // Create new record
//	}
func IsRecordNotFound(err error) bool {
	return errors.Is(err, ErrRecordNotFound)
}

// IsBucketNotFound checks if the error (or any error in its chain) is ErrBucketNotFound.
// This typically indicates database corruption or initialization issues.
//
// Returns true if err is or wraps ErrBucketNotFound, false otherwise.
//
// Example:
//
//	if IsBucketNotFound(err) {
//	    log.Fatal("Database not properly initialized")
//	}
func IsBucketNotFound(err error) bool {
	return errors.Is(err, ErrBucketNotFound)
}

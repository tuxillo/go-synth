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

// IsValidationError checks if the error is a validation error.
// This is useful for distinguishing between user input errors and system errors.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// IsDatabaseError checks if the error is a database operation error.
// This helps identify infrastructure-level failures.
func IsDatabaseError(err error) bool {
	var de *DatabaseError
	return errors.As(err, &de)
}

// IsRecordNotFound checks if the error indicates a build record was not found.
// This is useful for implementing "create if not exists" patterns.
func IsRecordNotFound(err error) bool {
	return errors.Is(err, ErrRecordNotFound)
}

// IsBucketNotFound checks if the error indicates a database bucket was not found.
// This typically indicates database corruption or initialization issues.
func IsBucketNotFound(err error) bool {
	return errors.Is(err, ErrBucketNotFound)
}

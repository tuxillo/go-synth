package builddb

import (
	"errors"
	"fmt"
	"testing"
)

// TestSentinelErrors verifies that sentinel errors are distinct
func TestSentinelErrors(t *testing.T) {
	sentinels := []error{
		ErrDatabaseNotOpen,
		ErrDatabaseClosed,
		ErrEmptyUUID,
		ErrInvalidUUID,
		ErrEmptyPortDir,
		ErrRecordNotFound,
		ErrBucketNotFound,
		ErrCorruptedData,
		ErrOrphanedRecord,
	}

	// Verify all sentinels are non-nil
	for i, err := range sentinels {
		if err == nil {
			t.Errorf("sentinel error %d is nil", i)
		}
	}

	// Verify sentinels are distinct (no duplicates)
	for i := 0; i < len(sentinels); i++ {
		for j := i + 1; j < len(sentinels); j++ {
			if sentinels[i] == sentinels[j] {
				t.Errorf("sentinel errors %d and %d are the same: %v", i, j, sentinels[i])
			}
		}
	}
}

// TestDatabaseError tests DatabaseError structure and methods
func TestDatabaseError(t *testing.T) {
	tests := []struct {
		name           string
		err            *DatabaseError
		wantError      string
		wantUnwrap     error
		containsBucket bool
	}{
		{
			name: "with bucket",
			err: &DatabaseError{
				Op:     "create bucket",
				Bucket: "builds",
				Err:    errors.New("file not found"),
			},
			wantError:      "database create bucket [bucket: builds]: file not found",
			wantUnwrap:     errors.New("file not found"),
			containsBucket: true,
		},
		{
			name: "without bucket",
			err: &DatabaseError{
				Op:  "open",
				Err: errors.New("permission denied"),
			},
			wantError:      "database open: permission denied",
			wantUnwrap:     errors.New("permission denied"),
			containsBucket: false,
		},
		{
			name: "with sentinel error",
			err: &DatabaseError{
				Op:     "get bucket",
				Bucket: "crc_index",
				Err:    ErrBucketNotFound,
			},
			wantError:      "database get bucket [bucket: crc_index]: database bucket not found",
			wantUnwrap:     ErrBucketNotFound,
			containsBucket: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Error() method
			if got := tt.err.Error(); got != tt.wantError {
				t.Errorf("Error() = %q, want %q", got, tt.wantError)
			}

			// Test Unwrap() method
			if got := tt.err.Unwrap(); got == nil && tt.wantUnwrap != nil {
				t.Errorf("Unwrap() = nil, want %v", tt.wantUnwrap)
			}

			// Test errors.Is() for sentinel errors
			if tt.wantUnwrap == ErrBucketNotFound {
				if !errors.Is(tt.err, ErrBucketNotFound) {
					t.Errorf("errors.Is() should match ErrBucketNotFound")
				}
			}
		})
	}
}

// TestRecordError tests RecordError structure and methods
func TestRecordError(t *testing.T) {
	testErr := errors.New("test error")
	err := &RecordError{
		Op:   "save",
		UUID: "test-uuid-123",
		Err:  testErr,
	}

	// Test Error() output format
	got := err.Error()
	want := "build record save [uuid: test-uuid-123]: test error"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	// Test Unwrap()
	if err.Unwrap() != testErr {
		t.Error("Unwrap() did not return the wrapped error")
	}

	// Test errors.Is() with sentinel
	err2 := &RecordError{
		Op:   "get",
		UUID: "uuid-456",
		Err:  ErrRecordNotFound,
	}
	if !errors.Is(err2, ErrRecordNotFound) {
		t.Error("errors.Is() should match ErrRecordNotFound")
	}
}

// TestPackageIndexError tests PackageIndexError structure and methods
func TestPackageIndexError(t *testing.T) {
	err := &PackageIndexError{
		Op:      "update",
		PortDir: "editors/vim",
		Version: "9.0.1",
		Err:     errors.New("disk full"),
	}

	// Test Error() output format
	got := err.Error()
	want := "package index update [editors/vim@9.0.1]: disk full"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	// Test Unwrap()
	if err.Unwrap() == nil {
		t.Error("Unwrap() returned nil")
	}

	// Test with orphaned record sentinel
	err2 := &PackageIndexError{
		Op:      "validate",
		PortDir: "devel/git",
		Version: "default",
		Err:     ErrOrphanedRecord,
	}
	if !errors.Is(err2, ErrOrphanedRecord) {
		t.Error("errors.Is() should match ErrOrphanedRecord")
	}
}

// TestCRCError tests CRCError structure and methods
func TestCRCError(t *testing.T) {
	err := &CRCError{
		Op:      "compute",
		PortDir: "shells/bash",
		Err:     errors.New("file not found"),
	}

	// Test Error() output format
	got := err.Error()
	want := "CRC compute [shells/bash]: file not found"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	// Test Unwrap()
	if err.Unwrap() == nil {
		t.Error("Unwrap() returned nil")
	}
}

// TestValidationError tests ValidationError structure and methods
func TestValidationError(t *testing.T) {
	tests := []struct {
		name      string
		err       *ValidationError
		wantError string
	}{
		{
			name: "with value",
			err: &ValidationError{
				Field: "uuid",
				Value: "invalid-uuid",
				Err:   ErrInvalidUUID,
			},
			wantError: "validation failed [uuid=invalid-uuid]: invalid UUID format",
		},
		{
			name: "without value",
			err: &ValidationError{
				Field: "uuid",
				Err:   ErrEmptyUUID,
			},
			wantError: "validation failed [uuid]: UUID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Error() output format
			if got := tt.err.Error(); got != tt.wantError {
				t.Errorf("Error() = %q, want %q", got, tt.wantError)
			}

			// Test Unwrap()
			if tt.err.Unwrap() == nil {
				t.Error("Unwrap() returned nil")
			}

			// Test errors.Is()
			if !errors.Is(tt.err, tt.err.Err) {
				t.Error("errors.Is() should match wrapped sentinel error")
			}
		})
	}
}

// TestIsValidationError tests the IsValidationError helper
func TestIsValidationError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "validation error",
			err:  &ValidationError{Field: "uuid", Err: ErrEmptyUUID},
			want: true,
		},
		{
			name: "database error",
			err:  &DatabaseError{Op: "open", Err: errors.New("fail")},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "generic error",
			err:  fmt.Errorf("some error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidationError(tt.err); got != tt.want {
				t.Errorf("IsValidationError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsDatabaseError tests the IsDatabaseError helper
func TestIsDatabaseError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "database error",
			err:  &DatabaseError{Op: "open", Err: errors.New("fail")},
			want: true,
		},
		{
			name: "validation error",
			err:  &ValidationError{Field: "uuid", Err: ErrEmptyUUID},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDatabaseError(tt.err); got != tt.want {
				t.Errorf("IsDatabaseError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsRecordNotFound tests the IsRecordNotFound helper
func TestIsRecordNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "wrapped record not found",
			err:  &RecordError{Op: "get", UUID: "123", Err: ErrRecordNotFound},
			want: true,
		},
		{
			name: "direct record not found",
			err:  ErrRecordNotFound,
			want: true,
		},
		{
			name: "different error",
			err:  ErrBucketNotFound,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRecordNotFound(tt.err); got != tt.want {
				t.Errorf("IsRecordNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsBucketNotFound tests the IsBucketNotFound helper
func TestIsBucketNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "wrapped bucket not found",
			err:  &DatabaseError{Op: "get bucket", Bucket: "builds", Err: ErrBucketNotFound},
			want: true,
		},
		{
			name: "direct bucket not found",
			err:  ErrBucketNotFound,
			want: true,
		},
		{
			name: "different error",
			err:  ErrRecordNotFound,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBucketNotFound(tt.err); got != tt.want {
				t.Errorf("IsBucketNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestErrorChaining tests that errors.Is and errors.As work through error chains
func TestErrorChaining(t *testing.T) {
	// Create a chain: CRCError -> ValidationError -> ErrCorruptedData
	innerErr := &ValidationError{
		Field: "crc",
		Value: "10 bytes",
		Err:   ErrCorruptedData,
	}
	outerErr := &CRCError{
		Op:      "get",
		PortDir: "editors/vim",
		Err:     innerErr,
	}

	// Test errors.Is() can find ErrCorruptedData through the chain
	if !errors.Is(outerErr, ErrCorruptedData) {
		t.Error("errors.Is() should find ErrCorruptedData through error chain")
	}

	// Test errors.As() can extract ValidationError from chain
	var ve *ValidationError
	if !errors.As(outerErr, &ve) {
		t.Error("errors.As() should extract ValidationError from chain")
	}
	if ve.Field != "crc" {
		t.Errorf("extracted ValidationError has wrong field: got %q, want %q", ve.Field, "crc")
	}

	// Test errors.As() can extract CRCError from chain
	var ce *CRCError
	if !errors.As(outerErr, &ce) {
		t.Error("errors.As() should extract CRCError from chain")
	}
	if ce.PortDir != "editors/vim" {
		t.Errorf("extracted CRCError has wrong portDir: got %q, want %q", ce.PortDir, "editors/vim")
	}
}

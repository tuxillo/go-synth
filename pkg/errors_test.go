package pkg

import (
	"errors"
	"testing"
)

func TestPortNotFoundError(t *testing.T) {
	err := &PortNotFoundError{
		PortSpec: "editors/vim",
		Path:     "/usr/ports/editors/vim",
	}

	// Test Error() message
	msg := err.Error()
	expected := "port not found: editors/vim (path: /usr/ports/editors/vim)"
	if msg != expected {
		t.Errorf("unexpected error message:\ngot:  %s\nwant: %s", msg, expected)
	}

	// Test Unwrap() and errors.Is()
	if !errors.Is(err, ErrPortNotFound) {
		t.Error("PortNotFoundError should unwrap to ErrPortNotFound")
	}

	// Test errors.As() can extract the typed error
	var pnfErr *PortNotFoundError
	if !errors.As(err, &pnfErr) {
		t.Error("errors.As should be able to extract *PortNotFoundError")
	}
	if pnfErr.PortSpec != "editors/vim" {
		t.Errorf("extracted error has wrong PortSpec: %s", pnfErr.PortSpec)
	}
}

func TestCycleError(t *testing.T) {
	err := &CycleError{
		TotalPackages:   10,
		OrderedPackages: 7,
	}

	// Test Error() message
	msg := err.Error()
	expected := "cycle detected: only 7 of 10 packages ordered"
	if msg != expected {
		t.Errorf("unexpected error message:\ngot:  %s\nwant: %s", msg, expected)
	}

	// Test Unwrap() and errors.Is()
	if !errors.Is(err, ErrCycleDetected) {
		t.Error("CycleError should unwrap to ErrCycleDetected")
	}

	// Test errors.As() can extract the typed error
	var cycleErr *CycleError
	if !errors.As(err, &cycleErr) {
		t.Error("errors.As should be able to extract *CycleError")
	}
	if cycleErr.TotalPackages != 10 {
		t.Errorf("extracted error has wrong TotalPackages: %d", cycleErr.TotalPackages)
	}
	if cycleErr.OrderedPackages != 7 {
		t.Errorf("extracted error has wrong OrderedPackages: %d", cycleErr.OrderedPackages)
	}
}

func TestCycleErrorWithPackages(t *testing.T) {
	pkgA := &Package{PortDir: "cat/A", Name: "A"}
	pkgB := &Package{PortDir: "cat/B", Name: "B"}

	err := &CycleError{
		TotalPackages:   2,
		OrderedPackages: 0,
		CyclePackages:   []*Package{pkgA, pkgB},
	}

	var cycleErr *CycleError
	if !errors.As(err, &cycleErr) {
		t.Fatal("errors.As should be able to extract *CycleError")
	}

	if len(cycleErr.CyclePackages) != 2 {
		t.Errorf("expected 2 cycle packages, got %d", len(cycleErr.CyclePackages))
	}
}

func TestSentinelErrors(t *testing.T) {
	// Test that sentinel errors can be compared with errors.Is()
	tests := []struct {
		name string
		err  error
	}{
		{"ErrCycleDetected", ErrCycleDetected},
		{"ErrInvalidSpec", ErrInvalidSpec},
		{"ErrPortNotFound", ErrPortNotFound},
		{"ErrNoValidPorts", ErrNoValidPorts},
		{"ErrEmptySpec", ErrEmptySpec},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s is nil", tt.name)
			}
			// Each sentinel should equal itself
			if !errors.Is(tt.err, tt.err) {
				t.Errorf("%s should equal itself with errors.Is()", tt.name)
			}
		})
	}
}

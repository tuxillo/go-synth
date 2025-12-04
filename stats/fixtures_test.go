//go:build dragonfly || freebsd

package stats

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

// TestFixturesExist verifies that BSD sysctl fixtures are present.
// These fixtures should be captured from a real BSD system using:
//
//	./scripts/capture-bsd-fixtures.sh
func TestFixturesExist(t *testing.T) {
	fixtureDir := "testdata/fixtures"

	fixtures := []struct {
		name     string
		required bool
	}{
		{"vm.loadavg.bin", true},
		{"vm.vmtotal.bin", true},
		{"vm.swap_info.bin", false}, // Optional - may not exist if no swap
		{"system_info.txt", true},
	}

	for _, f := range fixtures {
		path := filepath.Join(fixtureDir, f.name)
		info, err := os.Stat(path)

		if err != nil {
			if f.required {
				t.Errorf("Required fixture missing: %s", f.name)
				t.Logf("  Run: ./scripts/capture-bsd-fixtures.sh on BSD VM")
			} else {
				t.Logf("Optional fixture not found: %s (ok)", f.name)
			}
			continue
		}

		t.Logf("✓ Found %s (%d bytes)", f.name, info.Size())
	}
}

// TestParseLoadavgFixture tests parsing real vm.loadavg binary data.
func TestParseLoadavgFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/fixtures/vm.loadavg.bin")
	if err != nil {
		t.Skip("Fixture not available - run ./scripts/capture-bsd-fixtures.sh")
		return
	}

	if len(data) < int(unsafe.Sizeof(loadavg{})) {
		t.Fatalf("Fixture too small: got %d bytes, need %d", len(data), unsafe.Sizeof(loadavg{}))
	}

	var la loadavg
	err = binary.Read(newBytesReader(data), binary.LittleEndian, &la)
	if err != nil {
		t.Fatalf("Failed to parse fixture: %v", err)
	}

	// Validate reasonable values
	for i := 0; i < 3; i++ {
		load := float64(la.Load[i]) / fscale
		if load < 0 || load > 1000 {
			t.Errorf("Load[%d] out of range: %f (raw: %d)", i, load, la.Load[i])
		} else {
			t.Logf("Load[%d] = %.2f", i, load)
		}
	}

	if la.Scale != 2048 {
		t.Logf("Warning: Scale is %d, expected 2048", la.Scale)
	}
}

// TestParseVmtotalFixture tests parsing real vm.vmtotal binary data.
func TestParseVmtotalFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/fixtures/vm.vmtotal.bin")
	if err != nil {
		t.Skip("Fixture not available - run ./scripts/capture-bsd-fixtures.sh")
		return
	}

	if len(data) < int(unsafe.Sizeof(vmtotal{})) {
		t.Fatalf("Fixture too small: got %d bytes, need %d", len(data), unsafe.Sizeof(vmtotal{}))
	}

	var vmt vmtotal
	err = binary.Read(newBytesReader(data), binary.LittleEndian, &vmt)
	if err != nil {
		t.Fatalf("Failed to parse fixture: %v", err)
	}

	// Log all fields for inspection
	t.Logf("vmtotal fields:")
	t.Logf("  T_rq (runnable):          %d", vmt.T_rq)
	t.Logf("  T_dw (disk wait):         %d", vmt.T_dw)
	t.Logf("  T_pw (page wait):         %d", vmt.T_pw)
	t.Logf("  T_sl (sleeping):          %d", vmt.T_sl)
	t.Logf("  T_sw (swapped):           %d", vmt.T_sw)
	t.Logf("  T_vm (virtual pages):     %d", vmt.T_vm)
	t.Logf("  T_avm (active virt):      %d", vmt.T_avm)
	t.Logf("  T_rm (real memory):       %d", vmt.T_rm)
	t.Logf("  T_arm (active real):      %d", vmt.T_arm)
	t.Logf("  T_free (free pages):      %d", vmt.T_free)

	// Validate critical field for adjusted load
	if vmt.T_pw < 0 {
		t.Errorf("T_pw (page wait) negative: %d", vmt.T_pw)
	}

	// Demonstrate adjusted load calculation
	// (This would be combined with loadavg data in real usage)
	adjustedLoad := float64(vmt.T_pw)
	t.Logf("Adjusted load contribution: +%.2f", adjustedLoad)
}

// TestParseSwapInfoFixture tests parsing real vm.swap_info binary data.
func TestParseSwapInfoFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/fixtures/vm.swap_info.bin")
	if err != nil {
		t.Skip("Fixture not available - run ./scripts/capture-bsd-fixtures.sh")
		return
	}

	if len(data) == 0 {
		t.Log("No swap configured (empty fixture)")
		return
	}

	entrySize := int(unsafe.Sizeof(xswdev{}))
	if len(data)%entrySize != 0 {
		t.Fatalf("Fixture size (%d) not multiple of xswdev size (%d)", len(data), entrySize)
	}

	numEntries := len(data) / entrySize
	t.Logf("Swap devices: %d", numEntries)

	var totalBlks, usedBlks int32

	for i := 0; i < numEntries; i++ {
		offset := i * entrySize
		chunk := data[offset : offset+entrySize]

		var xs xswdev
		err := binary.Read(newBytesReader(chunk), binary.LittleEndian, &xs)
		if err != nil {
			t.Errorf("Failed to parse entry %d: %v", i, err)
			continue
		}

		t.Logf("Swap device %d:", i)
		t.Logf("  Version: %d", xs.Version)
		t.Logf("  Dev:     %d", xs.Dev)
		t.Logf("  Flags:   %d", xs.Flags)
		t.Logf("  Nblks:   %d", xs.Nblks)
		t.Logf("  Used:    %d", xs.Used)

		if xs.Nblks > 0 {
			pct := int((float64(xs.Used) / float64(xs.Nblks)) * 100.0)
			t.Logf("  Usage:   %d%%", pct)
		}

		totalBlks += xs.Nblks
		usedBlks += xs.Used
	}

	if totalBlks > 0 {
		totalPct := int((float64(usedBlks) / float64(totalBlks)) * 100.0)
		t.Logf("Total swap usage: %d%%", totalPct)
	}
}

// TestAdjustedLoadCalculation demonstrates the full adjusted load calculation
// using real fixture data (combining vm.loadavg + vm.vmtotal).
func TestAdjustedLoadCalculation(t *testing.T) {
	// Load vm.loadavg fixture
	loadData, err := os.ReadFile("testdata/fixtures/vm.loadavg.bin")
	if err != nil {
		t.Skip("Fixture not available")
		return
	}

	var la loadavg
	if err := binary.Read(newBytesReader(loadData), binary.LittleEndian, &la); err != nil {
		t.Fatalf("Failed to parse loadavg: %v", err)
	}

	load1min := float64(la.Load[0]) / fscale

	// Load vm.vmtotal fixture
	vmData, err := os.ReadFile("testdata/fixtures/vm.vmtotal.bin")
	if err != nil {
		t.Logf("No vmtotal fixture - using base load only: %.2f", load1min)
		return
	}

	var vmt vmtotal
	if err := binary.Read(newBytesReader(vmData), binary.LittleEndian, &vmt); err != nil {
		t.Logf("Failed to parse vmtotal - using base load only: %.2f", load1min)
		return
	}

	// Calculate adjusted load (same logic as getAdjustedLoad)
	adjustedLoad := load1min + float64(vmt.T_pw)

	t.Logf("Base load (1min):        %.2f", load1min)
	t.Logf("Page wait processes:     %d", vmt.T_pw)
	t.Logf("Adjusted load:           %.2f", adjustedLoad)

	if vmt.T_pw > 0 {
		t.Logf("ℹ Page-fault waits detected - load adjusted upward")
	}
}

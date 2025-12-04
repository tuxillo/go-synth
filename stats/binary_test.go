//go:build dragonfly || freebsd
// +build dragonfly freebsd

package stats

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestExportedFieldsBinaryRead verifies that our structs can be used with
// encoding/binary.Read() by having exported fields. This test would panic
// if the fields were unexported (lowercase).
func TestExportedFieldsBinaryRead(t *testing.T) {
	// Test loadavg struct
	t.Run("loadavg", func(t *testing.T) {
		// Create a buffer with some data
		buf := new(bytes.Buffer)

		// Write some uint32 values for Load array
		binary.Write(buf, binary.LittleEndian, uint32(2048)) // Load[0]
		binary.Write(buf, binary.LittleEndian, uint32(4096)) // Load[1]
		binary.Write(buf, binary.LittleEndian, uint32(6144)) // Load[2]
		binary.Write(buf, binary.LittleEndian, int32(2048))  // Scale

		// Try to read into struct - this will panic if fields are unexported
		var la loadavg
		err := binary.Read(buf, binary.LittleEndian, &la)
		if err != nil {
			t.Fatalf("binary.Read failed: %v (fields likely unexported)", err)
		}

		// Verify values were read correctly
		if la.Load[0] != 2048 {
			t.Errorf("Load[0]: expected 2048, got %d", la.Load[0])
		}
		if la.Scale != 2048 {
			t.Errorf("Scale: expected 2048, got %d", la.Scale)
		}

		t.Logf("✓ loadavg binary.Read succeeded with exported fields")
	})

	// Test vmtotal struct
	t.Run("vmtotal", func(t *testing.T) {
		buf := new(bytes.Buffer)

		// Write all fields
		binary.Write(buf, binary.LittleEndian, int16(10))    // T_rq
		binary.Write(buf, binary.LittleEndian, int16(5))     // T_dw
		binary.Write(buf, binary.LittleEndian, int16(3))     // T_pw
		binary.Write(buf, binary.LittleEndian, int16(20))    // T_sl
		binary.Write(buf, binary.LittleEndian, int16(0))     // T_sw
		binary.Write(buf, binary.LittleEndian, uint32(1000)) // T_vm
		binary.Write(buf, binary.LittleEndian, uint32(800))  // T_avm
		binary.Write(buf, binary.LittleEndian, uint32(500))  // T_rm
		binary.Write(buf, binary.LittleEndian, uint32(400))  // T_arm
		binary.Write(buf, binary.LittleEndian, uint32(100))  // T_vmshr
		binary.Write(buf, binary.LittleEndian, uint32(80))   // T_avmshr
		binary.Write(buf, binary.LittleEndian, uint32(50))   // T_rmshr
		binary.Write(buf, binary.LittleEndian, uint32(40))   // T_armshr
		binary.Write(buf, binary.LittleEndian, uint32(200))  // T_free

		// Try to read - will panic if fields are unexported
		var vmt vmtotal
		err := binary.Read(buf, binary.LittleEndian, &vmt)
		if err != nil {
			t.Fatalf("binary.Read failed: %v (fields likely unexported)", err)
		}

		// Verify critical field (t_pw for adjusted load)
		if vmt.T_pw != 3 {
			t.Errorf("T_pw: expected 3, got %d", vmt.T_pw)
		}

		t.Logf("✓ vmtotal binary.Read succeeded with exported fields")
	})

	// Test xswdev struct
	t.Run("xswdev", func(t *testing.T) {
		buf := new(bytes.Buffer)

		// Write all fields
		binary.Write(buf, binary.LittleEndian, uint32(1))   // Version
		binary.Write(buf, binary.LittleEndian, uint64(0))   // Dev
		binary.Write(buf, binary.LittleEndian, int32(0))    // Flags
		binary.Write(buf, binary.LittleEndian, int32(1000)) // Nblks
		binary.Write(buf, binary.LittleEndian, int32(250))  // Used

		// Try to read - will panic if fields are unexported
		var xs xswdev
		err := binary.Read(buf, binary.LittleEndian, &xs)
		if err != nil {
			t.Fatalf("binary.Read failed: %v (fields likely unexported)", err)
		}

		// Verify values
		if xs.Nblks != 1000 {
			t.Errorf("Nblks: expected 1000, got %d", xs.Nblks)
		}
		if xs.Used != 250 {
			t.Errorf("Used: expected 250, got %d", xs.Used)
		}

		// Calculate percentage (same logic as getSwapUsage)
		pct := int((float64(xs.Used) / float64(xs.Nblks)) * 100.0)
		if pct != 25 {
			t.Errorf("Percentage: expected 25, got %d", pct)
		}

		t.Logf("✓ xswdev binary.Read succeeded with exported fields")
	})
}

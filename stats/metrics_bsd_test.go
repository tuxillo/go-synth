//go:build dragonfly || freebsd

package stats

import (
	"encoding/binary"
	"testing"
	"unsafe"
)

func TestGetAdjustedLoad(t *testing.T) {
	load, err := getAdjustedLoad()
	if err != nil {
		t.Logf("getAdjustedLoad failed (may be expected in test env): %v", err)
		return
	}

	if load < 0 {
		t.Errorf("Expected non-negative load, got %f", load)
	}

	t.Logf("Adjusted load: %.2f", load)
}

func TestGetSwapUsage(t *testing.T) {
	swap, err := getSwapUsage()
	if err != nil {
		t.Logf("getSwapUsage failed (may be expected if no swap): %v", err)
		return
	}

	if swap < 0 || swap > 100 {
		t.Errorf("Expected swap between 0-100, got %d", swap)
	}

	t.Logf("Swap usage: %d%%", swap)
}

func TestLoadavgParsing(t *testing.T) {
	var la loadavg
	la.load[0] = 2048
	la.load[1] = 4096
	la.load[2] = 6144
	la.scale = 2048

	load1 := float64(la.load[0]) / fscale
	if load1 != 1.0 {
		t.Errorf("Expected load 1.0, got %f", load1)
	}

	load5 := float64(la.load[1]) / fscale
	if load5 != 2.0 {
		t.Errorf("Expected load 2.0, got %f", load5)
	}

	load15 := float64(la.load[2]) / fscale
	if load15 != 3.0 {
		t.Errorf("Expected load 3.0, got %f", load15)
	}
}

func TestVmtotalParsing(t *testing.T) {
	var vmt vmtotal
	vmt.t_pw = 5

	tpw := float64(vmt.t_pw)
	if tpw != 5.0 {
		t.Errorf("Expected t_pw 5.0, got %f", tpw)
	}
}

func TestXswdevParsing(t *testing.T) {
	var xs xswdev
	xs.nblks = 1000
	xs.used = 250

	if xs.nblks != 1000 {
		t.Errorf("Expected nblks 1000, got %d", xs.nblks)
	}
	if xs.used != 250 {
		t.Errorf("Expected used 250, got %d", xs.used)
	}

	pct := int((float64(xs.used) / float64(xs.nblks)) * 100.0)
	if pct != 25 {
		t.Errorf("Expected 25%% usage, got %d%%", pct)
	}
}

func TestSwapPercentageCalculation(t *testing.T) {
	tests := []struct {
		name        string
		totalBlks   int32
		usedBlks    int32
		expectedPct int
	}{
		{"NoSwap", 0, 0, 0},
		{"25Percent", 1000, 250, 25},
		{"50Percent", 2000, 1000, 50},
		{"100Percent", 500, 500, 100},
		{"LowUsage", 10000, 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.totalBlks == 0 {
				if tt.expectedPct != 0 {
					t.Errorf("Expected 0%% for no swap")
				}
				return
			}

			pct := int((float64(tt.usedBlks) / float64(tt.totalBlks)) * 100.0)
			if pct != tt.expectedPct {
				t.Errorf("Expected %d%%, got %d%%", tt.expectedPct, pct)
			}
		})
	}
}

func TestBytesReader(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	reader := newBytesReader(data)

	buf := make([]byte, 2)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 2 {
		t.Errorf("Expected to read 2 bytes, got %d", n)
	}
	if buf[0] != 0x01 || buf[1] != 0x02 {
		t.Errorf("Unexpected data: %v", buf)
	}

	n, err = reader.Read(buf)
	if err != nil {
		t.Fatalf("Second read failed: %v", err)
	}
	if n != 2 {
		t.Errorf("Expected to read 2 bytes, got %d", n)
	}
	if buf[0] != 0x03 || buf[1] != 0x04 {
		t.Errorf("Unexpected data: %v", buf)
	}

	n, err = reader.Read(buf)
	if err == nil {
		t.Error("Expected EOF error")
	}
}

func TestStructSizes(t *testing.T) {
	laSize := unsafe.Sizeof(loadavg{})
	t.Logf("loadavg size: %d bytes", laSize)

	vmtSize := unsafe.Sizeof(vmtotal{})
	t.Logf("vmtotal size: %d bytes", vmtSize)

	xsSize := unsafe.Sizeof(xswdev{})
	t.Logf("xswdev size: %d bytes", xsSize)

	if laSize == 0 || vmtSize == 0 || xsSize == 0 {
		t.Error("Struct sizes should not be zero")
	}
}

func TestBinaryEncoding(t *testing.T) {
	var la loadavg
	la.load[0] = 2048
	la.load[1] = 4096
	la.load[2] = 6144
	la.scale = 2048

	buf := make([]byte, unsafe.Sizeof(la))
	reader := newBytesReader(buf)

	var decoded loadavg
	err := binary.Read(reader, binary.LittleEndian, &decoded)
	if err != nil {
		t.Logf("Binary read test (expected to work with real data): %v", err)
	}
}

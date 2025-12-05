//go:build ignore
// +build ignore

// generate_fixtures creates synthetic BSD sysctl fixtures for testing.
// This allows testing BSD parsing logic on any platform without actual syscalls.
//
// Usage: go run scripts/generate_fixtures.go

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	fscale = 2048.0
)

// BSD struct definitions (must match stats/metrics_bsd.go)
type loadavg struct {
	Load  [3]uint32
	Scale int32
}

type vmtotal struct {
	T_rq     int16
	T_dw     int16
	T_pw     int16
	T_sl     int16
	T_sw     int16
	T_vm     uint32
	T_avm    uint32
	T_rm     uint32
	T_arm    uint32
	T_vmshr  uint32
	T_avmshr uint32
	T_rmshr  uint32
	T_armshr uint32
	T_free   uint32
}

type xswdev struct {
	Version uint32
	Dev     uint64
	Flags   int32
	Nblks   int32
	Used    int32
}

func main() {
	fixtureDir := "stats/testdata/fixtures"

	if err := os.MkdirAll(fixtureDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create fixture dir: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generating synthetic BSD sysctl fixtures...")
	fmt.Printf("Output directory: %s\n\n", fixtureDir)

	// Generate vm.loadavg fixture
	if err := generateLoadavgFixture(fixtureDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate loadavg: %v\n", err)
		os.Exit(1)
	}

	// Generate vm.vmtotal fixture
	if err := generateVmtotalFixture(fixtureDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate vmtotal: %v\n", err)
		os.Exit(1)
	}

	// Generate vm.swap_info fixture
	if err := generateSwapInfoFixture(fixtureDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate swap_info: %v\n", err)
		os.Exit(1)
	}

	// Generate system_info.txt
	if err := generateSystemInfo(fixtureDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate system_info: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✓ All fixtures generated successfully!")
	fmt.Println("\nYou can now run: go test ./stats/ -run Fixture")
}

func generateLoadavgFixture(dir string) error {
	// Realistic load averages: 3.24, 2.85, 2.12
	// These represent a moderately loaded system
	scale := float64(fscale)
	la := loadavg{
		Load: [3]uint32{
			uint32(3.24 * scale), // 1min:  6635
			uint32(2.85 * scale), // 5min:  5837
			uint32(2.12 * scale), // 15min: 4342
		},
		Scale: 2048,
	}

	binPath := filepath.Join(dir, "vm.loadavg.bin")
	txtPath := filepath.Join(dir, "vm.loadavg.txt")

	// Write binary
	f, err := os.Create(binPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := binary.Write(f, binary.LittleEndian, &la); err != nil {
		return err
	}

	fmt.Printf("✓ vm.loadavg.bin (%d bytes)\n", binary.Size(la))

	// Write text reference
	txt := fmt.Sprintf("vm.loadavg: { %.2f %.2f %.2f }\n",
		float64(la.Load[0])/fscale,
		float64(la.Load[1])/fscale,
		float64(la.Load[2])/fscale)

	if err := os.WriteFile(txtPath, []byte(txt), 0644); err != nil {
		return err
	}

	fmt.Printf("✓ vm.loadavg.txt (reference)\n")
	return nil
}

func generateVmtotalFixture(dir string) error {
	// Realistic vmtotal state:
	// - 4 runnable processes
	// - 1 in disk wait
	// - 2 in page wait (contributes to adjusted load)
	// - 50 sleeping
	// - 0 swapped out
	// - Memory stats for a 16GB system
	vmt := vmtotal{
		T_rq:     4,      // Runnable
		T_dw:     1,      // Disk wait
		T_pw:     2,      // Page wait (adjusted load)
		T_sl:     50,     // Sleeping
		T_sw:     0,      // Swapped
		T_vm:     400000, // Virtual pages
		T_avm:    320000, // Active virtual
		T_rm:     200000, // Real memory pages
		T_arm:    180000, // Active real
		T_vmshr:  50000,  // Shared virtual
		T_avmshr: 45000,  // Active shared virtual
		T_rmshr:  30000,  // Shared real
		T_armshr: 28000,  // Active shared real
		T_free:   20000,  // Free pages
	}

	binPath := filepath.Join(dir, "vm.vmtotal.bin")
	txtPath := filepath.Join(dir, "vm.vmtotal.txt")

	// Write binary
	f, err := os.Create(binPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := binary.Write(f, binary.LittleEndian, &vmt); err != nil {
		return err
	}

	fmt.Printf("✓ vm.vmtotal.bin (%d bytes)\n", binary.Size(vmt))

	// Write text reference
	txt := fmt.Sprintf(`vm.vmtotal:
  Processes:
    Runnable: %d
    Disk Wait: %d
    Page Wait: %d (adjusted load contribution)
    Sleeping: %d
    Swapped: %d
  Memory (pages):
    Virtual: %d
    Active Virtual: %d
    Real: %d
    Active Real: %d
    Free: %d
`,
		vmt.T_rq, vmt.T_dw, vmt.T_pw, vmt.T_sl, vmt.T_sw,
		vmt.T_vm, vmt.T_avm, vmt.T_rm, vmt.T_arm, vmt.T_free)

	if err := os.WriteFile(txtPath, []byte(txt), 0644); err != nil {
		return err
	}

	fmt.Printf("✓ vm.vmtotal.txt (reference)\n")
	return nil
}

func generateSwapInfoFixture(dir string) error {
	// Two swap devices with realistic usage
	devices := []xswdev{
		{
			Version: 1,
			Dev:     0x00000000, // /dev/da0s1b
			Flags:   0,
			Nblks:   2097152, // 8GB (in 4K blocks)
			Used:    524288,  // 2GB used (25%)
		},
		{
			Version: 1,
			Dev:     0x00000001, // /dev/da1s1b
			Flags:   0,
			Nblks:   2097152, // 8GB (in 4K blocks)
			Used:    1048576, // 4GB used (50%)
		},
	}

	binPath := filepath.Join(dir, "vm.swap_info.bin")
	txtPath := filepath.Join(dir, "vm.swap_info.txt")

	// Write binary
	f, err := os.Create(binPath)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, dev := range devices {
		if err := binary.Write(f, binary.LittleEndian, &dev); err != nil {
			f.Close()
			return err
		}
	}
	f.Close()

	totalSize := binary.Size(devices[0]) * len(devices)
	fmt.Printf("✓ vm.swap_info.bin (%d bytes, %d devices)\n", totalSize, len(devices))

	// Write text reference
	txt := "vm.swap_info:\n"
	totalBlks := int32(0)
	totalUsed := int32(0)

	for i, dev := range devices {
		pct := int(float64(dev.Used) / float64(dev.Nblks) * 100.0)
		txt += fmt.Sprintf("  Device %d:\n", i)
		txt += fmt.Sprintf("    Blocks: %d (%.1f GB)\n", dev.Nblks, float64(dev.Nblks)*4096/1024/1024/1024)
		txt += fmt.Sprintf("    Used:   %d (%.1f GB)\n", dev.Used, float64(dev.Used)*4096/1024/1024/1024)
		txt += fmt.Sprintf("    Usage:  %d%%\n", pct)
		totalBlks += dev.Nblks
		totalUsed += dev.Used
	}

	totalPct := int(float64(totalUsed) / float64(totalBlks) * 100.0)
	txt += fmt.Sprintf("  Total swap usage: %d%%\n", totalPct)

	if err := os.WriteFile(txtPath, []byte(txt), 0644); err != nil {
		return err
	}

	fmt.Printf("✓ vm.swap_info.txt (reference)\n")
	return nil
}

func generateSystemInfo(dir string) error {
	path := filepath.Join(dir, "system_info.txt")

	txt := fmt.Sprintf(`# System Information
# Generated: %s
# Source: Synthetic fixtures for testing

System: DragonFly BSD (synthetic test data)
Kernel: DragonFly v6.4.0-RELEASE
CPU: 8 cores

Load Averages: 3.24, 2.85, 2.12
Adjusted Load: 3.24 + 2 (page wait) = 5.24

Swap Configuration:
  Device 0: 8GB, 25%% used
  Device 1: 8GB, 50%% used
  Total: 16GB, 37.5%% used

Memory: 16GB RAM

Notes:
- These are synthetic fixtures created by scripts/generate_fixtures.go
- Values represent a moderately loaded system with some page-fault waits
- Swap usage is intentionally varied across devices for testing
- Use ./scripts/capture-bsd-fixtures.sh to capture real VM data
`, time.Now().Format(time.RFC3339))

	if err := os.WriteFile(path, []byte(txt), 0644); err != nil {
		return err
	}

	fmt.Printf("✓ system_info.txt (metadata)\n")
	return nil
}

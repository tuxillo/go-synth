# BSD Sysctl Test Fixtures

This directory contains captured binary sysctl output from real BSD systems, used for testing the stats package parsing logic on any platform.

## Fixtures

- `vm.loadavg.bin` - Raw binary output from `sysctl -b vm.loadavg`
- `vm.vmtotal.bin` - Raw binary output from `sysctl -b vm.vmtotal`
- `vm.swap_info.bin` - Raw binary output from `sysctl -b vm.swap_info`
- `*.txt` - Human-readable reference versions

## Capturing Fixtures

To update fixtures with current VM data:

```bash
# 1. Sync project to VM
make vm-sync

# 2. SSH into VM
make vm-ssh

# 3. Run capture script
cd /root/go-synth
./scripts/capture-bsd-fixtures.sh

# 4. Exit VM and sync back
exit
make vm-sync-back  # or manually scp files back
```

## Using Fixtures in Tests

Tests can load fixtures using:

```go
func TestParseLoadavg(t *testing.T) {
    data, err := os.ReadFile("testdata/fixtures/vm.loadavg.bin")
    if err != nil {
        t.Fatal(err)
    }
    
    var la loadavg
    err = binary.Read(newBytesReader(data), binary.LittleEndian, &la)
    // ... test parsing logic
}
```

## Why Fixtures?

1. **Cross-platform testing**: Test BSD syscall parsing on Linux/macOS
2. **Reproducible**: Same test data every run
3. **Fast**: No need to boot VM for every test
4. **Documented**: Captures real system state at a point in time
5. **Regression protection**: Detects parsing changes

## Fixture Metadata

See `system_info.txt` for details about the system where fixtures were captured.

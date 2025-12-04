//go:build dragonfly || freebsd

package stats

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	fscale = 2048.0
)

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

func getAdjustedLoad() (float64, error) {
	rawLoad, err := unix.SysctlRaw("vm.loadavg")
	if err != nil {
		return 0.0, fmt.Errorf("sysctl vm.loadavg: %w", err)
	}

	if len(rawLoad) < int(unsafe.Sizeof(loadavg{})) {
		return 0.0, fmt.Errorf("vm.loadavg: unexpected size %d", len(rawLoad))
	}

	var la loadavg
	if err := binary.Read(newBytesReader(rawLoad), binary.LittleEndian, &la); err != nil {
		return 0.0, fmt.Errorf("parse vm.loadavg: %w", err)
	}

	load1min := float64(la.Load[0]) / fscale

	rawVmtotal, err := unix.SysctlRaw("vm.vmtotal")
	if err != nil {
		return load1min, nil
	}

	if len(rawVmtotal) < int(unsafe.Sizeof(vmtotal{})) {
		return load1min, nil
	}

	var vmt vmtotal
	if err := binary.Read(newBytesReader(rawVmtotal), binary.LittleEndian, &vmt); err != nil {
		return load1min, nil
	}

	return load1min + float64(vmt.T_pw), nil
}

func getSwapUsage() (int, error) {
	rawSwap, err := unix.SysctlRaw("vm.swap_info")
	if err != nil {
		return 0, fmt.Errorf("sysctl vm.swap_info: %w", err)
	}

	if len(rawSwap) == 0 {
		return 0, nil
	}

	entrySize := int(unsafe.Sizeof(xswdev{}))
	if len(rawSwap)%entrySize != 0 {
		return 0, fmt.Errorf("vm.swap_info: invalid size %d", len(rawSwap))
	}

	var totalBlks, usedBlks int32
	numEntries := len(rawSwap) / entrySize

	for i := 0; i < numEntries; i++ {
		offset := i * entrySize
		chunk := rawSwap[offset : offset+entrySize]

		var xs xswdev
		if err := binary.Read(newBytesReader(chunk), binary.LittleEndian, &xs); err != nil {
			continue
		}

		totalBlks += xs.Nblks
		usedBlks += xs.Used
	}

	if totalBlks == 0 {
		return 0, nil
	}

	return int((float64(usedBlks) / float64(totalBlks)) * 100.0), nil
}

type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data}
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

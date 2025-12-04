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
	load  [3]uint32
	scale int32
}

type vmtotal struct {
	t_rq     int16
	t_dw     int16
	t_pw     int16
	t_sl     int16
	t_sw     int16
	t_vm     uint32
	t_avm    uint32
	t_rm     uint32
	t_arm    uint32
	t_vmshr  uint32
	t_avmshr uint32
	t_rmshr  uint32
	t_armshr uint32
	t_free   uint32
}

type xswdev struct {
	version uint32
	dev     uint64
	flags   int32
	nblks   int32
	used    int32
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

	load1min := float64(la.load[0]) / fscale

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

	return load1min + float64(vmt.t_pw), nil
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

		totalBlks += xs.nblks
		usedBlks += xs.used
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

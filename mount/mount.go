package mount

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dsynth/config"

	"golang.org/x/sys/unix"
)

const (
	MountTypeMask = 0x000F
	MountTypeTmpfs = 0x0001
	MountTypeNullfs = 0x0002
	MountTypeDevfs = 0x0003
	MountTypeProcfs = 0x0004
	MountTypeRW = 0x0010
	MountTypeBig = 0x0020
	MountTypeMed = 0x0080
)

const (
	TmpfsRW    = MountTypeTmpfs | MountTypeRW
	TmpfsRWBig = MountTypeTmpfs | MountTypeRW | MountTypeBig
	TmpfsRWMed = MountTypeTmpfs | MountTypeRW | MountTypeMed
	NullfsRO   = MountTypeNullfs
	NullfsRW   = MountTypeNullfs | MountTypeRW
	DevfsRW    = MountTypeDevfs | MountTypeRW
	ProcfsRO   = MountTypeProcfs
)

type Worker struct {
	Index      int
	BaseDir    string
	MountError int
	AccumError int
	Status     string
}

// DoWorkerMounts sets up all filesystem mounts for a worker
func DoWorkerMounts(work *Worker, cfg *config.Config) error {
	work.MountError = 0

	// Create base directories
	if err := os.MkdirAll(work.BaseDir, 0755); err != nil {
		return fmt.Errorf("cannot create basedir: %w", err)
	}

	// Mount root tmpfs
	doMount(work, cfg, TmpfsRW, "dummy", "", "")

	// Create /usr structure
	dirs := []string{
		filepath.Join(work.BaseDir, "usr"),
		filepath.Join(work.BaseDir, "usr/packages"),
		filepath.Join(work.BaseDir, "boot"),
		filepath.Join(work.BaseDir, "boot/modules.local"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir %s failed: %v\n", dir, err)
			work.MountError++
		}
	}

	// System mounts
	doMount(work, cfg, TmpfsRW, "dummy", "/boot", "")
	doMount(work, cfg, DevfsRW, "dummy", "/dev", "")
	doMount(work, cfg, ProcfsRO, "dummy", "/proc", "")

	// Nullfs mounts from system
	doMount(work, cfg, NullfsRO, "$/bin", "/bin", "")
	doMount(work, cfg, NullfsRO, "$/sbin", "/sbin", "")
	doMount(work, cfg, NullfsRO, "$/lib", "/lib", "")
	doMount(work, cfg, NullfsRO, "$/libexec", "/libexec", "")
	doMount(work, cfg, NullfsRO, "$/usr/bin", "/usr/bin", "")
	doMount(work, cfg, NullfsRO, "$/usr/include", "/usr/include", "")
	doMount(work, cfg, NullfsRO, "$/usr/lib", "/usr/lib", "")
	doMount(work, cfg, NullfsRO, "$/usr/libdata", "/usr/libdata", "")
	doMount(work, cfg, NullfsRO, "$/usr/libexec", "/usr/libexec", "")
	doMount(work, cfg, NullfsRO, "$/usr/sbin", "/usr/sbin", "")
	doMount(work, cfg, NullfsRO, "$/usr/share", "/usr/share", "")
	doMount(work, cfg, NullfsRO, "$/usr/games", "/usr/games", "")

	if cfg.UseUsrSrc {
		doMount(work, cfg, NullfsRO, "$/usr/src", "/usr/src", "")
	}

	// Ports and build directories
	doMount(work, cfg, NullfsRO, cfg.DPortsPath, "/xports", "")
	doMount(work, cfg, NullfsRW, cfg.OptionsPath, "/options", "")
	doMount(work, cfg, NullfsRW, cfg.PackagesPath, "/packages", "")
	doMount(work, cfg, NullfsRW, cfg.DistFilesPath, "/distfiles", "")
	doMount(work, cfg, TmpfsRWBig, "dummy", "/construction", "")
	doMount(work, cfg, TmpfsRWMed, "dummy", "/usr/local", "")

	if cfg.UseCCache {
		doMount(work, cfg, NullfsRW, cfg.CCachePath, "/ccache", "")
	}

	// Copy template
	templatePath := filepath.Join(cfg.BuildBase, "Template")
	cmd := exec.Command("cp", "-Rp", templatePath+"/.", work.BaseDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("template copy failed: %w", err)
	}

	if work.MountError > 0 {
		return fmt.Errorf("mount errors occurred")
	}

	return nil
}

// DoWorkerUnmounts tears down all mounts for a worker
func DoWorkerUnmounts(work *Worker, cfg *config.Config) error {
	work.MountError = 0

	for retry := 0; retry < 10; retry++ {
		doUnmount(work, "/proc")
		doUnmount(work, "/dev")
		doUnmount(work, "/usr/src")
		doUnmount(work, "/usr/games")
		doUnmount(work, "/boot")
		doUnmount(work, "/usr/local")
		doUnmount(work, "/construction")
		doUnmount(work, "/ccache")
		doUnmount(work, "/distfiles")
		doUnmount(work, "/packages")
		doUnmount(work, "/options")
		doUnmount(work, "/xports")
		doUnmount(work, "/usr/share")
		doUnmount(work, "/usr/sbin")
		doUnmount(work, "/usr/libexec")
		doUnmount(work, "/usr/libdata")
		doUnmount(work, "/usr/lib")
		doUnmount(work, "/usr/include")
		doUnmount(work, "/usr/bin")
		doUnmount(work, "/libexec")
		doUnmount(work, "/lib")
		doUnmount(work, "/sbin")
		doUnmount(work, "/bin")
		doUnmount(work, "")

		if work.MountError == 0 {
			break
		}

		time.Sleep(5 * time.Second)
		work.MountError = 0
	}

	if work.MountError > 0 {
		return fmt.Errorf("unable to unmount all filesystems")
	}

	return nil
}

func doMount(work *Worker, cfg *config.Config, mountType int, spath, dpath, discreteFmt string) {
	// Resolve source path
	var source string
	if spath == "dummy" {
		source = "tmpfs"
	} else if strings.HasPrefix(spath, "$") {
		// System path
		sysPath := cfg.SystemPath
		if sysPath == "/" {
			source = spath[1:]
		} else {
			source = filepath.Join(sysPath, spath[1:])
		}
	} else {
		source = spath
	}

	// Resolve target path
	target := filepath.Join(work.BaseDir, dpath)

	// Create target directory
	if err := os.MkdirAll(target, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s failed: %v\n", target, err)
		work.MountError++
		return
	}

	// Determine mount options
	rwOpt := "ro"
	if mountType&MountTypeRW != 0 {
		rwOpt = "rw"
	}

	var fstype string
	var opts []string

	switch mountType & MountTypeMask {
	case MountTypeTmpfs:
		fstype = "tmpfs"
		opts = []string{rwOpt}
		if mountType&MountTypeBig != 0 {
			opts = append(opts, "size=64g")
		} else if mountType&MountTypeMed != 0 {
			opts = append(opts, "size=16g")
		} else {
			opts = append(opts, "size=16g")
		}

	case MountTypeNullfs:
		fstype = "nullfs"
		opts = []string{rwOpt}

	case MountTypeDevfs:
		fstype = "devfs"
		opts = []string{rwOpt}

	case MountTypeProcfs:
		fstype = "procfs"
		opts = []string{rwOpt}

	default:
		fmt.Fprintf(os.Stderr, "unknown mount type: %x\n", mountType)
		work.MountError++
		return
	}

	// Execute mount command
	// Note: On Linux this would use unix.Mount(), on BSDs we exec mount(8)
	optStr := strings.Join(opts, ",")
	cmd := exec.Command("mount", "-t", fstype, "-o", optStr, source, target)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "mount failed: %v (mount -t %s -o %s %s %s)\n",
			err, fstype, optStr, source, target)
		work.MountError++
	}
}

func doUnmount(work *Worker, rpath string) {
	target := filepath.Join(work.BaseDir, rpath)

	if err := unix.Unmount(target, 0); err != nil {
		switch err {
		case unix.EPERM, unix.ENOENT, unix.EINVAL:
			// Expected errors, ignore
		default:
			fmt.Fprintf(os.Stderr, "unmount %s failed: %v\n", target, err)
			work.MountError++
		}
	}
}
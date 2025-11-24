package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// Config holds all dsynth configuration
type Config struct {
	// Paths
	ConfigPath     string
	DPortsPath     string
	RepositoryPath string
	BuildBase      string
	DistFilesPath  string
	OptionsPath    string
	PackagesPath   string
	LogsPath       string
	SystemPath     string
	CCachePath     string

	// Build settings
	MaxWorkers   int
	MaxJobs      int
	SlowStart    int
	NumaMask     string
	UseSSCCBase  bool
	UseUsrSrc    bool
	UseCCache    bool
	UseTmpfs     bool
	UseVKernel   bool
	UsePKGDepend bool

	// Sizes
	TmpfsWorkSize     string
	TmpfsLocalbaseSize string
	TmpfsUsrLocalSize string

	// Behavior
	Debug      bool
	Force      bool
	YesAll     bool
	DevMode    bool
	CheckPlist bool
	DisableUI  bool

	// Profile
	Profile string
}

// LoadConfig loads configuration from file
func LoadConfig(configDir string, profile string) (*Config, error) {
	cfg := &Config{
		MaxWorkers: runtime.NumCPU() / 2,
		MaxJobs:    runtime.NumCPU(),
		SlowStart:  0,
		Profile:    profile,
		SystemPath: "/",
		UseUsrSrc:  false,
		UseCCache:  false,
		UseTmpfs:   true,
		TmpfsWorkSize: "64g",
		TmpfsLocalbaseSize: "16g",
		TmpfsUsrLocalSize: "16g",
	}

	if cfg.MaxWorkers < 1 {
		cfg.MaxWorkers = 1
	}

	// Determine config path
	if configDir == "" {
		if _, err := os.Stat("/etc/dsynth"); err == nil {
			configDir = "/etc/dsynth"
		} else if _, err := os.Stat("/usr/local/etc/dsynth"); err == nil {
			configDir = "/usr/local/etc/dsynth"
		} else {
			configDir = "/etc/dsynth"
		}
	}
	cfg.ConfigPath = configDir

	// Load config file if it exists
	configFile := filepath.Join(configDir, "dsynth.ini")
	if _, err := os.Stat(configFile); err == nil {
		if err := cfg.parseINI(configFile); err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}
	}

	// Apply defaults for unset paths
	if cfg.BuildBase == "" {
		cfg.BuildBase = "/build"
	}
	if cfg.DPortsPath == "" {
		cfg.DPortsPath = "/usr/dports"
		// Fall back to /usr/ports if dports doesn't exist
		if _, err := os.Stat(cfg.DPortsPath); err != nil {
			if _, err := os.Stat("/usr/ports"); err == nil {
				cfg.DPortsPath = "/usr/ports"
			}
		}
	}
	if cfg.RepositoryPath == "" {
		cfg.RepositoryPath = cfg.BuildBase + "/packages"
	}
	if cfg.DistFilesPath == "" {
		cfg.DistFilesPath = cfg.BuildBase + "/distfiles"
	}
	if cfg.OptionsPath == "" {
		cfg.OptionsPath = cfg.BuildBase + "/options"
	}
	if cfg.PackagesPath == "" {
		cfg.PackagesPath = cfg.RepositoryPath
	}
	if cfg.LogsPath == "" {
		cfg.LogsPath = cfg.BuildBase + "/logs"
	}
	if cfg.CCachePath == "" {
		cfg.CCachePath = cfg.BuildBase + "/ccache"
	}

	return cfg, nil
}

// parseINI parses an INI-format configuration file
func (cfg *Config) parseINI(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}

		// Skip if we have a profile and this isn't it
		if cfg.Profile != "" && currentSection != "" && currentSection != strings.ToLower(cfg.Profile) {
			continue
		}

		// Key-value pair
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes
		value = strings.Trim(value, "\"'")

		cfg.setConfigValue(key, value)
	}

	return scanner.Err()
}

func (cfg *Config) setConfigValue(key, value string) {
	// Normalize key (replace _ with space, lowercase, etc.)
	key = strings.ToLower(key)
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, " ", "")

	switch key {
	case "numberofbuilders", "builders", "workers":
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			cfg.MaxWorkers = n
		}
	case "maxjobs", "jobs":
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			cfg.MaxJobs = n
		}
	case "directorypackages", "packages":
		cfg.RepositoryPath = value
		cfg.PackagesPath = value
	case "directorybuildbase", "buildbase":
		cfg.BuildBase = value
	case "directoryportsdir", "portsdir", "dportsdir":
		cfg.DPortsPath = value
	case "directorydistfiles", "distfiles":
		cfg.DistFilesPath = value
	case "directoryoptions", "options":
		cfg.OptionsPath = value
	case "directorylogs", "logs":
		cfg.LogsPath = value
	case "systempath":
		cfg.SystemPath = value
	case "ccachedir", "ccache":
		cfg.CCachePath = value
		cfg.UseCCache = true
	case "useccache":
		cfg.UseCCache = parseBool(value)
	case "useusrsrc":
		cfg.UseUsrSrc = parseBool(value)
	case "usetmpfs":
		cfg.UseTmpfs = parseBool(value)
	case "usevkernel":
		cfg.UseVKernel = parseBool(value)
	case "usepkgdepend":
		cfg.UsePKGDepend = parseBool(value)
	case "tmpfsworksize":
		cfg.TmpfsWorkSize = value
	case "tmpfslocalbasesize":
		cfg.TmpfsLocalbaseSize = value
	case "tmpfsusrlocalsize":
		cfg.TmpfsUsrLocalSize = value
	case "numamask":
		cfg.NumaMask = value
	}
}

func parseBool(value string) bool {
	value = strings.ToLower(value)
	return value == "yes" || value == "true" || value == "1" || value == "on"
}

// WriteDefaultConfig writes a default configuration file
func WriteDefaultConfig(filename string, cfg *Config) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintln(file, "# dsynth configuration file")
	fmt.Fprintln(file, "# See dsynth(1) for details")
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "[Global Configuration]")
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "# Number of builders (workers)")
	fmt.Fprintf(file, "Number_of_builders=%d\n", cfg.MaxWorkers)
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "# Maximum jobs per builder")
	fmt.Fprintf(file, "Max_jobs=%d\n", cfg.MaxJobs)
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "# Directory paths")
	fmt.Fprintf(file, "Directory_packages=%s\n", cfg.RepositoryPath)
	fmt.Fprintf(file, "Directory_buildbase=%s\n", cfg.BuildBase)
	fmt.Fprintf(file, "Directory_portsdir=%s\n", cfg.DPortsPath)
	fmt.Fprintf(file, "Directory_distfiles=%s\n", cfg.DistFilesPath)
	fmt.Fprintf(file, "Directory_options=%s\n", cfg.OptionsPath)
	fmt.Fprintf(file, "Directory_logs=%s\n", cfg.LogsPath)
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "# System path (use / for native)")
	fmt.Fprintf(file, "System_path=%s\n", cfg.SystemPath)
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "# Use tmpfs for work directories")
	fmt.Fprintf(file, "Use_tmpfs=%v\n", cfg.UseTmpfs)
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "# Tmpfs sizes")
	fmt.Fprintf(file, "Tmpfs_worksize=%s\n", cfg.TmpfsWorkSize)
	fmt.Fprintf(file, "Tmpfs_localbasesize=%s\n", cfg.TmpfsLocalbaseSize)
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "# Use ccache")
	fmt.Fprintf(file, "Use_ccache=%v\n", cfg.UseCCache)
	if cfg.UseCCache {
		fmt.Fprintf(file, "Ccache_dir=%s\n", cfg.CCachePath)
	}
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "# Use /usr/src")
	fmt.Fprintf(file, "Use_usrsrc=%v\n", cfg.UseUsrSrc)
	fmt.Fprintln(file, "")

	return nil
}

// Validate checks configuration validity
func (cfg *Config) Validate() error {
	// Check required paths exist or can be created
	requiredDirs := map[string]string{
		"BuildBase":      cfg.BuildBase,
		"DPortsPath":     cfg.DPortsPath,
		"RepositoryPath": cfg.RepositoryPath,
		"DistFilesPath":  cfg.DistFilesPath,
	}

	for name, path := range requiredDirs {
		if path == "" {
			return fmt.Errorf("%s is not configured", name)
		}

		// Check if exists or is creatable
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				// Try to create it
				if err := os.MkdirAll(path, 0755); err != nil {
					return fmt.Errorf("%s directory %s cannot be created: %w", name, path, err)
				}
			} else {
				return fmt.Errorf("%s directory %s: %w", name, path, err)
			}
		} else if !info.IsDir() {
			return fmt.Errorf("%s path %s is not a directory", name, path)
		}
	}

	// Validate workers count
	if cfg.MaxWorkers < 1 {
		return fmt.Errorf("MaxWorkers must be at least 1")
	}
	if cfg.MaxWorkers > 1024 {
		return fmt.Errorf("MaxWorkers is too large (max 1024)")
	}

	return nil
}

// GetSystemInfo returns system information
func GetSystemInfo() (osname, osversion, arch string, ncpus int) {
	// Get OS information
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err == nil {
		osname = string(utsname.Sysname[:])
		osversion = string(utsname.Release[:])
		arch = string(utsname.Machine[:])
		// Trim null bytes
		osname = strings.TrimRight(osname, "\x00")
		osversion = strings.TrimRight(osversion, "\x00")
		arch = strings.TrimRight(arch, "\x00")
	}

	ncpus = runtime.NumCPU()

	return
}

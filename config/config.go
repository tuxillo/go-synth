package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"gopkg.in/ini.v1"
)

// Config holds go-synth configuration
type Config struct {
	Profile    string
	ConfigPath string

	BuildBase      string
	DPortsPath     string
	RepositoryPath string
	PackagesPath   string
	DistFilesPath  string
	OptionsPath    string
	LogsPath       string
	CCachePath     string
	SystemPath     string

	MaxWorkers int
	MaxJobs    int
	SlowStart  int

	UseCCache    bool
	UseUsrSrc    bool
	UseTmpfs     bool
	UseVKernel   bool
	UsePKGDepend bool

	Debug      bool
	Force      bool
	YesAll     bool
	DevMode    bool
	CheckPlist bool
	DisableUI  bool

	// Migration settings
	Migration struct {
		AutoMigrate  bool // Default: true
		BackupLegacy bool // Default: true
	}

	// Database settings
	Database struct {
		Path       string // Default: ${BuildBase}/builds.db
		AutoVacuum bool   // Default: true
	}
}

var globalConfig *Config

// GetConfig returns the global configuration
func GetConfig() *Config {
	return globalConfig
}

// SetConfig sets the global configuration
func SetConfig(cfg *Config) {
	globalConfig = cfg
}

// LoadConfig loads configuration from file
func LoadConfig(configDir, profile string) (*Config, error) {
	// Determine sensible defaults based on system resources
	defaultWorkers := runtime.NumCPU()
	// Cap at 16 workers to avoid overwhelming the system
	// User can override via config file if they want more
	if defaultWorkers > 16 {
		defaultWorkers = 16
	}
	// Minimum of 1 worker
	if defaultWorkers < 1 {
		defaultWorkers = 1
	}

	cfg := &Config{
		Profile:    profile,
		MaxWorkers: defaultWorkers,
		MaxJobs:    1,
	}

	// Determine config file path
	configFile := "/etc/dsynth/dsynth.ini"
	if configDir != "" {
		configFile = configDir + "/dsynth.ini"
	}
	cfg.ConfigPath = configFile

	// Try to load config file
	configFileExists := false
	if _, err := os.Stat(configFile); err == nil {
		configFileExists = true
		iniFile, err := ini.Load(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}

		// If no profile specified, read from global section
		if cfg.Profile == "" || cfg.Profile == "default" {
			globalSec := iniFile.Section("Global Configuration")
			if globalSec == nil {
				globalSec = iniFile.Section("global configuration")
			}
			if globalSec == nil {
				globalSec = iniFile.Section("Global")
			}

			if globalSec != nil {
				if key := globalSec.Key("profile_selected"); key != nil {
					cfg.Profile = key.String()
					if cfg.Profile != "" {
						fmt.Printf("Auto-selected profile from config: %s\n", cfg.Profile)
					}
				}
			}
		}

		// Load the profile section
		if cfg.Profile != "" {
			profileSec := iniFile.Section(cfg.Profile)
			if profileSec != nil {
				cfg.loadFromSection(profileSec)
			}
		}

		// Also load from global section for any unset values
		globalSec := iniFile.Section("Global Configuration")
		if globalSec == nil {
			globalSec = iniFile.Section("global configuration")
		}
		if globalSec != nil {
			cfg.loadFromSection(globalSec)
		}
	}

	// Warn if no config file was found and using defaults
	if !configFileExists {
		fmt.Fprintf(os.Stderr, "Warning: No config file found at %s\n", configFile)
		fmt.Fprintf(os.Stderr, "Using defaults: %d workers (detected from CPU count)\n", cfg.MaxWorkers)
		fmt.Fprintf(os.Stderr, "Run 'go-synth init' to create a config file, or override with config file settings.\n")
	}

	// Apply defaults for unset paths
	if cfg.BuildBase == "" {
		cfg.BuildBase = "/build/synth"
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

	// Apply defaults for Migration settings (default to true)
	// These are only false if explicitly set in config
	if !cfg.Migration.AutoMigrate && !cfg.Migration.BackupLegacy {
		// Neither was explicitly set, apply defaults
		cfg.Migration.AutoMigrate = true
		cfg.Migration.BackupLegacy = true
	} else if cfg.Migration.AutoMigrate && !cfg.Migration.BackupLegacy {
		// AutoMigrate was set but BackupLegacy wasn't, default it
		cfg.Migration.BackupLegacy = true
	} else if !cfg.Migration.AutoMigrate && cfg.Migration.BackupLegacy {
		// BackupLegacy was set but AutoMigrate wasn't, default it
		cfg.Migration.AutoMigrate = true
	}

	// Apply defaults for Database settings
	if cfg.Database.Path == "" {
		cfg.Database.Path = cfg.BuildBase + "/builds.db"
	}
	cfg.Database.AutoVacuum = true // Always default to true for MVP

	return cfg, nil
}

// loadFromSection loads config values from an INI section
func (cfg *Config) loadFromSection(sec *ini.Section) {
	// Skip if section is nil
	if sec == nil {
		return
	}

	// Directory paths
	if key := sec.Key("Directory_buildbase"); key != nil && key.String() != "" {
		cfg.BuildBase = key.String()
	}
	if key := sec.Key("Directory_portsdir"); key != nil && key.String() != "" {
		cfg.DPortsPath = key.String()
	}
	if key := sec.Key("Directory_repository"); key != nil && key.String() != "" {
		cfg.RepositoryPath = key.String()
	}
	if key := sec.Key("Directory_packages"); key != nil && key.String() != "" {
		cfg.PackagesPath = key.String()
	}
	if key := sec.Key("Directory_distfiles"); key != nil && key.String() != "" {
		cfg.DistFilesPath = key.String()
	}
	if key := sec.Key("Directory_options"); key != nil && key.String() != "" {
		cfg.OptionsPath = key.String()
	}
	if key := sec.Key("Directory_logs"); key != nil && key.String() != "" {
		cfg.LogsPath = key.String()
	}
	if key := sec.Key("Directory_ccache"); key != nil && key.String() != "" {
		cfg.CCachePath = key.String()
	}
	if key := sec.Key("Directory_system"); key != nil && key.String() != "" {
		cfg.SystemPath = key.String()
	}

	// Worker settings
	if key := sec.Key("Number_of_builders"); key != nil {
		if n, err := key.Int(); err == nil && n > 0 {
			cfg.MaxWorkers = n
		}
	}
	if key := sec.Key("Max_jobs_per_builder"); key != nil {
		if n, err := key.Int(); err == nil && n > 0 {
			cfg.MaxJobs = n
		}
	}

	// Boolean options
	if key := sec.Key("Tmpfs_workdir"); key != nil {
		cfg.UseTmpfs = cfg.UseTmpfs || parseBool(key.String())
	}
	if key := sec.Key("Tmpfs_localbase"); key != nil {
		cfg.UseTmpfs = cfg.UseTmpfs || parseBool(key.String())
	}
	if key := sec.Key("Display_with_ncurses"); key != nil {
		cfg.DisableUI = !parseBool(key.String())
	}
	if key := sec.Key("leverage_prebuilt"); key != nil {
		// TODO: Implement if needed
		_ = key
	}

	// Migration settings
	if key := sec.Key("Migration_auto_migrate"); key != nil {
		cfg.Migration.AutoMigrate = parseBool(key.String())
	}
	if key := sec.Key("Migration_backup_legacy"); key != nil {
		cfg.Migration.BackupLegacy = parseBool(key.String())
	}

	// Database settings
	if key := sec.Key("Database_path"); key != nil && key.String() != "" {
		cfg.Database.Path = key.String()
	}
	if key := sec.Key("Database_auto_vacuum"); key != nil {
		cfg.Database.AutoVacuum = parseBool(key.String())
	}
}

func parseBool(s string) bool {
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}
	// Handle yes/no
	s = s
	return s == "yes" || s == "Yes" || s == "YES" || s == "1" || s == "on" || s == "On" || s == "ON"
}

func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// SaveConfig writes the current configuration to disk in INI format.
// The path parameter takes precedence; when empty, cfg.ConfigPath is used,
// falling back to the default /etc/dsynth/dsynth.ini.
func SaveConfig(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	targetPath := path
	if targetPath == "" {
		targetPath = cfg.ConfigPath
	}
	if targetPath == "" {
		targetPath = "/etc/dsynth/dsynth.ini"
	}

	iniFile := ini.Empty()
	section, err := iniFile.NewSection("Global Configuration")
	if err != nil {
		return fmt.Errorf("failed to create config section: %w", err)
	}

	profile := cfg.Profile
	if profile == "" {
		profile = "default"
	}
	section.Key("profile_selected").SetValue(profile)

	setStr := func(key, value string) {
		if value != "" {
			section.Key(key).SetValue(value)
		}
	}

	setStr("Directory_buildbase", cfg.BuildBase)
	setStr("Directory_portsdir", cfg.DPortsPath)
	setStr("Directory_repository", cfg.RepositoryPath)
	setStr("Directory_packages", cfg.PackagesPath)
	setStr("Directory_distfiles", cfg.DistFilesPath)
	setStr("Directory_options", cfg.OptionsPath)
	setStr("Directory_logs", cfg.LogsPath)
	setStr("Directory_ccache", cfg.CCachePath)
	setStr("Directory_system", cfg.SystemPath)

	section.Key("Number_of_builders").SetValue(strconv.Itoa(cfg.MaxWorkers))
	section.Key("Max_jobs_per_builder").SetValue(strconv.Itoa(cfg.MaxJobs))

	section.Key("Tmpfs_workdir").SetValue(boolToYesNo(cfg.UseTmpfs))
	section.Key("Tmpfs_localbase").SetValue(boolToYesNo(cfg.UseTmpfs))
	section.Key("Display_with_ncurses").SetValue(boolToYesNo(!cfg.DisableUI))

	section.Key("Migration_auto_migrate").SetValue(boolToYesNo(cfg.Migration.AutoMigrate))
	section.Key("Migration_backup_legacy").SetValue(boolToYesNo(cfg.Migration.BackupLegacy))

	setStr("Database_path", cfg.Database.Path)
	section.Key("Database_auto_vacuum").SetValue(boolToYesNo(cfg.Database.AutoVacuum))

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	tmpPath := targetPath + ".tmp"
	if err := iniFile.SaveTo(tmpPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to finalize config: %w", err)
	}

	cfg.ConfigPath = targetPath
	return nil
}

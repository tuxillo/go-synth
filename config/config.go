package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds dsynth configuration
type Config struct {
	Profile        string
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
	cfg := &Config{
		Profile:    profile,
		MaxWorkers: 1,
		MaxJobs:    1,
	}

	// Determine config file path
	configFile := "/etc/dsynth/dsynth.ini"
	if configDir != "" {
		configFile = configDir + "/dsynth.ini"
	}

	// Try to load config file
	if _, err := os.Stat(configFile); err == nil {
		// First pass: read global config to get profile_selected if no profile specified
		if cfg.Profile == "" {
			if selectedProfile, err := readProfileSelected(configFile); err == nil && selectedProfile != "" {
				cfg.Profile = selectedProfile
			}
		}

		// Second pass: parse the full config with the selected profile
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

// readProfileSelected reads the profile_selected value from the global section
func readProfileSelected(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inGlobalSection := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.ToLower(strings.Trim(line, "[]"))
			inGlobalSection = (section == "global configuration" || section == "global")
			continue
		}

		// Look for profile_selected in global section
		if inGlobalSection {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				value = strings.Trim(value, "\"' ")

				// Normalize key
				keyNorm := strings.ToLower(key)
				keyNorm = strings.ReplaceAll(keyNorm, "_", "")
				keyNorm = strings.ReplaceAll(keyNorm, " ", "")

				if keyNorm == "profileselected" {
					return value, nil
				}
			}
		}
	}

	return "", scanner.Err()
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

		// Skip if we have a profile and this isn't it (and not global section)
		if cfg.Profile != "" && currentSection != "" {
			isGlobal := (currentSection == "global configuration" || currentSection == "global")
			isTargetProfile := (currentSection == strings.ToLower(cfg.Profile))
			
			if !isGlobal && !isTargetProfile {
				continue
			}
		}

		// Key-value pair
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes and extra spaces
		value = strings.Trim(value, "\"' ")

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
	case "profileselected":
		// Skip - already handled in readProfileSelected
		return
	case "numberofbuilders", "builders", "workers":
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			cfg.MaxWorkers = n
		}
	case "maxjobs", "jobs", "maxjobsperbuilder":
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			cfg.MaxJobs = n
		}
	case "directorypackages", "packages":
		cfg.PackagesPath = value
	case "directoryrepository", "repository":
		cfg.RepositoryPath = value
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
	case "directorysystem", "systempath":
		cfg.SystemPath = value
	case "directoryccache", "ccachedir", "ccache":
		cfg.CCachePath = value
		cfg.UseCCache = true
	case "useccache":
		cfg.UseCCache = parseBool(value)
	case "useusrsrc":
		cfg.UseUsrSrc = parseBool(value)
	case "usetmpfs", "tmpfsworkdir", "tmpfslocalbase":
		cfg.UseTmpfs = cfg.UseTmpfs || parseBool(value)
	case "usevkernel":
		cfg.UseVKernel = parseBool(value)
	case "usepkgdepend":
		cfg.UsePKGDepend = parseBool(value)
	case "operatingsystem":
		// Informational only, ignore
	case "packagesuffix":
		// TODO: Store package suffix if needed
	case "displaywithncurses":
		cfg.DisableUI = !parseBool(value)
	case "leverageprebuilt":
		// TODO: Implement if needed
	}
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "yes" || s == "1" || s == "on"
}
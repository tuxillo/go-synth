package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/ini.v1"
)

func TestParseBool(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"true lowercase", "true", true},
		{"false lowercase", "false", false},
		{"yes lowercase", "yes", true},
		{"Yes capitalized", "Yes", true},
		{"YES uppercase", "YES", true},
		{"no lowercase", "no", false},
		{"1 as string", "1", true},
		{"0 as string", "0", false},
		{"on lowercase", "on", true},
		{"On capitalized", "On", true},
		{"ON uppercase", "ON", true},
		{"off lowercase", "off", false},
		{"random string", "random", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBool(tt.input)
			if result != tt.expected {
				t.Errorf("parseBool(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	// Test loading config with no file (should use defaults)
	cfg, err := LoadConfig("/nonexistent/path", "")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Check default values
	if cfg.BuildBase != "/build/synth" {
		t.Errorf("BuildBase = %q, want %q", cfg.BuildBase, "/build/synth")
	}
	if cfg.DPortsPath != "/usr/dports" && cfg.DPortsPath != "/usr/ports" {
		t.Errorf("DPortsPath = %q, want /usr/dports or /usr/ports", cfg.DPortsPath)
	}
	if cfg.RepositoryPath != "/build/synth/packages" {
		t.Errorf("RepositoryPath = %q, want %q", cfg.RepositoryPath, "/build/synth/packages")
	}
	if cfg.DistFilesPath != "/build/synth/distfiles" {
		t.Errorf("DistFilesPath = %q, want %q", cfg.DistFilesPath, "/build/synth/distfiles")
	}
	if cfg.OptionsPath != "/build/synth/options" {
		t.Errorf("OptionsPath = %q, want %q", cfg.OptionsPath, "/build/synth/options")
	}
	if cfg.PackagesPath != "/build/synth/packages" {
		t.Errorf("PackagesPath = %q, want %q", cfg.PackagesPath, "/build/synth/packages")
	}
	if cfg.LogsPath != "/build/synth/logs" {
		t.Errorf("LogsPath = %q, want %q", cfg.LogsPath, "/build/synth/logs")
	}
	if cfg.CCachePath != "/build/synth/ccache" {
		t.Errorf("CCachePath = %q, want %q", cfg.CCachePath, "/build/synth/ccache")
	}

	expectedWorkers := runtime.NumCPU()
	if expectedWorkers > 16 {
		expectedWorkers = 16
	}
	if expectedWorkers < 1 {
		expectedWorkers = 1
	}
	if cfg.MaxWorkers != expectedWorkers {
		t.Errorf("MaxWorkers = %d, want %d", cfg.MaxWorkers, expectedWorkers)
	}
	if cfg.MaxJobs != 1 {
		t.Errorf("MaxJobs = %d, want 1", cfg.MaxJobs)
	}
}

func TestConfig_LoadFromFile(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "dsynth.ini")

	// Write test config
	configContent := `[Global Configuration]
profile_selected=test-profile

[test-profile]
Directory_buildbase=/custom/build
Directory_portsdir=/custom/ports
Directory_repository=/custom/packages
Directory_distfiles=/custom/distfiles
Directory_options=/custom/options
Directory_logs=/custom/logs
Directory_ccache=/custom/ccache
Directory_system=/custom/system
Number_of_builders=4
Max_jobs_per_builder=8
Tmpfs_workdir=yes
Display_with_ncurses=no
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load config
	cfg, err := LoadConfig(tempDir, "")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify values from config file
	if cfg.Profile != "test-profile" {
		t.Errorf("Profile = %q, want %q", cfg.Profile, "test-profile")
	}
	if cfg.BuildBase != "/custom/build" {
		t.Errorf("BuildBase = %q, want %q", cfg.BuildBase, "/custom/build")
	}
	if cfg.DPortsPath != "/custom/ports" {
		t.Errorf("DPortsPath = %q, want %q", cfg.DPortsPath, "/custom/ports")
	}
	if cfg.RepositoryPath != "/custom/packages" {
		t.Errorf("RepositoryPath = %q, want %q", cfg.RepositoryPath, "/custom/packages")
	}
	if cfg.DistFilesPath != "/custom/distfiles" {
		t.Errorf("DistFilesPath = %q, want %q", cfg.DistFilesPath, "/custom/distfiles")
	}
	if cfg.OptionsPath != "/custom/options" {
		t.Errorf("OptionsPath = %q, want %q", cfg.OptionsPath, "/custom/options")
	}
	if cfg.LogsPath != "/custom/logs" {
		t.Errorf("LogsPath = %q, want %q", cfg.LogsPath, "/custom/logs")
	}
	if cfg.CCachePath != "/custom/ccache" {
		t.Errorf("CCachePath = %q, want %q", cfg.CCachePath, "/custom/ccache")
	}
	if cfg.SystemPath != "/custom/system" {
		t.Errorf("SystemPath = %q, want %q", cfg.SystemPath, "/custom/system")
	}
	if cfg.MaxWorkers != 4 {
		t.Errorf("MaxWorkers = %d, want 4", cfg.MaxWorkers)
	}
	if cfg.MaxJobs != 8 {
		t.Errorf("MaxJobs = %d, want 8", cfg.MaxJobs)
	}
	if !cfg.UseTmpfs {
		t.Error("UseTmpfs = false, want true")
	}
	if !cfg.DisableUI {
		t.Error("DisableUI = false, want true")
	}
}

func TestConfig_ExplicitProfile(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "dsynth.ini")

	// Write test config with multiple profiles
	configContent := `[Global Configuration]
profile_selected=default-profile

[default-profile]
Directory_buildbase=/default/build

[custom-profile]
Directory_buildbase=/custom/build
Number_of_builders=2
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load config with explicit profile
	cfg, err := LoadConfig(tempDir, "custom-profile")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Should use custom-profile, not default-profile
	if cfg.Profile != "custom-profile" {
		t.Errorf("Profile = %q, want %q", cfg.Profile, "custom-profile")
	}
	if cfg.BuildBase != "/custom/build" {
		t.Errorf("BuildBase = %q, want %q", cfg.BuildBase, "/custom/build")
	}
	if cfg.MaxWorkers != 2 {
		t.Errorf("MaxWorkers = %d, want 2", cfg.MaxWorkers)
	}
}

func TestConfig_GlobalFallback(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "dsynth.ini")

	// Write test config with profile and global section
	// Note: Current implementation has global override profile (not fallback)
	// Profile sets some values, global section will override if present
	configContent := `[Global Configuration]
Directory_portsdir=/global/ports
Number_of_builders=10

[test-profile]
Directory_buildbase=/profile/build
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load config
	cfg, err := LoadConfig(tempDir, "test-profile")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Profile value (not in global) should be kept
	if cfg.BuildBase != "/profile/build" {
		t.Errorf("BuildBase = %q, want %q", cfg.BuildBase, "/profile/build")
	}
	// Global value should be loaded for values not in profile
	if cfg.DPortsPath != "/global/ports" {
		t.Errorf("DPortsPath = %q, want %q", cfg.DPortsPath, "/global/ports")
	}
	if cfg.MaxWorkers != 10 {
		t.Errorf("MaxWorkers = %d, want 10", cfg.MaxWorkers)
	}
}

func TestConfig_TmpfsLocalbase(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "dsynth.ini")

	// Test Tmpfs_localbase setting
	configContent := `[test-profile]
Tmpfs_localbase=yes
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(tempDir, "test-profile")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if !cfg.UseTmpfs {
		t.Error("UseTmpfs = false, want true (from Tmpfs_localbase)")
	}
}

func TestConfig_InvalidConfigFile(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "dsynth.ini")

	// Write invalid INI content
	if err := os.WriteFile(configFile, []byte("invalid[[[ini]]]content"), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Should return error
	_, err := LoadConfig(tempDir, "")
	if err == nil {
		t.Error("LoadConfig should fail with invalid config file")
	}
}

func TestGetSetConfig(t *testing.T) {
	// Save original config
	original := globalConfig

	// Test SetConfig and GetConfig
	testCfg := &Config{
		Profile:    "test",
		BuildBase:  "/test",
		MaxWorkers: 5,
	}

	SetConfig(testCfg)
	retrieved := GetConfig()

	if retrieved != testCfg {
		t.Error("GetConfig did not return the same config set by SetConfig")
	}
	if retrieved.Profile != "test" {
		t.Errorf("Profile = %q, want %q", retrieved.Profile, "test")
	}
	if retrieved.BuildBase != "/test" {
		t.Errorf("BuildBase = %q, want %q", retrieved.BuildBase, "/test")
	}
	if retrieved.MaxWorkers != 5 {
		t.Errorf("MaxWorkers = %d, want 5", retrieved.MaxWorkers)
	}

	// Restore original
	globalConfig = original
}

func TestConfig_CaseInsensitiveSections(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "dsynth.ini")

	// Test that "Global Configuration" section works
	// The code checks for "Global Configuration", then "global configuration", then "Global"
	configContent := `[Global Configuration]
profile_selected=test1
[test1]
Directory_buildbase=/test1
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(tempDir, "")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Profile != "test1" {
		t.Errorf("Profile = %q, want %q", cfg.Profile, "test1")
	}
	if cfg.BuildBase != "/test1" {
		t.Errorf("BuildBase = %q, want %q", cfg.BuildBase, "/test1")
	}
}

func TestConfig_MultipleProfiles(t *testing.T) {
	// Test that we can have multiple profiles (without conflicting global values)
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "dsynth.ini")

	configContent := `[profile1]
Directory_buildbase=/profile1
Number_of_builders=2

[profile2]
Directory_buildbase=/profile2
Number_of_builders=4
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	tests := []struct {
		name          string
		profile       string
		expectBase    string
		expectWorkers int
	}{
		{"profile1", "profile1", "/profile1", 2},
		{"profile2", "profile2", "/profile2", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig(tempDir, tt.profile)
			if err != nil {
				t.Fatalf("LoadConfig failed: %v", err)
			}

			if cfg.Profile != tt.profile {
				t.Errorf("Profile = %q, want %q", cfg.Profile, tt.profile)
			}
			if cfg.BuildBase != tt.expectBase {
				t.Errorf("BuildBase = %q, want %q", cfg.BuildBase, tt.expectBase)
			}
			if cfg.MaxWorkers != tt.expectWorkers {
				t.Errorf("MaxWorkers = %d, want %d", cfg.MaxWorkers, tt.expectWorkers)
			}
		})
	}
}

func TestConfig_DerivedPaths(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "dsynth.ini")

	// Only set BuildBase - others should derive from it
	configContent := `[test-profile]
Directory_buildbase=/base
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(tempDir, "test-profile")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify derived paths
	if cfg.RepositoryPath != "/base/packages" {
		t.Errorf("RepositoryPath = %q, want %q", cfg.RepositoryPath, "/base/packages")
	}
	if cfg.PackagesPath != "/base/packages" {
		t.Errorf("PackagesPath = %q, want %q", cfg.PackagesPath, "/base/packages")
	}
	if cfg.DistFilesPath != "/base/distfiles" {
		t.Errorf("DistFilesPath = %q, want %q", cfg.DistFilesPath, "/base/distfiles")
	}
	if cfg.OptionsPath != "/base/options" {
		t.Errorf("OptionsPath = %q, want %q", cfg.OptionsPath, "/base/options")
	}
	if cfg.LogsPath != "/base/logs" {
		t.Errorf("LogsPath = %q, want %q", cfg.LogsPath, "/base/logs")
	}
	if cfg.CCachePath != "/base/ccache" {
		t.Errorf("CCachePath = %q, want %q", cfg.CCachePath, "/base/ccache")
	}
}

func TestConfig_CustomPackagesPath(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "dsynth.ini")

	// Set custom packages path different from repository
	configContent := `[test-profile]
Directory_repository=/repo
Directory_packages=/custom/packages
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(tempDir, "test-profile")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify custom packages path is respected
	if cfg.RepositoryPath != "/repo" {
		t.Errorf("RepositoryPath = %q, want %q", cfg.RepositoryPath, "/repo")
	}
	if cfg.PackagesPath != "/custom/packages" {
		t.Errorf("PackagesPath = %q, want %q", cfg.PackagesPath, "/custom/packages")
	}
}

func TestConfig_ZeroAndNegativeWorkers(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "dsynth.ini")

	defaultWorkers := runtime.NumCPU()
	if defaultWorkers > 16 {
		defaultWorkers = 16
	}
	if defaultWorkers < 1 {
		defaultWorkers = 1
	}

	tests := []struct {
		name          string
		buildersValue string
		jobsValue     string
		expectWorkers int
		expectJobs    int
	}{
		{"zero builders", "0", "1", defaultWorkers, 1},      // Should keep default
		{"negative builders", "-1", "1", defaultWorkers, 1}, // Should keep default
		{"zero jobs", "2", "0", 2, 1},                       // Should keep default
		{"negative jobs", "2", "-1", 2, 1},                  // Should keep default
		{"valid values", "4", "8", 4, 8},                    // Should use config
		{"invalid builders", "abc", "1", defaultWorkers, 1}, // Should keep default
		{"invalid jobs", "2", "xyz", 2, 1},                  // Should keep default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configContent := "[test-profile]\n" +
				"Number_of_builders=" + tt.buildersValue + "\n" +
				"Max_jobs_per_builder=" + tt.jobsValue + "\n"

			if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			cfg, err := LoadConfig(tempDir, "test-profile")
			if err != nil {
				t.Fatalf("LoadConfig failed: %v", err)
			}

			if cfg.MaxWorkers != tt.expectWorkers {
				t.Errorf("MaxWorkers = %d, want %d", cfg.MaxWorkers, tt.expectWorkers)
			}
			if cfg.MaxJobs != tt.expectJobs {
				t.Errorf("MaxJobs = %d, want %d", cfg.MaxJobs, tt.expectJobs)
			}
		})
	}
}

func TestSaveConfigWritesIni(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Profile:        "default",
		BuildBase:      filepath.Join(tmpDir, "build"),
		DPortsPath:     filepath.Join(tmpDir, "dports"),
		RepositoryPath: filepath.Join(tmpDir, "repo"),
		PackagesPath:   filepath.Join(tmpDir, "packages"),
		DistFilesPath:  filepath.Join(tmpDir, "dist"),
		OptionsPath:    filepath.Join(tmpDir, "options"),
		LogsPath:       filepath.Join(tmpDir, "logs"),
		CCachePath:     filepath.Join(tmpDir, "ccache"),
		SystemPath:     "/",
		MaxWorkers:     4,
		MaxJobs:        2,
		UseTmpfs:       true,
	}
	cfg.Migration.AutoMigrate = true
	cfg.Migration.BackupLegacy = true
	cfg.Database.Path = filepath.Join(tmpDir, "builds.db")
	cfg.Database.AutoVacuum = true

	configPath := filepath.Join(tmpDir, "etc", "dsynth", "dsynth.ini")
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() failed: %v", err)
	}

	iniFile, err := ini.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	sec := iniFile.Section("Global Configuration")
	if sec.Key("Directory_buildbase").String() != cfg.BuildBase {
		t.Fatalf("Directory_buildbase mismatch: %s", sec.Key("Directory_buildbase").String())
	}
	if got := sec.Key("Number_of_builders").String(); got != "4" {
		t.Fatalf("Number_of_builders mismatch: %s", got)
	}
	if sec.Key("Tmpfs_workdir").String() != "yes" {
		t.Fatalf("Tmpfs_workdir should be yes, got %s", sec.Key("Tmpfs_workdir").String())
	}
	if got := sec.Key("Database_path").String(); got != cfg.Database.Path {
		t.Fatalf("Database_path mismatch: got %s want %s", got, cfg.Database.Path)
	}

	if cfg.ConfigPath != configPath {
		t.Fatalf("ConfigPath not updated, got %s want %s", cfg.ConfigPath, configPath)
	}
}

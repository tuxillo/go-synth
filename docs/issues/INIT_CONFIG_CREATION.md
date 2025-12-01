# Issue #4: `go-synth init` does not create configuration files

**Status**: 🟢 Resolved (2025-12-01)  
**Priority**: P1 (blocks first-run UX)  
**Discovered**: 2025-12-01  
**Component**: `main.go`, `service/init.go`, docs  

---

## Resolution Summary

- Added `config.SaveConfig()` to serialize the in-memory settings to `dsynth.ini`, mirroring the keys consumed by `LoadConfig`.
- `go-synth init` now calls `config.SaveConfig` after successful initialization, writing `/etc/dsynth/dsynth.ini` (or the `-C` override path) when the file is missing. Errors are surfaced as warnings so users know to rerun with proper permissions.
- Configuration tests cover round-tripping via the new helper.

## Problem Statement

Documentation (AGENTS.md §Configuration Files, QUICKSTART.md, docs/issues/WORKER_SLOT_ASSIGNMENT.md) states that running `go-synth init` will create `/etc/dsynth/dsynth.ini`. In practice, the CLI never writes a configuration file: `doInit()` (main.go:191-275) calls `svc.Initialize()` and only prints directory/database setup. The service layer (`service/init.go`) creates directories, templates, and BuildDB entries but contains **no code** that writes any config. As a result, first-time users still have to create `/etc/dsynth/dsynth.ini` manually, causing confusing warnings about missing configs despite having run `go-synth init`.

---

## Evidence

1. `service/init.go` only performs `os.MkdirAll`, template creation, BuildDB init, and migration checks. There is no call to a config-writer.
2. `main.go:191-275` displays directory setup and “Next steps” but never attempts to write `/etc/dsynth/dsynth.ini` nor prompt for config values.
3. Running `go-synth init` on the VM still leaves `/etc/dsynth/dsynth.ini` absent; every subsequent command prints:
   ```
   Warning: No config file found at /etc/dsynth/dsynth.ini
   Using defaults: 8 workers (detected from CPU count)
   Run 'go-synth init' to create a config file, or override with config file settings.
   ```
   —a circular instruction.

---

## Root Cause

- Legacy dsynth created the INI file during `dsynth init`. The Go implementation never re-implemented that step; it assumes a config exists and only initializes runtime directories/build DB.
- Documentation was migrated from the C project and still claims `init` creates the file.
- No helper exists in `config/` to render the current in-memory config back to disk.

---

## Impact

- First-time users cannot acquire a valid config automatically and must craft `/etc/dsynth/dsynth.ini` by hand.
- CLI repeatedly warns about missing config even after init, eroding trust.
- Automation/scripts relying on `go-synth init` to prepare systems fail unless an external tool writes the config.

---

## Proposed Fix

1. **Config Serialization:** Add a function in `config/` (e.g., `SaveConfig(path string, cfg *Config)`) that renders the current settings (respecting comments/order) to INI format.
2. **Init Writing:** During `go-synth init`, after successful initialization, write `/etc/dsynth/dsynth.ini` (and/or `/usr/local/etc/dsynth/dsynth.ini`) if it does not exist, honoring `-C` overrides and `YesAll` semantics.
3. **Interactive Configure:** Optionally tie into `configure` command to prompt users before writing.
4. **Docs Update:** Adjust AGENTS.md/QUICKSTART.md to reflect actual behavior once implemented.

---

## Status / Next Steps

- [x] Implement config save helper in `config/`
- [x] Teach `doInit` (or service layer) to call it when appropriate
- [x] Update documentation and tests (`config/config_test.go`) to cover config creation

> Tracking reference: `DEVELOPMENT.md` Known Issues → Issue #4.

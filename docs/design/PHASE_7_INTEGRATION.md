# Phase 7: Integration & Migration

## Goals
- Wire `pkg`, `builddb`, `builder`, and `environment` into CLI.
- Introduce minimal migration/fallback for legacy CRC state.

## Initialization Sequence
1. Load config.
2. Open BuildDB (BoltDB); if unavailable, use legacy CRC fallback.
3. Parse ports; resolve deps; compute order.
4. Run builder with environment.

## CLI Mapping
- `dsynth build [ports...]` → uses new pipeline; prompts can be bypassed with `-y`.
- `dsynth force` → bypass `NeedsBuild`.

## Migration Decision Tree
- If BoltDB present → use it.
- Else initialize BoltDB; populate `crc_index` lazily after successful builds.

## Logging (MVP)
- Continue existing file logs; no streaming; include UUID in messages.

## Exit Criteria
- End-to-end build via CLI works; CRC skip validated across two runs.

## Dependencies
- Phases 1–6 complete.
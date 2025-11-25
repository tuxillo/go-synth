# Phase 2: Minimal BuildDB (BoltDB)

## Goals
- Add minimal persistent tracking of build attempts and CRCs using BoltDB.
- Enable incremental builds by skipping unchanged ports.

## Schema (Buckets)
- `builds`: `uuid -> {portdir, version, status, start_time, end_time}`
- `packages`: `portdir@version -> uuid` (latest successful)
- `crc_index`: `portdir -> crc32`

## API (Proposed)
```go
type BuildRecord struct {
    UUID      string
    PortDir   string
    Version   string
    Status    string // running|success|failed
    StartTime time.Time
    EndTime   time.Time
}

func SaveRecord(rec *BuildRecord) error
func GetRecord(uuid string) (*BuildRecord, error)
func LatestFor(portDir, version string) (*BuildRecord, error)
func NeedsBuild(portDir string, crc uint32) bool
func UpdateCRC(portDir string, crc uint32) error
```

## Key Decisions
- Backend: BoltDB (pure Go, embedded, ACID).
- Keys: ASCII strings; `packages` key: `portdir@version`.

## Migration (Minimal)
- Build alongside existing CRC file; populate `crc_index` on first run.
- Update `packages` on successful builds; no backfill required.

## Validation
- Unit tests for CRUD and CRC path.
- Integration: run one build, verify records and CRC updates.

## Exit Criteria
- `NeedsBuild` returns false when CRC unchanged; true otherwise.
- Successful build writes `builds`, updates `packages` and `crc_index`.

## Dependencies
- Phase 1 (`pkg` provides stable `PortDir` and `Version`).
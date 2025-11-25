# Phase 3: Builder Orchestration

## Goals
- Implement worker pool to execute essential build phases in order.
- Integrate with `pkg` (build order) and `builddb` (records, CRC).

## MVP Phases
1. fetch
2. checksum
3. extract
4. patch
5. build
6. stage
7. package

## Concurrency Model
- N workers; channel-based queue of packages in topo order.
- A package enters run only when deps are successful (by topo order).

## Error Handling
- On phase failure: mark `failed`, stop dependent packages (skip).
- Graceful cleanup via `defer`/cleanup hooks.

## Stats (MVP)
- Track totals: success, failed, skipped; duration.

## Interfaces (Proposed)
```go
type BuildStats struct { Total, Success, Failed, Skipped int; Duration time.Duration }

type Builder struct {
    Env      environment.Environment
    DB       *builddb.DB
    Workers  int
}

func (b *Builder) Run(pkgs []*pkg.Package) (*BuildStats, error)
```

## Tasks
- Implement queue + worker lifecycle (start, run, stop).
- Wire per-phase execution via `Env.Execute(port, phase)`.
- Persist `running/success/failed` to `builds` and update CRC on success.
- Collate stats and return.

## Exit Criteria
- Builds a small set of ports; correct stats; CRC skip works.

## Dependencies
- Phases 1–2 complete.
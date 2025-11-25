# Phase 1: Library Extraction (pkg)

## Goals
- Isolate package metadata and dependency resolution into a pure library (`pkg`).
- Provide a small, stable API for parsing port specs and generating build order.
- Remove mixed concerns (CRC/status/build flags) from `pkg` where possible.

## Scope (MVP)
- Parse port specs (supports flavor syntax `origin@flavor`).
- Build dependency graph and compute topological order.
- Expose minimal `Package` struct and functions.

## Non-Goals (Deferred)
- Persistent package registry, advanced metadata caching.
- Deep validation of port Makefiles beyond what MVP needs.

## Public API (Proposed)
```go
type Package struct {
    PortDir  string
    Name     string
    Version  string
    Flavor   string
    PkgFile  string
    Deps     []*Package
    Next     *Package
}

func Parse(portSpecs []string, cfg *config.Config) (*Package, error)
func Resolve(head *Package, cfg *config.Config) error
func TopoOrder(head *Package) []*Package
```

## Current API Status

Implemented Wrappers:
- Parse -> ParsePortList
- Resolve -> ResolveDependencies
- TopoOrder -> GetBuildOrder
- TopoOrderStrict -> GetBuildOrder + cycle length check

Cycle Detection:
- TopoOrderStrict returns error if ordered count < linked list count.

Testing Coverage:
- topo order happy path (TopoOrder)
- cycle detection (TopoOrderStrict)
- dependency parsing (basic, flavor, skip nonexistent)
- empty parse specs error path

Next Enhancements:
- Introduce structured errors (ErrCycleDetected, ErrInvalidSpec)
- Separate pure metadata from build/CRC fields in Package (Phase 1 exit or early Phase 2)
- Add docs usage snippet in README once API stabilized.

## Tasks
- Identify and extract mixed responsibilities from current `pkg`.
- Implement `Parse` for port specification → initial linked list.
- Implement `Resolve` to populate `Deps` recursively.
- Implement `TopoOrder` (Kahn’s algorithm) for build order.
- Add input validation and minimal error types.
- Write unit tests for Parse/Resolve/TopoOrder.
- Update callers in CLI/builder to use new API.

## Deliverables
- Compilable `pkg` library with docstrings.
- Unit tests (happy paths + common edge cases).
- Minimal developer guide on using `pkg`.

## Exit Criteria
- Given a set of ports, `TopoOrder` returns a correct, cycle-free order.
- All existing commands that depend on dependency resolution compile and run with new API.

## Dependencies
- None (foundation for later phases).

## Risks & Mitigations
- Risk: Cycles in ports graph → Mitigation: detect and return descriptive error.
- Risk: Flavors parsing ambiguity → Mitigation: explicit parser + tests.
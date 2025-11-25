# Phase 6: Testing Strategy (MVP)

## Goals
- Ensure reliability of core libraries and minimal integration flow.

## Unit Tests
- `pkg`: Parse, Resolve, TopoOrder (cycles detection).
- `builddb`: CRUD, NeedsBuild/UpdateCRC.
- `builder`: Single worker happy path; failure propagation to dependents.

## Integration Tests
- Start a build for 1–3 small ports; poll until completion; assert records and stats.

## Fixtures
- Minimal test ports or mocked command execution.

## CI
- Run `go test -v -race ./...` on PRs.

## Out of Scope (MVP)
- Benchmarks, chaos tests, full E2E matrix.
# Phase 4: Environment Abstraction (DragonFly/FreeBSD)

## Goals
- Define a minimal environment interface and implement a single backend (DragonFly/FreeBSD).
- Provide phase execution with isolation primitives already used by dsynth.

## Interface (Proposed)
```go
type Environment interface {
    Setup(workerID int, cfg *config.Config) error
    Execute(port *pkg.Package, phase string) error
    Cleanup() error
}
```

## Implementation Notes (DF/FreeBSD)
- Use existing nullfs/tmpfs + chroot conventions from dsynth.
- Map ports tree under a fixed path (e.g., `/xports`).
- Ensure required tools are in PATH inside environment.

## Security & Privileges
- Requires root for mounts/chroot; validate early and fail fast.
- Clean up mounts even on failure (trap signals).

## Tasks
- Implement `Setup` to prepare work dirs and mounts.
- Implement `Execute` to run `make -C /xports/<cat>/<name> <phase>` (+ FLAVOR if any).
- Implement `Cleanup` to unmount and remove temp dirs.
- Add unit tests with mocks; integration test on a tiny port.

## Exit Criteria
- Each phase runs in isolation and returns success/failure.

## Dependencies
- Phase 3 (builder integration).
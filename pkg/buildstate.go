package pkg

import (
	"sync"
)

// BuildState tracks build-time state for a package.
// This is separate from Package to keep Package as pure metadata.
type BuildState struct {
	Pkg          *Package // Reference to the package
	Flags        int      // Build status flags (PkgF* constants)
	IgnoreReason string   // Reason package was ignored (from IGNORE in Makefile)
	LastPhase    string   // Last build phase that executed
}

// BuildStateRegistry maintains a mapping from Package to BuildState.
// This allows the pkg package to remain pure metadata while build-time
// state is tracked separately.
type BuildStateRegistry struct {
	mu     sync.RWMutex
	states map[*Package]*BuildState
}

// NewBuildStateRegistry creates a new empty registry.
func NewBuildStateRegistry() *BuildStateRegistry {
	return &BuildStateRegistry{
		states: make(map[*Package]*BuildState),
	}
}

// Get retrieves the BuildState for a package, creating it if it doesn't exist.
func (r *BuildStateRegistry) Get(pkg *Package) *BuildState {
	r.mu.RLock()
	state, ok := r.states[pkg]
	r.mu.RUnlock()

	if ok {
		return state
	}

	// Create new state if it doesn't exist
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if state, ok := r.states[pkg]; ok {
		return state
	}

	state = &BuildState{
		Pkg: pkg,
	}
	r.states[pkg] = state
	return state
}

// Set explicitly sets the BuildState for a package.
func (r *BuildStateRegistry) Set(pkg *Package, state *BuildState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states[pkg] = state
}

// Has checks if a package has build state registered.
func (r *BuildStateRegistry) Has(pkg *Package) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.states[pkg]
	return ok
}

// GetFlags is a convenience method to get just the flags.
func (r *BuildStateRegistry) GetFlags(pkg *Package) int {
	return r.Get(pkg).Flags
}

// SetFlags is a convenience method to set flags.
func (r *BuildStateRegistry) SetFlags(pkg *Package, flags int) {
	state := r.Get(pkg)
	state.Flags = flags
}

// AddFlags sets one or more flag bits.
func (r *BuildStateRegistry) AddFlags(pkg *Package, flags int) {
	state := r.Get(pkg)
	state.Flags |= flags
}

// ClearFlags clears one or more flag bits.
func (r *BuildStateRegistry) ClearFlags(pkg *Package, flags int) {
	state := r.Get(pkg)
	state.Flags &^= flags
}

// HasFlags checks if all specified flags are set.
func (r *BuildStateRegistry) HasFlags(pkg *Package, flags int) bool {
	state := r.Get(pkg)
	return (state.Flags & flags) == flags
}

// HasAnyFlags checks if any of the specified flags are set.
func (r *BuildStateRegistry) HasAnyFlags(pkg *Package, flags int) bool {
	state := r.Get(pkg)
	return (state.Flags & flags) != 0
}

// SetIgnoreReason sets the ignore reason for a package.
func (r *BuildStateRegistry) SetIgnoreReason(pkg *Package, reason string) {
	state := r.Get(pkg)
	state.IgnoreReason = reason
}

// GetIgnoreReason gets the ignore reason for a package.
func (r *BuildStateRegistry) GetIgnoreReason(pkg *Package) string {
	return r.Get(pkg).IgnoreReason
}

// SetLastPhase sets the last build phase for a package.
func (r *BuildStateRegistry) SetLastPhase(pkg *Package, phase string) {
	state := r.Get(pkg)
	state.LastPhase = phase
}

// GetLastPhase gets the last build phase for a package.
func (r *BuildStateRegistry) GetLastPhase(pkg *Package) string {
	return r.Get(pkg).LastPhase
}

// Count returns the number of packages tracked in the registry.
func (r *BuildStateRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.states)
}

// Clear removes all build state from the registry.
func (r *BuildStateRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states = make(map[*Package]*BuildState)
}

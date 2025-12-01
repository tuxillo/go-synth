package pkg

import (
	"sort"
	"strings"

	"dsynth/config"
)

// resolveDependencies builds the complete dependency graph
func resolveDependencies(packages []*Package, cfg *config.Config, registry *BuildStateRegistry, pkgRegistry *PackageRegistry, logger interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
}) error {
	// Phase 1: Collect all dependencies recursively
	logger.Info("Resolving dependencies...")

	bq := newBulkQueue(cfg, cfg.MaxWorkers)
	defer bq.Close()

	// Track what we've already queued to avoid duplicates
	queued := make(map[string]bool)

	// Process initial package list
	toProcess := make([]*Package, 0)
	for _, pkg := range packages {
		toProcess = append(toProcess, pkg)
		queued[pkg.PortDir] = true
	}

	processedCount := 0
	totalQueued := len(toProcess)

	// Track which packages we've added to the slice
	inSlice := make(map[string]bool)
	for _, pkg := range packages {
		inSlice[pkg.PortDir] = true
	}

	// Iteratively resolve dependencies
	for len(toProcess) > 0 {
		currentBatch := toProcess
		toProcess = make([]*Package, 0)

		for _, pkg := range currentBatch {
			// Parse and queue all dependency types
			deps := []struct {
				depStr  string
				depType DepType
			}{
				{pkg.FetchDeps, DepTypeFetch},
				{pkg.ExtractDeps, DepTypeExtract},
				{pkg.PatchDeps, DepTypePatch},
				{pkg.BuildDeps, DepTypeBuild},
				{pkg.LibDeps, DepTypeLib},
				{pkg.RunDeps, DepTypeRun},
			}

			for _, d := range deps {
				if d.depStr == "" {
					continue
				}

				depOrigins := parseDependencyString(d.depStr, cfg)
				for _, origin := range depOrigins {
					if queued[origin.portDir] {
						continue
					}

					// Check if already in registry
					if existing := pkgRegistry.Find(origin.portDir); existing != nil {
						// Already in registry, add to slice if not there
						if !inSlice[origin.portDir] {
							packages = append(packages, existing)
							inSlice[origin.portDir] = true
						}
						continue
					}

					// Queue for fetching
					bq.Queue(origin.category, origin.name, origin.flavor, "x")
					queued[origin.portDir] = true
					totalQueued++
				}
			}

			processedCount++
			if processedCount%10 == 0 {
				logger.Info("  Processed %d/%d packages...", processedCount, len(currentBatch))
			}
		}

		// Collect results from this batch
		for bq.Pending() > 0 {
			pkg, initialFlags, parseFlags, ignoreReason, err := bq.GetResult()
			if err != nil {
				logger.Warn("Dependency resolution failed: %v", err)
				continue
			}

			// Store all flags in registry
			allFlags := initialFlags | parseFlags
			if allFlags != 0 {
				registry.AddFlags(pkg, allFlags)
			}
			if ignoreReason != "" {
				registry.SetIgnoreReason(pkg, ignoreReason)
			}

			// Add to registry
			existingPkg := pkgRegistry.Enter(pkg)

			// Add to slice if it's a new package
			if existingPkg == pkg {
				// New package, add to slice
				packages = append(packages, pkg)
				inSlice[pkg.PortDir] = true
				toProcess = append(toProcess, pkg)
			} else {
				// Existing package, add to slice if not there
				if !inSlice[existingPkg.PortDir] {
					packages = append(packages, existingPkg)
					inSlice[existingPkg.PortDir] = true
				}
				toProcess = append(toProcess, existingPkg)
			}
		}
	}

	logger.Info("  Resolved %d total packages", totalQueued)

	// Phase 2: Build the dependency graph
	logger.Info("Building dependency graph...")
	if err := buildDependencyGraph(packages, cfg, pkgRegistry, logger); err != nil {
		return err
	}

	// Mark ports-mgmt/pkg for special bootstrap handling
	markPkgPkgFlag(packages, registry)

	return nil
}

// markPkgPkgFlag marks ports-mgmt/pkg with the PkgFPkgPkg flag for special
// bootstrap handling. This port must be built first before the worker pool
// starts, since it's required to create .pkg files for all other packages.
func markPkgPkgFlag(packages []*Package, registry *BuildStateRegistry) {
	for _, pkg := range packages {
		if pkg.PortDir == "ports-mgmt/pkg" {
			registry.AddFlags(pkg, PkgFPkgPkg)
			return
		}
	}
}

type depOrigin struct {
	category string
	name     string
	flavor   string
	portDir  string
}

// parseDependencyString parses a dependency string from a Makefile
// Format examples:
//
//	"tool:path/to/port"
//	"/path/to/ports/category/port:category/port"
//	"lib.so:category/port"
func parseDependencyString(depStr string, cfg *config.Config) []depOrigin {
	if depStr == "" {
		return nil
	}

	var origins []depOrigin
	deps := strings.Fields(depStr)

	for _, dep := range deps {
		// Skip ${NONEXISTENT} dependencies
		if strings.HasPrefix(dep, "/nonexistent:") {
			continue
		}

		// Find the colon separator
		colonIdx := strings.Index(dep, ":")
		if colonIdx < 0 {
			continue
		}

		// Extract the port origin (after the colon)
		origin := dep[colonIdx+1:]

		// Strip DPortsPath prefix if present
		if strings.HasPrefix(origin, cfg.DPortsPath) {
			origin = strings.TrimPrefix(origin, cfg.DPortsPath)
			origin = strings.TrimPrefix(origin, "/")
		}

		// Strip any trailing :tag
		if tagIdx := strings.LastIndex(origin, ":"); tagIdx > 0 {
			origin = origin[:tagIdx]
		}

		// Parse category/name[@flavor]
		parts := strings.Split(origin, "/")
		if len(parts) != 2 {
			continue
		}

		category := parts[0]
		nameAndFlavor := strings.SplitN(parts[1], "@", 2)
		name := nameAndFlavor[0]
		flavor := ""
		if len(nameAndFlavor) == 2 {
			flavor = nameAndFlavor[1]
		}

		portDir := category + "/" + name
		if flavor != "" {
			portDir += "@" + flavor
		}

		origins = append(origins, depOrigin{
			category: category,
			name:     name,
			flavor:   flavor,
			portDir:  portDir,
		})
	}

	return origins
}

// buildDependencyGraph creates the IDependOn and DependsOnMe links
func buildDependencyGraph(packages []*Package, cfg *config.Config, pkgRegistry *PackageRegistry, logger interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
}) error {
	// Process all packages
	count := 0
	for _, pkg := range packages {
		if err := linkPackageDependencies(pkg, cfg, pkgRegistry, logger); err != nil {
			return err
		}
		count++
		if count%50 == 0 {
			logger.Info("  Linked %d packages...", count)
		}
	}
	logger.Info("  Linked %d packages", count)

	// Calculate recursive dependency counts
	logger.Info("Calculating dependency depths...")
	for _, pkg := range packages {
		calculateDepthRecursive(pkg)
	}

	return nil
}

func linkPackageDependencies(pkg *Package, cfg *config.Config, pkgRegistry *PackageRegistry, logger interface {
	Warn(format string, args ...any)
}) error {
	deps := []struct {
		depStr  string
		depType DepType
	}{
		{pkg.FetchDeps, DepTypeFetch},
		{pkg.ExtractDeps, DepTypeExtract},
		{pkg.PatchDeps, DepTypePatch},
		{pkg.BuildDeps, DepTypeBuild},
		{pkg.LibDeps, DepTypeLib},
		{pkg.RunDeps, DepTypeRun},
	}

	for _, d := range deps {
		if d.depStr == "" {
			continue
		}

		depOrigins := parseDependencyString(d.depStr, cfg)
		for _, origin := range depOrigins {
			depPkg := pkgRegistry.Find(origin.portDir)
			if depPkg == nil {
				// This shouldn't happen if dependency resolution worked correctly
				logger.Warn("Dependency not found: %s (required by %s)",
					origin.portDir, pkg.PortDir)
				continue
			}

			// Create bidirectional link
			// pkg depends on depPkg
			pkg.IDependOn = append(pkg.IDependOn, &PkgLink{
				Pkg:     depPkg,
				DepType: d.depType,
			})

			// depPkg is depended on by pkg
			depPkg.DependsOnMe = append(depPkg.DependsOnMe, &PkgLink{
				Pkg:     pkg,
				DepType: d.depType,
			})

			depPkg.DepiCount++
		}
	}

	return nil
}

// calculateDepthRecursive calculates the maximum dependency depth
func calculateDepthRecursive(pkg *Package) int {
	if pkg.DepiDepth > 0 {
		return pkg.DepiDepth // Already calculated
	}

	maxDepth := 0
	for _, link := range pkg.DependsOnMe {
		depth := calculateDepthRecursive(link.Pkg)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	pkg.DepiDepth = maxDepth + 1
	return pkg.DepiDepth
}

// sortQueueByPriority sorts packages in the queue to optimize build order.
// Packages are prioritized by:
//  1. Number of dependents (higher = more packages unlocked when built)
//  2. DepiDepth (higher = more critical, deeper in dependency tree)
//  3. PortDir (lexicographic for determinism)
//
// This ensures that building high-fanout packages early maximizes parallelism
// potential and provides faster feedback for typical workflows.
func sortQueueByPriority(queue []*Package) {
	sort.Slice(queue, func(i, j int) bool {
		pi, pj := queue[i], queue[j]

		// Primary: Number of dependents (higher fanout = more packages unlocked)
		iDeps := len(pi.DependsOnMe)
		jDeps := len(pj.DependsOnMe)
		if iDeps != jDeps {
			return iDeps > jDeps
		}

		// Secondary: DepiDepth (higher depth = more critical)
		if pi.DepiDepth != pj.DepiDepth {
			return pi.DepiDepth > pj.DepiDepth
		}

		// Tertiary: PortDir (deterministic tie-breaker)
		return pi.PortDir < pj.PortDir
	})
}

// GetBuildOrder computes a topological ordering of packages using Kahn's
// algorithm, ensuring dependencies are built before packages that depend
// on them.
//
// # Algorithm
//
// Kahn's algorithm works by:
//  1. Computing in-degree (number of dependencies) for each package
//  2. Starting with packages that have zero dependencies (in-degree = 0)
//  3. Processing packages in order, removing edges and adding newly
//     zero-dependency packages to the queue
//  4. Continuing until all packages are processed or a cycle is detected
//
// # Priority Ordering
//
// When multiple packages have the same in-degree (i.e., are ready to build
// simultaneously), they are prioritized by:
//  1. Number of dependents (descending) - high-fanout packages first
//  2. DepiDepth (descending) - packages with deeper dependency trees first
//  3. PortDir (ascending) - deterministic tie-breaker
//
// This optimization ensures that packages with many dependents (like devel/pkgconf)
// are built as early as possible, maximizing parallelism in the build system.
//
// If the dependency graph contains cycles, some packages will remain unordered
// and a warning is printed. The function returns successfully with a partial
// ordering containing only the packages that could be ordered. Use
// TopoOrderStrict() if you need to detect and handle cycles as errors.
//
// # Parameters
//
//   - packages: slice of packages with resolved dependencies (IDependOn and
//     DependsOnMe fields populated)
//
// IMPORTANT: After calling ResolveDependencies(), you must pass ALL packages
// from pkgRegistry.AllPackages(), not just the root packages you initially
// requested. The complete dependency graph is stored in the registry.
//
// # Returns
//
// A slice of packages in build order (dependencies before dependents). If
// cycles exist, only non-cyclic packages are included. The function never
// returns an error; use TopoOrderStrict() for strict cycle checking.
//
// # Example
//
//	// After ResolveDependencies(), get all packages from registry
//	allPackages := pkgRegistry.AllPackages()
//	buildOrder := pkg.GetBuildOrder(allPackages)
//	for _, p := range buildOrder {
//	    fmt.Printf("Build: %s\n", p.PortDir)
//	}
func GetBuildOrder(packages []*Package, logger interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Debug(format string, args ...any)
}) []*Package {
	// Kahn's algorithm for topological sort
	inDegree := make(map[*Package]int)

	// Count in-degrees
	for _, pkg := range packages {
		inDegree[pkg] = len(pkg.IDependOn)
	}

	logger.Debug("GetBuildOrder: Total packages: %d", len(packages))

	// Debug: show in-degrees
	for pkg, degree := range inDegree {
		logger.Debug("%s in-degree=%d (depends on %d, depended by %d)",
			pkg.PortDir, degree, len(pkg.IDependOn), len(pkg.DependsOnMe))
	}

	// Queue packages with no dependencies
	queue := make([]*Package, 0)
	for _, pkg := range packages {
		if inDegree[pkg] == 0 {
			queue = append(queue, pkg)
			logger.Debug("Starting with zero in-degree: %s", pkg.PortDir)
		}
	}

	// Sort initial queue by priority (high-fanout packages first)
	sortQueueByPriority(queue)

	logger.Debug("Queue start size: %d", len(queue))

	// Process queue
	result := make([]*Package, 0, len(packages))
	for len(queue) > 0 {
		// Pop from queue
		pkg := queue[0]
		queue = queue[1:]
		result = append(result, pkg)

		logger.Debug("Processing %s, queue size now %d", pkg.PortDir, len(queue))

		// Reduce in-degree for dependents
		newlyReady := make([]*Package, 0)
		for _, link := range pkg.DependsOnMe {
			dep := link.Pkg
			inDegree[dep]--
			logger.Debug("  Reduced %s to in-degree=%d", dep.PortDir, inDegree[dep])
			if inDegree[dep] == 0 {
				newlyReady = append(newlyReady, dep)
				logger.Debug("  Added %s to queue", dep.PortDir)
			}
		}

		// Sort newly ready packages by priority before adding to queue
		if len(newlyReady) > 0 {
			sortQueueByPriority(newlyReady)
			queue = append(queue, newlyReady...)
		}
	}

	// Check for cycles
	if len(result) != len(packages) {
		logger.Warn("Circular dependencies detected (%d/%d packages in order)",
			len(result), len(packages))

		// Debug: show which packages are stuck
		logger.Debug("Packages not in result:")
		for pkg, degree := range inDegree {
			if degree > 0 {
				logger.Debug("  %s: in-degree=%d", pkg.PortDir, degree)
			}
		}
	}

	logger.Debug("Final result: %d packages", len(result))

	return result
}

// TopoOrderStrict is like GetBuildOrder but returns an error if circular
// dependencies are detected. Use this when you need to guarantee a complete,
// valid build order or fail explicitly.
//
// The function calls GetBuildOrder internally and checks if all packages were
// successfully ordered. If cycles exist, returns *CycleError with details
// about the remaining packages that couldn't be ordered.
//
// # Error Inspection
//
// The error can be inspected with errors.As() to access cycle information:
//
//	order, err := pkg.TopoOrderStrict(packages)
//	if err != nil {
//	    var cycleErr *pkg.CycleError
//	    if errors.As(err, &cycleErr) {
//	        fmt.Printf("Found %d packages in cycles\n", cycleErr.NumPackages())
//	        fmt.Printf("Successfully ordered: %d/%d\n",
//	            cycleErr.OrderedPackages, cycleErr.TotalPackages)
//	    }
//	    return err
//	}
//
// # Parameters
//
//   - packages: slice of packages with resolved dependencies
//
// IMPORTANT: After calling ResolveDependencies(), you must pass ALL packages
// from pkgRegistry.AllPackages(), not just the root packages you initially
// requested. The complete dependency graph is stored in the registry.
//
// # Returns
//
//   - slice of packages in build order (may be partial if cycles exist)
//   - *CycleError if cycles detected, wrapping ErrCycleDetected
//   - nil error if all packages were successfully ordered
//
// # Example
//
//	// After ResolveDependencies(), get all packages from registry
//	allPackages := pkgRegistry.AllPackages()
//	buildOrder, err := pkg.TopoOrderStrict(allPackages)
//	if err != nil {
//	    log.Fatalf("Cannot build: %v", err)
//	}
func TopoOrderStrict(packages []*Package, logger interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Debug(format string, args ...any)
}) ([]*Package, error) {
	order := GetBuildOrder(packages, logger)
	if len(order) != len(packages) {
		return order, &CycleError{
			TotalPackages:   len(packages),
			OrderedPackages: len(order),
			CyclePackages:   nil, // Could be enhanced to track specific cycle packages
		}
	}
	return order, nil
}

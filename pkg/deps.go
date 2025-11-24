package pkg

import (
	"fmt"
	"strings"

	"dsynth/config"
)

// resolveDependencies builds the complete dependency graph
func resolveDependencies(head *Package, cfg *config.Config) error {
	// Phase 1: Collect all dependencies recursively
	fmt.Println("Resolving dependencies...")

	bq := newBulkQueue(cfg, cfg.MaxWorkers)
	defer bq.Close()

	// Track what we've already queued to avoid duplicates
	queued := make(map[string]bool)

	// Process initial package list
	toProcess := make([]*Package, 0)
	for pkg := head; pkg != nil; pkg = pkg.Next {
		toProcess = append(toProcess, pkg)
		queued[pkg.PortDir] = true
	}

	processedCount := 0
	totalQueued := len(toProcess)

	// Iteratively resolve dependencies
	for len(toProcess) > 0 {
		currentBatch := toProcess
		toProcess = make([]*Package, 0)

		for _, pkg := range currentBatch {
			// Parse and queue all dependency types
			deps := []struct {
				depStr  string
				depType int
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
					if existing := globalRegistry.Find(origin.portDir); existing != nil {
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
				fmt.Printf("  Processed %d/%d packages...\r", processedCount, len(currentBatch))
			}
		}

		// Collect results from this batch
		for bq.Pending() > 0 {
			pkg, err := bq.GetResult()
			if err != nil {
				fmt.Printf("Warning: dependency resolution failed: %v\n", err)
				continue
			}

			globalRegistry.Enter(pkg)
			toProcess = append(toProcess, pkg)
		}
	}

	fmt.Printf("  Resolved %d total packages\n", totalQueued)

	// Phase 2: Build the dependency graph
	fmt.Println("Building dependency graph...")
	return buildDependencyGraph(head, cfg)
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
func buildDependencyGraph(head *Package, cfg *config.Config) error {
	// Process all packages
	count := 0
	for pkg := head; pkg != nil; pkg = pkg.Next {
		if err := linkPackageDependencies(pkg, cfg); err != nil {
			return err
		}
		count++
		if count%50 == 0 {
			fmt.Printf("  Linked %d packages...\r", count)
		}
	}
	fmt.Printf("  Linked %d packages\n", count)

	// Calculate recursive dependency counts
	fmt.Println("Calculating dependency depths...")
	for pkg := head; pkg != nil; pkg = pkg.Next {
		calculateDepthRecursive(pkg)
	}

	return nil
}

func linkPackageDependencies(pkg *Package, cfg *config.Config) error {
	deps := []struct {
		depStr  string
		depType int
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
			depPkg := globalRegistry.Find(origin.portDir)
			if depPkg == nil {
				// This shouldn't happen if dependency resolution worked correctly
				fmt.Printf("Warning: dependency not found: %s (required by %s)\n",
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

// GetBuildOrder returns packages in topological order (dependencies first)
func GetBuildOrder(head *Package) []*Package {
	// Kahn's algorithm for topological sort
	inDegree := make(map[*Package]int)
	packages := make([]*Package, 0)

	// Count in-degrees and collect all packages
	for pkg := head; pkg != nil; pkg = pkg.Next {
		packages = append(packages, pkg)
		inDegree[pkg] = len(pkg.IDependOn)
	}

	// Queue packages with no dependencies
	queue := make([]*Package, 0)
	for _, pkg := range packages {
		if inDegree[pkg] == 0 {
			queue = append(queue, pkg)
		}
	}

	// Process queue
	result := make([]*Package, 0, len(packages))
	for len(queue) > 0 {
		// Pop from queue
		pkg := queue[0]
		queue = queue[1:]
		result = append(result, pkg)

		// Reduce in-degree for dependents
		for _, link := range pkg.DependsOnMe {
			dep := link.Pkg
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	// Check for cycles
	if len(result) != len(packages) {
		fmt.Printf("Warning: circular dependencies detected (%d/%d packages in order)\n",
			len(result), len(packages))
	}

	return result
}

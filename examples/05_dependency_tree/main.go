// Example 05: Dependency Tree Visualization
//
// This example prints a visual tree representation of a package's dependencies,
// showing the hierarchical dependency structure.
//
// Usage:
//   go run main.go [port-spec] [max-depth]
//
// Examples:
//   go run main.go editors/vim
//   go run main.go editors/vim 3

package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"go-synth/config"
	"go-synth/pkg"
	dslog "go-synth/log"
)

func main() {
	// Get port spec and max depth from command line
	portSpec := "editors/vim"
	maxDepth := 5 // Default max depth
	if len(os.Args) > 1 {
		portSpec = os.Args[1]
	}
	if len(os.Args) > 2 {
		depth, err := strconv.Atoi(os.Args[2])
		if err == nil && depth > 0 {
			maxDepth = depth
		}
	}

	fmt.Printf("Dependency tree for: %s (max depth: %d)\n\n", portSpec, maxDepth)

	// 1. Load configuration
	cfg, err := config.LoadConfig("", "default")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Create registries
	pkgRegistry := pkg.NewPackageRegistry()
	bsRegistry := pkg.NewBuildStateRegistry()

	// 3. Parse port
	packages, err := pkg.ParsePortList([]string{portSpec}, cfg, bsRegistry, pkgRegistry, dslog.StdoutLogger{})
	if err != nil {
		log.Fatalf("Failed to parse port: %v", err)
	}

	// 4. Resolve dependencies
	err = pkg.ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry, dslog.StdoutLogger{})
	if err != nil {
		log.Fatalf("Failed to resolve dependencies: %v", err)
	}

	// 5. Print dependency tree
	rootPkg := packages[0]
	visited := make(map[*pkg.Package]bool)

	fmt.Printf("📦 %s (v%s)\n", rootPkg.PortDir, rootPkg.Version)
	printDependencyTree(rootPkg, "", true, visited, maxDepth, 0)

	// 6. Print statistics
	fmt.Println()
	fmt.Printf("Statistics:\n")
	fmt.Printf("  Unique dependencies shown: %d\n", len(visited)-1) // -1 for root
	totalDeps := countTotalDeps(rootPkg)
	fmt.Printf("  Total unique dependencies: %d\n", totalDeps)
	if len(visited)-1 < totalDeps {
		fmt.Printf("  (Some dependencies not shown due to max depth limit)\n")
	}
}

func printDependencyTree(p *pkg.Package, prefix string, isLast bool, visited map[*pkg.Package]bool, maxDepth int, currentDepth int) {
	// Check depth limit
	if currentDepth >= maxDepth {
		return
	}

	// Mark as visited
	if visited[p] && currentDepth > 0 {
		// Already shown, just indicate
		return
	}
	visited[p] = true

	// Get all dependencies
	deps := p.IDependOn
	if len(deps) == 0 {
		return
	}

	// Print each dependency
	for i, link := range deps {
		isLastDep := (i == len(deps)-1)

		// Build prefix for this line
		var connector string
		if isLastDep {
			connector = "└─"
		} else {
			connector = "├─"
		}

		// Print dependency type and package
		depType := link.DepType.String()
		fmt.Printf("%s%s [%s] %s", prefix, connector, depType, link.Pkg.PortDir)

		// Check if already visited (circular or duplicate)
		if visited[link.Pkg] {
			fmt.Printf(" (already shown)\n")
			continue
		}
		fmt.Printf("\n")

		// Build prefix for children
		var childPrefix string
		if isLastDep {
			childPrefix = prefix + "   "
		} else {
			childPrefix = prefix + "│  "
		}

		// Recursively print children
		printDependencyTree(link.Pkg, childPrefix, isLastDep, visited, maxDepth, currentDepth+1)
	}
}

func countTotalDeps(p *pkg.Package) int {
	visited := make(map[*pkg.Package]bool)
	countRecursive(p, visited)
	return len(visited) - 1 // -1 for root package itself
}

func countRecursive(p *pkg.Package, visited map[*pkg.Package]bool) {
	if visited[p] {
		return
	}
	visited[p] = true
	for _, link := range p.IDependOn {
		countRecursive(link.Pkg, visited)
	}
}

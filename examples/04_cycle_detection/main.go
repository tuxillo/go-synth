// Example 04: Cycle Detection
//
// This example demonstrates how to detect circular dependencies using TopoOrderStrict.
// It shows the difference between permissive ordering (GetBuildOrder) and
// strict ordering (TopoOrderStrict).
//
// Usage:
//   go run main.go [port-spec]
//
// Example:
//   go run main.go editors/vim

package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"dsynth/config"
	"dsynth/pkg"
	dslog "dsynth/log"
)

func main() {
	// Get port spec from command line or use default
	portSpec := "editors/vim"
	if len(os.Args) > 1 {
		portSpec = os.Args[1]
	}

	fmt.Printf("Checking for dependency cycles in: %s\n\n", portSpec)

	// 1. Load configuration
	cfg, err := config.LoadConfig("", "default")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Create registries
	pkgRegistry := pkg.NewPackageRegistry()
	bsRegistry := pkg.NewBuildStateRegistry()

	// 3. Parse port
	fmt.Println("Parsing port...")
	packages, err := pkg.ParsePortList([]string{portSpec}, cfg, bsRegistry, pkgRegistry, dslog.StdoutLogger{})
	if err != nil {
		log.Fatalf("Failed to parse port: %v", err)
	}

	// 4. Resolve dependencies
	fmt.Println("Resolving dependencies...")
	err = pkg.ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry, dslog.StdoutLogger{})
	if err != nil {
		log.Fatalf("Failed to resolve dependencies: %v", err)
	}

	totalPackages := countTotalPackages(packages)
	fmt.Printf("Resolved %d total packages\n\n", totalPackages)

	// 5. Try strict ordering (detects cycles)
	fmt.Println("Attempting strict topological ordering (cycle detection)...")
	// Note: TopoOrderStrict needs ALL packages from the registry
	allPackages := pkgRegistry.AllPackages()
	strictOrder, err := pkg.TopoOrderStrict(allPackages, dslog.StdoutLogger{})

	if err != nil {
		// Check if it's a cycle error
		var cycleErr *pkg.CycleError
		if errors.As(err, &cycleErr) {
			fmt.Printf("\n❌ CYCLE DETECTED!\n\n")
			fmt.Printf("Details:\n")
			fmt.Printf("  Total packages:     %d\n", cycleErr.TotalPackages)
			fmt.Printf("  Successfully ordered: %d\n", cycleErr.OrderedPackages)
			fmt.Printf("  Stuck in cycle:     %d\n", cycleErr.TotalPackages-cycleErr.OrderedPackages)

			if len(cycleErr.CyclePackages) > 0 {
				fmt.Printf("\nPackages involved in cycle:\n")
				for _, p := range cycleErr.CyclePackages {
					fmt.Printf("  - %s\n", p.PortDir)
				}
			}

			fmt.Println("\nCycles prevent strict ordering, but you can use permissive ordering for builds.")
		} else {
			// Some other error
			log.Fatalf("Strict ordering failed: %v", err)
		}

		// 6. Fall back to permissive ordering
		fmt.Println("\nFalling back to permissive ordering (ignores cycles)...")
		permissiveOrder := pkg.GetBuildOrder(allPackages, dslog.StdoutLogger{})
		fmt.Printf("✓ Permissive ordering succeeded with %d packages\n", len(permissiveOrder))
		fmt.Println("\nNote: Permissive ordering works around cycles by breaking them arbitrarily.")

	} else {
		// No cycles!
		fmt.Printf("\n✓ SUCCESS! No cycles detected.\n\n")
		fmt.Printf("Strict ordering computed %d packages in dependency order.\n", len(strictOrder))
		fmt.Println("\nThis dependency graph is cycle-free and can be built in strict order.")
	}
}

func countTotalPackages(packages []*pkg.Package) int {
	visited := make(map[*pkg.Package]bool)
	for _, p := range packages {
		countRecursive(p, visited)
	}
	return len(visited)
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

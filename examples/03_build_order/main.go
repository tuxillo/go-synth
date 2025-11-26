// Example 03: Compute Build Order
//
// This example demonstrates the complete workflow: parse -> resolve -> compute build order.
// The build order ensures dependencies are built before packages that depend on them.
//
// Usage:
//   go run main.go [port-spec...]
//
// Examples:
//   go run main.go editors/vim
//   go run main.go editors/vim shells/bash devel/git

package main

import (
	"fmt"
	"log"
	"os"

	"dsynth/config"
	"dsynth/pkg"
)

func main() {
	// Get port specs from command line or use default
	portSpecs := []string{"editors/vim"}
	if len(os.Args) > 1 {
		portSpecs = os.Args[1:]
	}

	fmt.Printf("Computing build order for %d port(s):\n", len(portSpecs))
	for _, spec := range portSpecs {
		fmt.Printf("  - %s\n", spec)
	}
	fmt.Println()

	// 1. Load configuration
	cfg, err := config.LoadConfig("", "default")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Create registries
	pkgRegistry := pkg.NewPackageRegistry()
	bsRegistry := pkg.NewBuildStateRegistry()

	// 3. Parse ports
	fmt.Println("Parsing ports...")
	packages, err := pkg.ParsePortList(portSpecs, cfg, bsRegistry, pkgRegistry)
	if err != nil {
		log.Fatalf("Failed to parse ports: %v", err)
	}
	fmt.Printf("Parsed %d package(s)\n", len(packages))

	// 4. Resolve dependencies
	fmt.Println("\nResolving dependencies...")
	err = pkg.ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry)
	if err != nil {
		log.Fatalf("Failed to resolve dependencies: %v", err)
	}
	fmt.Println("Dependencies resolved")

	// 5. Compute build order
	fmt.Println("\nComputing topological build order...")
	// Note: GetBuildOrder needs ALL packages from the registry, not just the root package(s)
	allPackages := pkgRegistry.AllPackages()
	buildOrder := pkg.GetBuildOrder(allPackages)

	// 6. Display build order
	fmt.Printf("\nBuild Order (%d packages total):\n", len(buildOrder))
	fmt.Println("=" + repeatChar("=", 70))

	for i, p := range buildOrder {
		// Calculate some statistics
		directDeps := len(p.IDependOn)
		reverseDeps := len(p.DependsOnMe)

		// Format output
		fmt.Printf("%4d. %-40s [%d deps, %d dependents]\n",
			i+1, p.PortDir, directDeps, reverseDeps)

		// Print first few for reference
		if i == 0 {
			fmt.Println("      ^ Build this first (no dependencies)")
		} else if i == len(buildOrder)-1 {
			fmt.Println("      ^ Build this last (requested package)")
		}
	}

	fmt.Println("=" + repeatChar("=", 70))
	fmt.Printf("\nSuccess! Build order computed with %d packages.\n", len(buildOrder))
	fmt.Println("\nNote: Packages with no dependencies are built first,")
	fmt.Println("      and each package is built after all its dependencies.")
}

func repeatChar(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

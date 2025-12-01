// Example 02: Resolve Dependencies
//
// This example shows how to resolve all dependencies of a port,
// building a complete dependency graph.
//
// Usage:
//   go run main.go [port-spec]
//
// Example:
//   go run main.go editors/vim

package main

import (
	"fmt"
	"log"
	"os"

	"go-synth/config"
	"go-synth/pkg"
	dslog "go-synth/log"
)

func main() {
	// Get port spec from command line or use default
	portSpec := "editors/vim"
	if len(os.Args) > 1 {
		portSpec = os.Args[1]
	}

	fmt.Printf("Resolving dependencies for: %s\n\n", portSpec)

	// 1. Load configuration
	cfg, err := config.LoadConfig("", "default")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Create registries
	pkgRegistry := pkg.NewPackageRegistry()
	bsRegistry := pkg.NewBuildStateRegistry()

	// 3. Parse the port
	packages, err := pkg.ParsePortList([]string{portSpec}, cfg, bsRegistry, pkgRegistry, dslog.StdoutLogger{})
	if err != nil {
		log.Fatalf("Failed to parse port: %v", err)
	}

	fmt.Printf("Parsed: %s (v%s)\n", packages[0].PortDir, packages[0].Version)

	// 4. Resolve all dependencies (this is where the magic happens!)
	fmt.Println("\nResolving dependencies...")
	err = pkg.ResolveDependencies(packages, cfg, bsRegistry, pkgRegistry, dslog.StdoutLogger{})
	if err != nil {
		log.Fatalf("Failed to resolve dependencies: %v", err)
	}

	// 5. Count total dependencies
	totalDeps := countAllPackages(packages)
	fmt.Printf("Resolution complete! Found %d total packages (including dependencies)\n\n", totalDeps)

	// 6. Print dependency statistics
	p := packages[0]
	fmt.Printf("Direct Dependencies:\n")
	fmt.Printf("  BUILD dependencies: %d\n", len(p.IDependOn))
	fmt.Printf("  Reverse dependencies: %d packages depend on this\n\n", len(p.DependsOnMe))

	// 7. Show some dependencies
	if len(p.IDependOn) > 0 {
		fmt.Println("First 10 dependencies:")
		count := 0
		for _, link := range p.IDependOn {
			fmt.Printf("  %s (%s)\n", link.Pkg.PortDir, link.DepType.String())
			count++
			if count >= 10 {
				break
			}
		}
		if len(p.IDependOn) > 10 {
			fmt.Printf("  ... and %d more\n", len(p.IDependOn)-10)
		}
	}

	fmt.Printf("\nSuccess! Dependencies resolved.\n")
}

// countAllPackages counts unique packages in the dependency graph
func countAllPackages(packages []*pkg.Package) int {
	visited := make(map[*pkg.Package]bool)
	for _, p := range packages {
		countPackagesRecursive(p, visited)
	}
	return len(visited)
}

func countPackagesRecursive(p *pkg.Package, visited map[*pkg.Package]bool) {
	if visited[p] {
		return
	}
	visited[p] = true
	for _, link := range p.IDependOn {
		countPackagesRecursive(link.Pkg, visited)
	}
}

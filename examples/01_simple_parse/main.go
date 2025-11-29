// Example 01: Simple Parse
//
// This example demonstrates the most basic usage of the pkg library:
// parsing a single port specification and printing its metadata.
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

	"dsynth/config"
	dslog "dsynth/log"
	"dsynth/pkg"
)

func main() {
	// Get port spec from command line or use default
	portSpec := "editors/vim"
	if len(os.Args) > 1 {
		portSpec = os.Args[1]
	}

	fmt.Printf("Parsing port: %s\n\n", portSpec)

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

	// 4. Print package information
	p := packages[0]
	fmt.Printf("Package Information:\n")
	fmt.Printf("  Port Directory:  %s\n", p.PortDir)
	fmt.Printf("  Version:         %s\n", p.Version)
	fmt.Printf("  Package File:    %s\n", p.PkgFile)
	if p.Flavor != "" {
		fmt.Printf("  Flavor:          %s\n", p.Flavor)
	}

	fmt.Printf("\nRaw Dependency Strings (not resolved yet):\n")
	if p.BuildDeps != "" {
		fmt.Printf("  BUILD_DEPENDS:   %s\n", p.BuildDeps)
	}
	if p.RunDeps != "" {
		fmt.Printf("  RUN_DEPENDS:     %s\n", p.RunDeps)
	}
	if p.LibDeps != "" {
		fmt.Printf("  LIB_DEPENDS:     %s\n", p.LibDeps)
	}
	if p.FetchDeps != "" {
		fmt.Printf("  FETCH_DEPENDS:   %s\n", p.FetchDeps)
	}
	if p.ExtractDeps != "" {
		fmt.Printf("  EXTRACT_DEPENDS: %s\n", p.ExtractDeps)
	}

	fmt.Printf("\nSuccess! Parsed 1 package.\n")
}

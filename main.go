package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/rancher/rancher-assets/internal/config"
	"github.com/rancher/rancher-assets/internal/generator"
	"github.com/rancher/rancher-assets/internal/lockfile"
)

const (
	configPath     = "config.yaml"
	lockPath       = "lock.yaml"
	dockerfilesDir = "dockerfiles"
	copyScriptPath = "package/copy-charts.sh"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "generate":
		if err := generateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: rancher-assets <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  generate    Generate Dockerfiles and update lock.yaml")
}

func generateCommand() error {
	fmt.Println("Loading configuration...")

	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Load lock file
	lock, err := lockfile.Load(lockPath)
	if err != nil {
		return fmt.Errorf("failed to load lock file: %w", err)
	}

	// Get chart majors and sort for consistent output
	majors := cfg.ListChartMajors()
	sort.Strings(majors)

	fmt.Printf("Found %d chart major versions: %v\n", len(majors), majors)

	// Compute copy script hash for reproducibility
	fmt.Printf("\nComputing copy-charts.sh hash...\n")
	scriptHash, err := lockfile.ComputeFileHash(copyScriptPath)
	if err != nil {
		return fmt.Errorf("failed to compute copy script hash: %w", err)
	}
	fmt.Printf("  Script hash: %s\n", scriptHash[:16]+"...")
	lock.CopyScriptHash = scriptHash

	// Generate Dockerfiles and query upstream for each chart major
	for _, major := range majors {
		fmt.Printf("\nProcessing chart major: %s\n", major)

		// Ensure lock entry exists
		lock.EnsureChartVersion(major)

		// Query upstream repos first (before generation, so commits are available)
		fmt.Printf("  Querying upstream repositories...\n")
		chartCfg, _ := cfg.GetChartVersion(major)

		// Query both prod and dev branches
		var prodRefs, devRefs lockfile.UpstreamRefsSet

		// Query PROD branches
		fmt.Printf("    [prod]\n")
		fmt.Printf("      - charts @ %s: ", chartCfg.Prod.ChartsBranch)
		chartsRef, err := lockfile.QueryUpstreamRef(cfg.ClusterRepos["charts"].URL, chartCfg.Prod.ChartsBranch)
		if err != nil {
			fmt.Printf("FAILED (%v)\n", err)
			return fmt.Errorf("failed to query charts upstream: %w", err)
		}
		fmt.Printf("%s\n", chartsRef.Commit[:8])
		prodRefs.Charts = chartsRef

		fmt.Printf("      - partner @ %s: ", chartCfg.Prod.PartnerBranch)
		partnerRef, err := lockfile.QueryUpstreamRef(cfg.ClusterRepos["partner"].URL, chartCfg.Prod.PartnerBranch)
		if err != nil {
			fmt.Printf("FAILED (%v)\n", err)
			return fmt.Errorf("failed to query partner upstream: %w", err)
		}
		fmt.Printf("%s\n", partnerRef.Commit[:8])
		prodRefs.Partner = partnerRef

		fmt.Printf("      - rke2 @ %s: ", chartCfg.Prod.Rke2Branch)
		rke2Ref, err := lockfile.QueryUpstreamRef(cfg.ClusterRepos["rke2"].URL, chartCfg.Prod.Rke2Branch)
		if err != nil {
			fmt.Printf("FAILED (%v)\n", err)
			return fmt.Errorf("failed to query rke2 upstream: %w", err)
		}
		fmt.Printf("%s\n", rke2Ref.Commit[:8])
		prodRefs.Rke2 = rke2Ref

		// Query DEV branches
		fmt.Printf("    [dev]\n")
		fmt.Printf("      - charts @ %s: ", chartCfg.Dev.ChartsBranch)
		chartsRef, err = lockfile.QueryUpstreamRef(cfg.ClusterRepos["charts"].URL, chartCfg.Dev.ChartsBranch)
		if err != nil {
			fmt.Printf("FAILED (%v)\n", err)
			return fmt.Errorf("failed to query charts upstream: %w", err)
		}
		fmt.Printf("%s\n", chartsRef.Commit[:8])
		devRefs.Charts = chartsRef

		fmt.Printf("      - partner @ %s: ", chartCfg.Dev.PartnerBranch)
		partnerRef, err = lockfile.QueryUpstreamRef(cfg.ClusterRepos["partner"].URL, chartCfg.Dev.PartnerBranch)
		if err != nil {
			fmt.Printf("FAILED (%v)\n", err)
			return fmt.Errorf("failed to query partner upstream: %w", err)
		}
		fmt.Printf("%s\n", partnerRef.Commit[:8])
		devRefs.Partner = partnerRef

		fmt.Printf("      - rke2 @ %s: ", chartCfg.Dev.Rke2Branch)
		rke2Ref, err = lockfile.QueryUpstreamRef(cfg.ClusterRepos["rke2"].URL, chartCfg.Dev.Rke2Branch)
		if err != nil {
			fmt.Printf("FAILED (%v)\n", err)
			return fmt.Errorf("failed to query rke2 upstream: %w", err)
		}
		fmt.Printf("%s\n", rke2Ref.Commit[:8])
		devRefs.Rke2 = rke2Ref

		// Update lock file with both prod and dev refs
		if err := lock.UpdateUpstreamRefs(major, prodRefs, devRefs); err != nil {
			return fmt.Errorf("failed to update lock file: %w", err)
		}

		// Generate Dockerfile (after updating lock, so commits are available)
		fmt.Printf("  Generating Dockerfile.%s...\n", major)
		if err := generator.Generate(cfg, lock, major, dockerfilesDir); err != nil {
			return fmt.Errorf("failed to generate Dockerfile for %s: %w", major, err)
		}
	}

	// Save lock file
	fmt.Printf("\nSaving lock file...\n")
	if err := lock.Save(lockPath); err != nil {
		return fmt.Errorf("failed to save lock file: %w", err)
	}

	fmt.Println("\n✅ Generation complete!")
	fmt.Println("\nGenerated files:")
	for _, major := range majors {
		fmt.Printf("  - dockerfiles/Dockerfile.%s\n", major)
	}
	fmt.Printf("  - %s\n", lockPath)
	fmt.Println("\nReview changes with: git diff dockerfiles/ lock.yaml")

	return nil
}

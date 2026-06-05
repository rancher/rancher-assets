package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/rancher/rancher-assets/internal/config"
	"github.com/rancher/rancher-assets/internal/generator"
	"github.com/rancher/rancher-assets/internal/imagelist"
	"github.com/rancher/rancher-assets/internal/lockfile"
	"github.com/rancher/rancher-assets/internal/versions"
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
	case "changed-majors":
		if err := changedMajorsCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "plan-release":
		if err := planReleaseCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "export-images":
		if err := exportImagesCommand(); err != nil {
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
	fmt.Println("  generate             Generate Dockerfiles and update lock.yaml")
	fmt.Println("  changed-majors       Detect chart majors with upstream ref changes")
	fmt.Println("                       Flags: --from=<commit> --to=<commit>")
	fmt.Println("  plan-release         Plan version bumps for releases")
	fmt.Println("                       Flags: --versions-file=<path> --type=<auto|manual>")
	fmt.Println("                              --changed-majors=<json> (for auto)")
	fmt.Println("                              --majors=<json> --bump=<minor|patch> --release=<stable|prerelease> (for manual)")
	fmt.Println("  export-images        Generate image lists from chart catalogs")
	fmt.Println("                       Flags: --charts-path=<path> --version=<version> --output-dir=<path>")
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

func changedMajorsCommand() error {
	// Parse flags
	fs := flag.NewFlagSet("changed-majors", flag.ExitOnError)
	fromCommit := fs.String("from", "", "From commit (required)")
	toCommit := fs.String("to", "", "To commit (required)")
	verbose := fs.Bool("verbose", false, "Show detailed change information")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *fromCommit == "" || *toCommit == "" {
		return fmt.Errorf("both --from and --to are required")
	}

	// Get changed majors
	changed, err := lockfile.ChangedMajors(*fromCommit, *toCommit)
	if err != nil {
		return err
	}

	// Ensure we have an empty array instead of nil for JSON marshaling
	if changed == nil {
		changed = []string{}
	}

	// Sort for consistent output
	sort.Strings(changed)

	if *verbose {
		// Verbose output - show what changed
		if len(changed) == 0 {
			fmt.Println("No chart majors with upstream ref changes detected")
			fmt.Println("(Only timestamp changes in lock.yaml)")
		} else {
			fmt.Printf("Changed chart majors (%d):\n", len(changed))
			for _, major := range changed {
				fmt.Printf("  - %s\n", major)
			}
		}
	}

	// Always output JSON array (for workflow consumption)
	output, err := json.Marshal(changed)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

func planReleaseCommand() error {
	// Parse flags
	fs := flag.NewFlagSet("plan-release", flag.ExitOnError)
	versionsFile := fs.String("versions-file", "", "Path to versions.yaml (required)")
	planType := fs.String("type", "", "Plan type: auto or manual (required)")
	changedMajorsJSON := fs.String("changed-majors", "", "JSON array of changed majors (for auto)")
	majorsJSON := fs.String("majors", "", "JSON array of majors to release (for manual)")
	bumpType := fs.String("bump", "", "Bump type: minor or patch (for manual)")
	releaseType := fs.String("release", "", "Release type: stable or prerelease (for manual)")
	verbose := fs.Bool("verbose", false, "Show detailed output")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *versionsFile == "" || *planType == "" {
		return fmt.Errorf("--versions-file and --type are required")
	}

	// Load versions file
	vf, err := versions.LoadVersionsFile(*versionsFile)
	if err != nil {
		return err
	}

	var plans []versions.ReleasePlan

	switch *planType {
	case "auto":
		if *changedMajorsJSON == "" {
			return fmt.Errorf("--changed-majors is required for auto type")
		}

		var changedMajors []string
		if err := json.Unmarshal([]byte(*changedMajorsJSON), &changedMajors); err != nil {
			return fmt.Errorf("failed to parse changed-majors JSON: %w", err)
		}

		plans, err = versions.PlanAutoPrerelease(vf, changedMajors)
		if err != nil {
			return err
		}

	case "manual":
		if *majorsJSON == "" || *bumpType == "" || *releaseType == "" {
			return fmt.Errorf("--majors, --bump, and --release are required for manual type")
		}

		var majors []string
		if err := json.Unmarshal([]byte(*majorsJSON), &majors); err != nil {
			return fmt.Errorf("failed to parse majors JSON: %w", err)
		}

		plans, err = versions.PlanManualRelease(vf, majors, *bumpType, *releaseType)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("invalid type: %s (must be auto or manual)", *planType)
	}

	// Verbose output
	if *verbose {
		if len(plans) == 0 {
			fmt.Println("No releases planned")
		} else {
			fmt.Printf("Planned releases (%d):\n", len(plans))
			for _, plan := range plans {
				fmt.Printf("  %s: %s (stable=%s, prerelease=%s)\n",
					plan.Major, plan.NewVersion, plan.CurrentStable, plan.CurrentPrerelease)
			}
		}
	}

	// Always output JSON for workflow consumption
	output, err := json.Marshal(plans)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

func exportImagesCommand() error {
	// Parse flags
	fs := flag.NewFlagSet("export-images", flag.ExitOnError)
	chartsPath := fs.String("charts-path", "", "Path to extracted chart catalogs (required)")
	version := fs.String("version", "", "Chart image version being exported (required)")
	outputDir := fs.String("output-dir", "", "Output directory for image lists (required)")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *chartsPath == "" || *version == "" || *outputDir == "" {
		return fmt.Errorf("--charts-path, --version, and --output-dir are required")
	}

	config := imagelist.ExportConfig{
		ChartsPath: *chartsPath,
		Version:    *version,
		OutputDir:  *outputDir,
	}

	fmt.Printf("Scanning chart catalogs for image references...\n")
	fmt.Printf("  Charts path: %s\n", config.ChartsPath)
	fmt.Printf("  Version: %s\n", config.Version)
	fmt.Printf("  Output dir: %s\n\n", config.OutputDir)

	// Scan charts for image references
	refs, err := imagelist.ScanCharts(config)
	if err != nil {
		return fmt.Errorf("failed to scan charts: %w", err)
	}

	fmt.Printf("\nFound %d unique images\n", len(refs))

	// Write image lists and scripts
	if err := imagelist.WriteImageLists(refs, config); err != nil {
		return fmt.Errorf("failed to write image lists: %w", err)
	}

	fmt.Println("\n✅ Image list export complete!")

	return nil
}

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
		// Deprecated - use changed-minors
		if err := changedMinorsCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "changed-minors":
		if err := changedMinorsCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "list-minors":
		if err := listMinorsCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "calver-dev-version":
		if err := calverDevVersionCommand(); err != nil {
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
	fmt.Println("  changed-minors       Detect Rancher minors with upstream ref changes")
	fmt.Println("                       Flags: --from=<commit> --to=<commit>")
	fmt.Println("  list-minors          List all Rancher minors from config.yaml")
	fmt.Println("  calver-dev-version   Generate CalVer dev version for a Rancher minor")
	fmt.Println("                       Flags: --minor=<rancher-minor>")
	fmt.Println("  plan-release         Plan CalVer versions for releases")
	fmt.Println("                       Flags: --type=<auto|manual>")
	fmt.Println("                              --changed-minors=<json> (for auto)")
	fmt.Println("                              --minors=<json> --release=<stable|prerelease> (for manual)")
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

	// Get Rancher minors and sort for consistent output
	majors := cfg.ListChartMajors()
	sort.Strings(majors)

	fmt.Printf("Found %d Rancher minor versions: %v\n", len(majors), majors)

	// Compute copy script hash for reproducibility
	fmt.Printf("\nComputing copy-charts.sh hash...\n")
	scriptHash, err := lockfile.ComputeFileHash(copyScriptPath)
	if err != nil {
		return fmt.Errorf("failed to compute copy script hash: %w", err)
	}
	fmt.Printf("  Script hash: %s\n", scriptHash[:16]+"...")
	lock.CopyScriptHash = scriptHash

	// Generate Dockerfiles and query upstream for each Rancher minor
	for _, major := range majors {
		fmt.Printf("\nProcessing Rancher minor: %s\n", major)

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

func changedMinorsCommand() error {
	// Parse flags
	fs := flag.NewFlagSet("changed-minors", flag.ExitOnError)
	fromCommit := fs.String("from", "", "From commit (required)")
	toCommit := fs.String("to", "", "To commit (required)")
	verbose := fs.Bool("verbose", false, "Show detailed change information")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *fromCommit == "" || *toCommit == "" {
		return fmt.Errorf("both --from and --to are required")
	}

	// Get changed Rancher minors
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
			fmt.Println("No Rancher minors with upstream ref changes detected")
			fmt.Println("(Only timestamp changes in lock.yaml)")
		} else {
			fmt.Printf("Changed Rancher minors (%d):\n", len(changed))
			for _, minor := range changed {
				fmt.Printf("  - %s\n", minor)
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

func listMinorsCommand() error {
	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get all Rancher minors
	minors := cfg.ListRancherMinors()

	// Sort for consistent output
	sort.Strings(minors)

	// Output each minor on its own line
	for _, minor := range minors {
		fmt.Println(minor)
	}

	return nil
}

func calverDevVersionCommand() error {
	// Parse flags
	fs := flag.NewFlagSet("calver-dev-version", flag.ExitOnError)
	minor := fs.String("minor", "", "Rancher minor (e.g., 2.15)")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *minor == "" {
		return fmt.Errorf("--minor is required")
	}

	// Generate CalVer dev version
	version := versions.GenerateCalVerTimestamp(*minor, "prerelease")
	fmt.Println(version)

	return nil
}

func planReleaseCommand() error {
	// Parse flags
	fs := flag.NewFlagSet("plan-release", flag.ExitOnError)
	planType := fs.String("type", "", "Plan type: auto or manual (required)")
	changedMinorsJSON := fs.String("changed-minors", "", "JSON array of changed Rancher minors (for auto)")
	minorsJSON := fs.String("minors", "", "JSON array of Rancher minors to release (for manual)")
	releaseType := fs.String("release", "", "Release type: stable or prerelease (for manual)")
	verbose := fs.Bool("verbose", false, "Show detailed output")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *planType == "" {
		return fmt.Errorf("--type is required")
	}

	var plans []versions.ReleasePlan
	var err error

	switch *planType {
	case "auto":
		if *changedMinorsJSON == "" {
			return fmt.Errorf("--changed-minors is required for auto type")
		}

		var changedMinors []string
		if err := json.Unmarshal([]byte(*changedMinorsJSON), &changedMinors); err != nil {
			return fmt.Errorf("failed to parse changed-minors JSON: %w", err)
		}

		plans, err = versions.PlanAutoPrerelease(changedMinors)
		if err != nil {
			return err
		}

	case "manual":
		if *minorsJSON == "" || *releaseType == "" {
			return fmt.Errorf("--minors and --release are required for manual type")
		}

		var minors []string
		if err := json.Unmarshal([]byte(*minorsJSON), &minors); err != nil {
			return fmt.Errorf("failed to parse minors JSON: %w", err)
		}

		plans, err = versions.PlanManualRelease(minors, *releaseType)
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
				fmt.Printf("  %s: %s (%s)\n",
					plan.RancherMinor, plan.Version, plan.ReleaseType)
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
	results, err := imagelist.ScanCharts(config)
	if err != nil {
		return fmt.Errorf("failed to scan charts: %w", err)
	}

	// Calculate totals across all catalogs
	totalValid := 0
	totalInvalid := 0
	for _, result := range results {
		totalValid += len(result.ValidImages)
		totalInvalid += len(result.InvalidImages)
	}

	fmt.Printf("\nScan complete:\n")
	fmt.Printf("  Catalogs scanned: %d\n", len(results))
	fmt.Printf("  Total valid images: %d\n", totalValid)
	if totalInvalid > 0 {
		fmt.Printf("  Total invalid images: %d ⚠️\n", totalInvalid)
	}

	// Write image lists and scripts
	if err := imagelist.WriteImageLists(results, config); err != nil {
		return fmt.Errorf("failed to write image lists: %w", err)
	}

	fmt.Println("\n✅ Image list export complete!")
	if totalInvalid > 0 {
		fmt.Printf("\n⚠️  Warning: %d invalid image references were found and excluded.\n", totalInvalid)
		fmt.Printf("   See *-invalid-images.txt files in each catalog directory for details.\n")
	}

	return nil
}

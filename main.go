package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/rancher/rancher-assets/internal/config"
	"github.com/rancher/rancher-assets/internal/generator"
	"github.com/rancher/rancher-assets/internal/imagelist"
	"github.com/rancher/rancher-assets/internal/lockfile"
	"github.com/rancher/rancher-assets/internal/logger"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	command := os.Args[1]

	switch command {
	case "generate":
		if err := generateCommand(ctx); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "changed-majors":
		// Deprecated - use changed-minors
		if err := changedMinorsCommand(ctx); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "changed-minors":
		if err := changedMinorsCommand(ctx); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "list-minors":
		if err := listMinorsCommand(ctx); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "calver-dev-version":
		if err := calverDevVersionCommand(ctx); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "plan-release":
		if err := planReleaseCommand(ctx); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "export-images":
		if err := exportImagesCommand(ctx); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	default:
		logger.Error("Unknown command: %s", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	logger.Println("Usage: rancher-assets <command>")
	logger.Println("")
	logger.Println("Commands:")
	logger.Println("  generate             Generate Dockerfiles and update lock.yaml")
	logger.Println("  changed-minors       Detect Rancher minors with upstream ref changes")
	logger.Println("                       Flags: --from=<commit> --to=<commit>")
	logger.Println("  list-minors          List all Rancher minors from config.yaml")
	logger.Println("  calver-dev-version   Generate CalVer dev version for a Rancher minor")
	logger.Println("                       Flags: --minor=<rancher-minor>")
	logger.Println("  plan-release         Plan CalVer versions for releases")
	logger.Println("                       Flags: --type=<auto|manual>")
	logger.Println("                              --changed-minors=<json> (for auto)")
	logger.Println("                              --minors=<json> --release=<stable|prerelease> (for manual)")
	logger.Println("  export-images        Generate image lists from chart catalogs")
	logger.Println("                       Flags: --charts-path=<path> --version=<version> --output-dir=<path>")
}

func generateCommand(ctx context.Context) error {
	logger.Info("Loading configuration...")

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

	logger.Info("Found %d Rancher minor versions: %v", len(majors), majors)

	// Compute copy script hash for reproducibility
	logger.Info("\nComputing copy-charts.sh hash...")
	scriptHash, err := lockfile.ComputeFileHash(copyScriptPath)
	if err != nil {
		return fmt.Errorf("failed to compute copy script hash: %w", err)
	}
	logger.Info("  Script hash: %s", scriptHash[:16]+"...")
	lock.CopyScriptHash = scriptHash

	// Generate Dockerfiles and query upstream for each Rancher minor
	for _, major := range majors {
		logger.Info("\nProcessing Rancher minor: %s", major)

		// Ensure lock entry exists
		lock.EnsureChartVersion(major)

		// Query upstream repos first (before generation, so commits are available)
		logger.Info("  Querying upstream repositories...")
		chartCfg, _ := cfg.GetChartVersion(major)

		// Query both prod and dev branches
		var prodRefs, devRefs lockfile.UpstreamRefsSet

		// Query PROD branches
		logger.Info("    [prod]")
		logger.StartProgress("      - charts @ %s: ", chartCfg.Prod.ChartsBranch)
		chartsRef, err := lockfile.QueryUpstreamRef(ctx, cfg.ClusterRepos["charts"].URL, chartCfg.Prod.ChartsBranch)
		if err != nil {
			logger.CompleteProgress("FAILED (%v)", err)
			return fmt.Errorf("failed to query charts upstream: %w", err)
		}
		logger.CompleteProgress("%s", chartsRef.Commit[:8])
		prodRefs.Charts = chartsRef

		logger.StartProgress("      - partner @ %s: ", chartCfg.Prod.PartnerBranch)
		partnerRef, err := lockfile.QueryUpstreamRef(ctx, cfg.ClusterRepos["partner"].URL, chartCfg.Prod.PartnerBranch)
		if err != nil {
			logger.CompleteProgress("FAILED (%v)", err)
			return fmt.Errorf("failed to query partner upstream: %w", err)
		}
		logger.CompleteProgress("%s", partnerRef.Commit[:8])
		prodRefs.Partner = partnerRef

		logger.StartProgress("      - rke2 @ %s: ", chartCfg.Prod.Rke2Branch)
		rke2Ref, err := lockfile.QueryUpstreamRef(ctx, cfg.ClusterRepos["rke2"].URL, chartCfg.Prod.Rke2Branch)
		if err != nil {
			logger.CompleteProgress("FAILED (%v)", err)
			return fmt.Errorf("failed to query rke2 upstream: %w", err)
		}
		logger.CompleteProgress("%s", rke2Ref.Commit[:8])
		prodRefs.Rke2 = rke2Ref

		// Query DEV branches
		logger.Info("    [dev]")
		logger.StartProgress("      - charts @ %s: ", chartCfg.Dev.ChartsBranch)
		chartsRef, err = lockfile.QueryUpstreamRef(ctx, cfg.ClusterRepos["charts"].URL, chartCfg.Dev.ChartsBranch)
		if err != nil {
			logger.CompleteProgress("FAILED (%v)", err)
			return fmt.Errorf("failed to query charts upstream: %w", err)
		}
		logger.CompleteProgress("%s", chartsRef.Commit[:8])
		devRefs.Charts = chartsRef

		logger.StartProgress("      - partner @ %s: ", chartCfg.Dev.PartnerBranch)
		partnerRef, err = lockfile.QueryUpstreamRef(ctx, cfg.ClusterRepos["partner"].URL, chartCfg.Dev.PartnerBranch)
		if err != nil {
			logger.CompleteProgress("FAILED (%v)", err)
			return fmt.Errorf("failed to query partner upstream: %w", err)
		}
		logger.CompleteProgress("%s", partnerRef.Commit[:8])
		devRefs.Partner = partnerRef

		logger.StartProgress("      - rke2 @ %s: ", chartCfg.Dev.Rke2Branch)
		rke2Ref, err = lockfile.QueryUpstreamRef(ctx, cfg.ClusterRepos["rke2"].URL, chartCfg.Dev.Rke2Branch)
		if err != nil {
			logger.CompleteProgress("FAILED (%v)", err)
			return fmt.Errorf("failed to query rke2 upstream: %w", err)
		}
		logger.CompleteProgress("%s", rke2Ref.Commit[:8])
		devRefs.Rke2 = rke2Ref

		// Update lock file with both prod and dev refs
		if err := lock.UpdateUpstreamRefs(major, prodRefs, devRefs); err != nil {
			return fmt.Errorf("failed to update lock file: %w", err)
		}

		// Generate Dockerfile (after updating lock, so commits are available)
		logger.Info("  Generating Dockerfile.%s...", major)
		if err := generator.Generate(cfg, lock, major, dockerfilesDir); err != nil {
			return fmt.Errorf("failed to generate Dockerfile for %s: %w", major, err)
		}
	}

	// Save lock file
	logger.Info("\nSaving lock file...")
	if err := lock.Save(lockPath); err != nil {
		return fmt.Errorf("failed to save lock file: %w", err)
	}

	logger.Success("\nGeneration complete!")
	logger.Info("\nGenerated files:")
	for _, major := range majors {
		logger.Info("  - dockerfiles/Dockerfile.%s", major)
	}
	logger.Info("  - %s", lockPath)
	logger.Info("\nReview changes with: git diff dockerfiles/ lock.yaml")

	return nil
}

func changedMinorsCommand(ctx context.Context) error {
	// Parse flags
	fs := flag.NewFlagSet("changed-minors", flag.ExitOnError)
	fromCommit := fs.String("from", "", "From commit (required)")
	toCommit := fs.String("to", "", "To commit (required)")
	verbose := fs.Bool("verbose", false, "Show detailed change information")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *fromCommit == "" || *toCommit == "" {
		return errors.New("both --from and --to are required")
	}

	// Get changed Rancher minors
	changed, err := lockfile.ChangedMajors(ctx, *fromCommit, *toCommit)
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
			logger.Info("No Rancher minors with upstream ref changes detected")
			logger.Info("(Only timestamp changes in lock.yaml)")
		} else {
			logger.Info("Changed Rancher minors (%d):", len(changed))
			for _, minor := range changed {
				logger.Info("  - %s", minor)
			}
		}
	}

	// Always output JSON array (for workflow consumption)
	output, err := json.Marshal(changed)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	logger.Println(string(output))
	return nil
}

func listMinorsCommand(ctx context.Context) error {
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
		logger.Println(minor)
	}

	return nil
}

func calverDevVersionCommand(ctx context.Context) error {
	// Parse flags
	fs := flag.NewFlagSet("calver-dev-version", flag.ExitOnError)
	minor := fs.String("minor", "", "Rancher minor (e.g., 2.15)")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *minor == "" {
		return errors.New("--minor is required")
	}

	// Generate CalVer dev version
	version := versions.GenerateCalVerTimestamp(*minor, "prerelease")
	logger.Println(version)

	return nil
}

func planReleaseCommand(ctx context.Context) error {
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
		return errors.New("--type is required")
	}

	var plans []versions.ReleasePlan
	var err error

	switch *planType {
	case "auto":
		if *changedMinorsJSON == "" {
			return errors.New("--changed-minors is required for auto type")
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
			return errors.New("--minors and --release are required for manual type")
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
			logger.Info("No releases planned")
		} else {
			logger.Info("Planned releases (%d):", len(plans))
			for _, plan := range plans {
				logger.Info("  %s: %s (%s)",
					plan.RancherMinor, plan.Version, plan.ReleaseType)
			}
		}
	}

	// Always output JSON for workflow consumption
	output, err := json.Marshal(plans)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	logger.Println(string(output))
	return nil
}

func exportImagesCommand(ctx context.Context) error {
	// Parse flags
	fs := flag.NewFlagSet("export-images", flag.ExitOnError)
	chartsPath := fs.String("charts-path", "", "Path to extracted chart catalogs (required)")
	version := fs.String("version", "", "Chart image version being exported (required)")
	outputDir := fs.String("output-dir", "", "Output directory for image lists (required)")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *chartsPath == "" || *version == "" || *outputDir == "" {
		return errors.New("--charts-path, --version, and --output-dir are required")
	}

	config := imagelist.ExportConfig{
		ChartsPath: *chartsPath,
		Version:    *version,
		OutputDir:  *outputDir,
	}

	logger.Info("Scanning chart catalogs for image references...")
	logger.Info("  Charts path: %s", config.ChartsPath)
	logger.Info("  Version: %s", config.Version)
	logger.Info("  Output dir: %s\n", config.OutputDir)

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

	logger.Info("\nScan complete:")
	logger.Info("  Catalogs scanned: %d", len(results))
	logger.Info("  Total valid images: %d", totalValid)
	if totalInvalid > 0 {
		logger.Info("  Total invalid images: %d ⚠️", totalInvalid)
	}

	// Write image lists and scripts
	if err := imagelist.WriteImageLists(results, config); err != nil {
		return fmt.Errorf("failed to write image lists: %w", err)
	}

	logger.Success("\nImage list export complete!")
	if totalInvalid > 0 {
		logger.Warn("\nWarning: %d invalid image references were found and excluded.", totalInvalid)
		logger.Info("   See *-invalid-images.txt files in each catalog directory for details.")
	}

	return nil
}

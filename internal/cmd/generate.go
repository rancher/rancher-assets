package cmd

import (
	"context"
	"flag"
	"fmt"
	"sort"

	"github.com/rancher/rancher-assets/internal/config"
	"github.com/rancher/rancher-assets/internal/generator"
	"github.com/rancher/rancher-assets/internal/lockfile"
	"github.com/rancher/rancher-assets/internal/logger"
)

const (
	configPath     = "config.yaml"
	lockPath       = "lock.yaml"
	dockerfilesDir = "dockerfiles"
	copyScriptPath = "package/copy-charts.sh"
)

func Generate(ctx context.Context, args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	update := fs.Bool("update", false, "Query upstream repos and update lock.yaml before generating")

	if err := fs.Parse(args); err != nil {
		return err
	}

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

	// Update lock file with upstream refs if --update flag is set
	if *update {
		// Compute copy script hash for reproducibility
		logger.Info("\nComputing copy-charts.sh hash...")
		scriptHash, err := lockfile.ComputeFileHash(copyScriptPath)
		if err != nil {
			return fmt.Errorf("failed to compute copy script hash: %w", err)
		}
		logger.Info("  Script hash: %s", scriptHash[:16]+"...")
		lock.CopyScriptHash = scriptHash

		// Query upstream for each Rancher minor
		for _, major := range majors {
			logger.Info("\nProcessing Rancher minor: %s", major)

			// Ensure lock entry exists
			lock.EnsureChartVersion(major)

			// Query upstream repos
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
		}

		// Save lock file
		logger.Info("\nSaving lock file...")
		if err := lock.Save(lockPath); err != nil {
			return fmt.Errorf("failed to save lock file: %w", err)
		}
	} else {
		logger.Info("\nUsing existing lock file (use --update to query upstream repos)")
	}

	// Generate Dockerfiles for each Rancher minor
	logger.Info("\nGenerating Dockerfiles...")
	for _, major := range majors {
		logger.Info("  Generating Dockerfile.%s...", major)
		if err := generator.Generate(cfg, lock, major, dockerfilesDir); err != nil {
			return fmt.Errorf("failed to generate Dockerfile for %s: %w", major, err)
		}
	}

	logger.Success("\nGeneration complete!")
	logger.Info("\nGenerated files:")
	for _, major := range majors {
		logger.Info("  - dockerfiles/Dockerfile.%s", major)
	}
	if *update {
		logger.Info("  - %s", lockPath)
		logger.Info("\nReview changes with: git diff dockerfiles/ lock.yaml")
	} else {
		logger.Info("\nReview changes with: git diff dockerfiles/")
	}

	return nil
}

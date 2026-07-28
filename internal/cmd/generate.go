//nolint:cyclop // Package average affected by Generate function's linear workflow
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
	configPath       = "config.yaml"
	lockPath         = "lock.yaml"
	dockerfilesDir   = "dockerfiles"
	copyScriptPath   = "package/copy-charts.sh"
	prodTemplatePath = "internal/generator/tmpl/prod.tmpl"
	devTemplatePath  = "internal/generator/tmpl/dev.tmpl"
)

// Generate renders new Dockerfile and optionally updates remote repo refs
//
//nolint:cyclop // Linear workflow with proper error handling; breaking into smaller functions would reduce clarity
func Generate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	update := fs.Bool("update", false, "Query upstream repos and update lock.yaml before generating")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse command flags: %w", err)
	}

	logger.Info("Loading configuration...")

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	lock, err := lockfile.Load(lockPath)
	if err != nil {
		return fmt.Errorf("failed to load lock file: %w", err)
	}

	majors := cfg.ListChartMajors()
	sort.Strings(majors) // Consistent output order

	logger.Info("Found %d Rancher minor versions: %v", len(majors), majors)

	// Always compute file hashes for reproducibility (these are local files)
	logger.Info("\nComputing file hashes for reproducibility...")
	logger.StartProgress("  - copy-charts.sh: ")
	scriptHash, err := lockfile.ComputeFileHash(copyScriptPath)
	if err != nil {
		logger.CompleteProgress("FAILED (%v)", err)
		return fmt.Errorf("failed to compute copy script hash: %w", err)
	}
	logger.CompleteProgress("%s", scriptHash[:16]+"...")

	logger.StartProgress("  - prod.tmpl: ")
	prodHash, err := lockfile.ComputeFileHash(prodTemplatePath)
	if err != nil {
		logger.CompleteProgress("FAILED (%v)", err)
		return fmt.Errorf("failed to compute prod template hash: %w", err)
	}
	logger.CompleteProgress("%s", prodHash[:16]+"...")

	logger.StartProgress("  - dev.tmpl: ")
	devHash, err := lockfile.ComputeFileHash(devTemplatePath)
	if err != nil {
		logger.CompleteProgress("FAILED (%v)", err)
		return fmt.Errorf("failed to compute dev template hash: %w", err)
	}
	logger.CompleteProgress("%s", devHash[:16]+"...")

	lockChanged := false

	if lock.CopyScriptHash != scriptHash {
		lock.CopyScriptHash = scriptHash
		lockChanged = true
	}
	if lock.TemplateHashes.ProdTemplate != prodHash {
		lock.TemplateHashes.ProdTemplate = prodHash
		lockChanged = true
	}
	if lock.TemplateHashes.DevTemplate != devHash {
		lock.TemplateHashes.DevTemplate = devHash
		lockChanged = true
	}

	// Check if base-image values changed
	if lock.BaseImage.BciBase != cfg.BaseImage.BciBaseVersion {
		lock.BaseImage.BciBase = cfg.BaseImage.BciBaseVersion
		lockChanged = true
	}
	if lock.BaseImage.BciMicro != cfg.BaseImage.BciMicroVersion {
		lock.BaseImage.BciMicro = cfg.BaseImage.BciMicroVersion
		lockChanged = true
	}

	if *update {
		for _, major := range majors {
			logger.Info("\nProcessing Rancher minor: %s", major)

			lock.EnsureChartVersion(major)

			logger.Info("  Querying upstream repositories...")
			chartCfg, err := cfg.GetChartVersion(major)
			if err != nil {
				return fmt.Errorf("failed to get chart version for %s: %w", major, err)
			}

			var prodRefs, devRefs lockfile.UpstreamRefsSet

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
		lockChanged = true
	} else {
		logger.Info("\nUsing existing upstream refs (use --update to query remote repos)")
	}

	// Save lock file only if something changed
	if lockChanged {
		logger.Info("\nSaving lock file...")
		if err := lock.Save(lockPath); err != nil {
			return fmt.Errorf("failed to save lock file: %w", err)
		}
	} else {
		logger.Info("\nNo changes to lock file")
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
		logger.Info("  - dockerfiles/Dockerfile.%s-dev", major)
	}
	if lockChanged {
		logger.Info("  - %s (updated)", lockPath)
		logger.Info("\nReview changes with: git diff dockerfiles/ lock.yaml")
	} else {
		logger.Info("\nReview changes with: git diff dockerfiles/")
	}

	return nil
}

package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/rancher/rancher-assets/internal/imagelist"
	"github.com/rancher/rancher-assets/internal/logger"
)

// ExportImages will export a list of images for each chart repo
func ExportImages(ctx context.Context, args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("export-images", flag.ExitOnError)
	chartsPath := fs.String("charts-path", "", "Path to extracted chart catalogs (required)")
	version := fs.String("version", "", "Chart image version being exported (required)")
	outputDir := fs.String("output-dir", "", "Output directory for image lists (required)")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse command flags: %w", err)
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

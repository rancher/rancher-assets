package imagelist

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/rancher/ob-charts-tool/helmtools/chart"
	"github.com/rancher/ob-charts-tool/helmtools/image"
	"gopkg.in/yaml.v3"

	"github.com/rancher/rancher-assets/internal/logger"
)

// ImageReference represents a container image found in charts
type ImageReference struct {
	Image   string
	OS      string
	Sources []string
}

// InvalidImageEntry represents an invalid image reference with full context
type InvalidImageEntry struct {
	Image      string
	Reason     string
	CatalogRef string
	Sources    []string
}

// ImageSet is a map of image -> set of sources
type ImageSet map[string]map[string]struct{}

// InvalidImageSet tracks invalid images with their context
type InvalidImageSet map[string]*InvalidImageEntry

// ExportConfig contains paths and settings for image list export
type ExportConfig struct {
	ChartsPath string // Path to extracted chart catalogs
	Version    string // Chart image version being exported
	OutputDir  string // Directory to write output files
}

// ScanResult contains both valid and invalid image references
type ScanResult struct {
	ValidImages   []ImageReference
	InvalidImages []InvalidImageEntry
}

// CatalogResults maps catalog name to its scan results
type CatalogResults map[string]*ScanResult

// SkipChecker is a logic predicate to decide if a item should be skipped
type SkipChecker func(entry chart.IndexEntry) bool

// OS types
const (
	Linux   = "linux"
	Windows = "windows"
)

func skipOldCharts(_ chart.IndexEntry) bool {
	return false
}

// ScanCharts scans all chart catalogs and extracts image references
// Returns a map of catalog name -> ScanResult for separate tracking
func ScanCharts(config ExportConfig) (CatalogResults, error) {
	results := make(CatalogResults)

	// Scan the three chart catalogs
	catalogs := []struct {
		skipChecker SkipChecker
		name        string
		path        string
	}{
		{
			skipOldCharts,
			"rancher-charts",
			filepath.Join(config.ChartsPath, "rancher-charts"),
		},
		{nil, "rancher-partner-charts", filepath.Join(config.ChartsPath, "rancher-partner-charts")},
		{nil, "rancher-rke2-charts", filepath.Join(config.ChartsPath, "rancher-rke2-charts")},
	}

	for _, catalog := range catalogs {
		// Find the actual catalog directory (it has a hash suffix)
		catalogPath, err := findCatalogDir(catalog.path)
		if err != nil {
			logger.Warn("Warning: skipping %s: %v", catalog.name, err)
			continue
		}

		logger.Info("  Scanning %s...", catalog.name)

		// Create separate tracking sets for this catalog
		imagesSet := make(ImageSet)
		invalidSet := make(InvalidImageSet)
		shouldSkipChart := func(entry chart.IndexEntry) bool { return false }
		if catalog.skipChecker != nil {
			shouldSkipChart = catalog.skipChecker
		}

		if err := scanCatalog(catalogPath, catalog.name, imagesSet, invalidSet, shouldSkipChart); err != nil {
			return nil, fmt.Errorf("failed to scan %s: %w", catalog.name, err)
		}

		// Convert sets to sorted lists for this catalog
		results[catalog.name] = &ScanResult{
			ValidImages:   imagesToList(imagesSet),
			InvalidImages: invalidImagesToList(invalidSet),
		}
	}

	return results, nil
}

// findCatalogDir finds the catalog directory which might have a hash suffix
func findCatalogDir(basePath string) (string, error) {
	// Check if basePath/index.yaml exists (direct path)
	if _, err := os.Stat(filepath.Join(basePath, "index.yaml")); err == nil {
		return basePath, nil
	}

	// Check if basePath exists and has a hash-suffixed subdirectory
	if info, err := os.Stat(basePath); err == nil && info.IsDir() {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			return "", fmt.Errorf("failed to read directory: %w", err)
		}

		// Look for first subdirectory (should be the hash)
		for _, entry := range entries {
			if entry.IsDir() {
				subPath := filepath.Join(basePath, entry.Name())
				// Check if index.yaml exists in this subdirectory
				if _, err := os.Stat(filepath.Join(subPath, "index.yaml")); err == nil {
					return subPath, nil
				}
			}
		}
	}

	return "", fmt.Errorf("catalog directory not found at %s", basePath)
}

// processValuesYAML extracts images from a values.yaml file data
func processValuesYAML(data []byte, source, catalogRef string, imagesSet ImageSet, invalidSet InvalidImageSet) {
	// Use helmtools to extract images
	images, err := image.ExtractImages(data, "")
	if err != nil {
		// Skip invalid YAML
		return
	}

	// Process extracted images and add to our tracking sets
	for _, img := range images.Values() {
		imageRef := formatImageReference(img)
		addImage(imagesSet, invalidSet, imageRef, source, catalogRef)
	}

	// Also scan for legacy patterns not covered by helmtools
	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err == nil {
		extractLegacyImages(values, source, catalogRef, imagesSet, invalidSet)
	}
}

// scanCatalog scans a single chart catalog
func scanCatalog(catalogPath, catalogName string, imagesSet ImageSet, invalidSet InvalidImageSet, shouldSkipChart SkipChecker) error {
	indexPath := filepath.Join(catalogPath, "index.yaml")

	// Load index.yaml
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("failed to read index.yaml: %w", err)
	}

	// Parse index using helmtools
	index, err := chart.ParseIndex(indexData)
	if err != nil {
		return fmt.Errorf("failed to parse index.yaml: %w", err)
	}

	// For each chart, scan the latest version
	for _, versions := range index.Entries {
		if len(versions) == 0 {
			continue
		}

		// Use the first version (should be latest)
		version := versions[0]
		chartSource := fmt.Sprintf("%s:%s", version.Name, version.Version)

		// Check if chart should be skipped
		if shouldSkipChart(version) {
			logger.Warn("    Warning: skipping %s", chartSource)
			continue
		}

		// Find and extract the chart archive
		if len(version.URLs) == 0 {
			logger.Warn("    Warning: no URLs for %s", chartSource)
			continue
		}

		chartPath := filepath.Join(catalogPath, version.URLs[0])
		if err := extractImagesFromChart(chartPath, chartSource, catalogName, imagesSet, invalidSet); err != nil {
			logger.Warn("    Warning: failed to scan %s: %v", chartSource, err)
			continue
		}
	}

	return nil
}

// extractImagesFromChart extracts image references from a chart .tgz file
func extractImagesFromChart(chartPath, source, catalogRef string, imagesSet ImageSet, invalidSet InvalidImageSet) error {
	file, err := os.Open(chartPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Warn("failed to close file: %v", closeErr)
		}
	}()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to open gzip reader: %w", err)
	}
	defer func() {
		if closeErr := gzr.Close(); closeErr != nil {
			logger.Warn("failed to close gzip reader: %v", closeErr)
		}
	}()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to advance to next file in tar: %w", err)
		}

		// Look for values.yaml files
		if strings.HasSuffix(header.Name, "values.yaml") {
			data, err := io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			processValuesYAML(data, source, catalogRef, imagesSet, invalidSet)
		}
	}

	return nil
}

// formatImageReference converts a helmtools Image to a full image reference string
func formatImageReference(img image.Image) string {
	var parts []string

	if img.Registry != "" {
		parts = append(parts, img.Registry)
	}

	if img.Repository != "" {
		parts = append(parts, img.Repository)
	}

	imageRef := strings.Join(parts, "/")

	if img.Tag != "" {
		imageRef = fmt.Sprintf("%s:%s", imageRef, img.Tag)
	}

	return imageRef
}

// extractLegacyImages handles image patterns not covered by helmtools
// This includes direct image strings and systemDefaultRegistry overrides
func extractLegacyImages(values interface{}, source, catalogRef string, imagesSet ImageSet, invalidSet InvalidImageSet) {
	switch v := values.(type) {
	case map[string]interface{}:
		// Check for direct "image" string field (not structured as registry/repo/tag)
		if img, ok := v["image"].(string); ok && img != "" {
			// Only add if it's a complete image reference (contains : or @)
			if strings.Contains(img, ":") || strings.Contains(img, "@") {
				addImage(imagesSet, invalidSet, img, source, catalogRef)
			}
		}

		// Recurse into nested maps
		for _, val := range v {
			extractLegacyImages(val, source, catalogRef, imagesSet, invalidSet)
		}

	case []interface{}:
		// Recurse into arrays
		for _, val := range v {
			extractLegacyImages(val, source, catalogRef, imagesSet, invalidSet)
		}
	}
}

// addImage adds an image to the set with its source, or tracks it as invalid
func addImage(imagesSet ImageSet, invalidSet InvalidImageSet, image, source, catalogRef string) {
	if image == "" {
		return
	}

	// Skip template variables (e.g., {{ .Values.image }})
	if strings.Contains(image, "{{") {
		return
	}

	// Normalize image reference
	image = strings.TrimSpace(image)

	// Validate the image reference
	if err := validateImage(image); err != nil {
		if !IsWarning(err) {
			// Hard validation error - track as invalid
			reason := err.Error()
			addInvalidImage(invalidSet, image, reason, source, catalogRef)
			logger.Warn("    Invalid image in %s/%s: %s - %s", catalogRef, source, image, reason)
			return
		}
		// Log warning but allow the image through
		logger.Warn("    Warning for image in %s/%s: %s - %s", catalogRef, source, image, err.Error())
	}

	// Add to valid images set
	if imagesSet[image] == nil {
		imagesSet[image] = make(map[string]struct{})
	}
	imagesSet[image][source] = struct{}{}
}

// imagesToList converts ImageSet to sorted list of ImageReferences
func imagesToList(imagesSet ImageSet) []ImageReference {
	refs := make([]ImageReference, 0, len(imagesSet))

	// Convert to list
	for image, sources := range imagesSet {
		sourcesList := make([]string, 0, len(sources))
		for source := range sources {
			sourcesList = append(sourcesList, source)
		}
		sort.Strings(sourcesList)

		// Detect OS based on image name (simple heuristic)
		os := Linux
		if strings.Contains(image, "windows") || strings.Contains(image, "nanoserver") {
			os = Windows
		}

		refs = append(refs, ImageReference{
			Image:   image,
			Sources: sourcesList,
			OS:      os,
		})
	}

	// Sort by image name
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Image < refs[j].Image
	})

	return refs
}

// invalidImagesToList converts InvalidImageSet to sorted list of InvalidImageEntries
func invalidImagesToList(invalidSet InvalidImageSet) []InvalidImageEntry {
	entries := make([]InvalidImageEntry, 0, len(invalidSet))

	// Convert to list
	for _, entry := range invalidSet {
		// Sort sources for deterministic output
		sort.Strings(entry.Sources)
		entries = append(entries, *entry)
	}

	// Sort by image name, then by reason
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Image != entries[j].Image {
			return entries[i].Image < entries[j].Image
		}
		return entries[i].Reason < entries[j].Reason
	})

	return entries
}

// writeImageListFile writes a single image list file for a specific OS type
func writeImageListFile(outputDir, catalogName, suffix string, images []ImageReference) error {
	if len(images) == 0 {
		return nil
	}

	filename := catalogName + suffix
	imageListPath := filepath.Join(outputDir, filename)
	var imageList []string
	for _, img := range images {
		imageList = append(imageList, img.Image)
	}
	if err := writeLines(imageListPath, imageList); err != nil {
		return err
	}
	logger.Info("  - %s (%d images)", filename, len(images))
	return nil
}

// WriteImageLists generates one image list file per catalog
func WriteImageLists(results CatalogResults, config ExportConfig) error {
	// Ensure output directory exists
	if err := os.MkdirAll(config.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	logger.Info("\nGenerating image lists in %s/:", config.OutputDir)

	// Process each catalog separately
	for catalogName, result := range results {
		refs := result.ValidImages

		// Separate Linux and Windows images
		var linuxImages, windowsImages []ImageReference
		for _, ref := range refs {
			if ref.OS == Windows {
				windowsImages = append(windowsImages, ref)
			} else {
				linuxImages = append(linuxImages, ref)
			}
		}

		// Generate main image list file (Linux images)
		if err := writeImageListFile(config.OutputDir, catalogName, "-images.txt", linuxImages); err != nil {
			return err
		}

		// Generate Windows image list file if needed
		if err := writeImageListFile(config.OutputDir, catalogName, "-windows-images.txt", windowsImages); err != nil {
			return err
		}

		if len(result.InvalidImages) > 0 {
			if err := writeInvalidImagesReport(catalogName, result.InvalidImages, config.OutputDir); err != nil {
				return err
			}
			logger.Info("  - %s-invalid-images.txt (%d invalid entries)", catalogName, len(result.InvalidImages))
		}
	}

	return nil
}

// writeLines writes a slice of strings to a file, one per line
func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n" // Ensure trailing newline
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

// writeInvalidImagesReport writes a detailed report of invalid image references for a catalog
func writeInvalidImagesReport(catalogName string, invalidImages []InvalidImageEntry, outputDir string) error {
	path := filepath.Join(outputDir, catalogName+"-invalid-images.txt")

	var buf bytes.Buffer

	// Parse and execute header template
	headerTmpl, err := template.New("header").Parse(imageRefsReport)
	if err != nil {
		return fmt.Errorf("failed to parse header template: %w", err)
	}

	headerData := struct {
		InvalidCount int
	}{
		InvalidCount: len(invalidImages),
	}

	if execErr := headerTmpl.Execute(&buf, headerData); execErr != nil {
		return fmt.Errorf("failed to execute header template: %w", execErr)
	}

	// Parse item template
	itemTmpl, err := template.New("item").Parse(imageRefItem)
	if err != nil {
		return fmt.Errorf("failed to parse item template: %w", err)
	}

	// Add each invalid entry using template
	for _, entry := range invalidImages {
		itemData := struct {
			Image      string
			Reason     string
			CatalogRef string
			Sources    string
		}{
			Image:      entry.Image,
			Reason:     entry.Reason,
			CatalogRef: entry.CatalogRef,
			Sources:    strings.Join(entry.Sources, ", "),
		}

		if err := itemTmpl.Execute(&buf, itemData); err != nil {
			return fmt.Errorf("failed to execute item template: %w", err)
		}
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("failed to write report to file: %w", err)
	}

	return nil
}

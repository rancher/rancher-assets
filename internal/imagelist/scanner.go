package imagelist

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rancher/ob-charts-tool/helmtools/chart"
	"github.com/rancher/ob-charts-tool/helmtools/image"
	"gopkg.in/yaml.v3"
)

const (
	// OS types
	Linux   = "linux"
	Windows = "windows"
)

// ScanResult contains both valid and invalid image references
type ScanResult struct {
	ValidImages   []ImageReference
	InvalidImages []InvalidImageEntry
}

// ScanCharts scans all chart catalogs and extracts image references
func ScanCharts(config ExportConfig) (*ScanResult, error) {
	imagesSet := make(ImageSet)
	invalidSet := make(InvalidImageSet)

	// Scan the three chart catalogs
	catalogs := []struct {
		name string
		path string
	}{
		{"rancher-charts", filepath.Join(config.ChartsPath, "rancher-charts")},
		{"rancher-partner-charts", filepath.Join(config.ChartsPath, "rancher-partner-charts")},
		{"rancher-rke2-charts", filepath.Join(config.ChartsPath, "rancher-rke2-charts")},
	}

	for _, catalog := range catalogs {
		// Find the actual catalog directory (it has a hash suffix)
		catalogPath, err := findCatalogDir(catalog.path)
		if err != nil {
			fmt.Printf("Warning: skipping %s: %v\n", catalog.name, err)
			continue
		}

		fmt.Printf("  Scanning %s...\n", catalog.name)
		if err := scanCatalog(catalogPath, catalog.name, imagesSet, invalidSet); err != nil {
			return nil, fmt.Errorf("failed to scan %s: %w", catalog.name, err)
		}
	}

	// Convert sets to sorted lists
	result := &ScanResult{
		ValidImages:   imagesToList(imagesSet),
		InvalidImages: invalidImagesToList(invalidSet),
	}

	return result, nil
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
			return "", err
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

// scanCatalog scans a single chart catalog
func scanCatalog(catalogPath string, catalogName string, imagesSet ImageSet, invalidSet InvalidImageSet) error {
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

		// Find and extract the chart archive
		if len(version.URLs) == 0 {
			fmt.Printf("    Warning: no URLs for %s\n", chartSource)
			continue
		}

		chartPath := filepath.Join(catalogPath, version.URLs[0])
		if err := extractImagesFromChart(chartPath, chartSource, catalogName, imagesSet, invalidSet); err != nil {
			fmt.Printf("    Warning: failed to scan %s: %v\n", chartSource, err)
			continue
		}
	}

	return nil
}

// extractImagesFromChart extracts image references from a chart .tgz file
func extractImagesFromChart(chartPath string, source string, catalogRef string, imagesSet ImageSet, invalidSet InvalidImageSet) error {
	file, err := os.Open(chartPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Look for values.yaml files
		if strings.HasSuffix(header.Name, "values.yaml") {
			data, err := io.ReadAll(tr)
			if err != nil {
				return err
			}

			// Use helmtools to extract images
			images, err := image.ExtractImages(data, "")
			if err != nil {
				// Skip invalid YAML
				continue
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
func extractLegacyImages(values interface{}, source string, catalogRef string, imagesSet ImageSet, invalidSet InvalidImageSet) {
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
func addImage(imagesSet ImageSet, invalidSet InvalidImageSet, image string, source string, catalogRef string) {
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
		// Track as invalid and emit warning
		reason := err.Error()
		addInvalidImage(invalidSet, image, reason, source, catalogRef)
		fmt.Printf("    ⚠️  Invalid image in %s/%s: %s - %s\n", catalogRef, source, image, reason)
		return
	}

	// Add to valid images set
	if imagesSet[image] == nil {
		imagesSet[image] = make(map[string]struct{})
	}
	imagesSet[image][source] = struct{}{}
}

// imagesToList converts ImageSet to sorted list of ImageReferences
func imagesToList(imagesSet ImageSet) []ImageReference {
	var refs []ImageReference

	// Convert to list
	for image, sources := range imagesSet {
		var sourcesList []string
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
	var entries []InvalidImageEntry

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

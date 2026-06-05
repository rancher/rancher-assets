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

	"gopkg.in/yaml.v3"
)

const (
	// OS types
	Linux   = "linux"
	Windows = "windows"
)

// ChartIndex represents the structure of a Helm chart repository index.yaml
type ChartIndex struct {
	Entries map[string][]ChartVersion `yaml:"entries"`
}

// ChartVersion represents a single chart version in the index
type ChartVersion struct {
	Name    string   `yaml:"name"`
	Version string   `yaml:"version"`
	URLs    []string `yaml:"urls"`
}

// ScanCharts scans all chart catalogs and extracts image references
func ScanCharts(config ExportConfig) ([]ImageReference, error) {
	imagesSet := make(ImageSet)

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
		if err := scanCatalog(catalogPath, catalog.name, imagesSet); err != nil {
			return nil, fmt.Errorf("failed to scan %s: %w", catalog.name, err)
		}
	}

	// Convert ImageSet to sorted list
	return imagesToList(imagesSet), nil
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
func scanCatalog(catalogPath string, catalogName string, imagesSet ImageSet) error {
	indexPath := filepath.Join(catalogPath, "index.yaml")

	// Load index.yaml
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("failed to read index.yaml: %w", err)
	}

	var index ChartIndex
	if err := yaml.Unmarshal(indexData, &index); err != nil {
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
		if err := extractImagesFromChart(chartPath, chartSource, imagesSet); err != nil {
			fmt.Printf("    Warning: failed to scan %s: %v\n", chartSource, err)
			continue
		}
	}

	return nil
}

// extractImagesFromChart extracts image references from a chart .tgz file
func extractImagesFromChart(chartPath string, source string, imagesSet ImageSet) error {
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

			// Parse values.yaml
			var values map[string]interface{}
			if err := yaml.Unmarshal(data, &values); err != nil {
				// Skip invalid YAML
				continue
			}

			// Extract images from values
			extractImages(values, source, imagesSet)
		}
	}

	return nil
}

// extractImages recursively extracts image references from values map
func extractImages(values interface{}, source string, imagesSet ImageSet) {
	switch v := values.(type) {
	case map[string]interface{}:
		// Check for direct image reference patterns
		if img := getImageFromMap(v); img != "" {
			addImage(imagesSet, img, source)
		}

		// Recurse into nested maps
		for _, val := range v {
			extractImages(val, source, imagesSet)
		}

	case []interface{}:
		// Recurse into arrays
		for _, val := range v {
			extractImages(val, source, imagesSet)
		}
	}
}

// getImageFromMap tries to extract an image reference from a map
func getImageFromMap(m map[string]interface{}) string {
	// Pattern 1: Direct "image" field
	if img, ok := m["image"].(string); ok && img != "" {
		return img
	}

	// Pattern 2: repository + tag
	repo, hasRepo := m["repository"].(string)
	tag, hasTag := m["tag"].(string)
	if hasRepo && hasTag && repo != "" && tag != "" {
		return fmt.Sprintf("%s:%s", repo, tag)
	}

	// Pattern 3: registry + repository + tag (handle systemDefaultRegistry override)
	if hasRepo && hasTag {
		// Check for global.cattle.systemDefaultRegistry
		if registry := getSystemDefaultRegistry(m); registry != "" {
			return fmt.Sprintf("%s/%s:%s", registry, repo, tag)
		}
	}

	return ""
}

// getSystemDefaultRegistry tries to find systemDefaultRegistry in values
func getSystemDefaultRegistry(values map[string]interface{}) string {
	if global, ok := values["global"].(map[string]interface{}); ok {
		if cattle, ok := global["cattle"].(map[string]interface{}); ok {
			if reg, ok := cattle["systemDefaultRegistry"].(string); ok {
				return reg
			}
		}
	}
	return ""
}

// addImage adds an image to the set with its source
func addImage(imagesSet ImageSet, image string, source string) {
	if image == "" {
		return
	}

	// Skip template variables (e.g., {{ .Values.image }})
	if strings.Contains(image, "{{") {
		return
	}

	// Normalize image reference
	image = strings.TrimSpace(image)

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

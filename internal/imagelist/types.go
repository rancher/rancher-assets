package imagelist

// ImageReference represents a container image found in charts
type ImageReference struct {
	Image   string   // Full image reference (e.g., rancher/fleet:v0.10.2)
	Sources []string // Chart sources (e.g., ["fleet:103.0.2+up0.10.2"])
	OS      string   // "linux" or "windows"
}

// InvalidImageEntry represents an invalid image reference with full context
type InvalidImageEntry struct {
	Image      string   // The invalid image reference
	Reason     string   // Why it's invalid
	Sources    []string // Chart sources where this was found (chart:version)
	CatalogRef string   // Cluster repo catalog reference
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

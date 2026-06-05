package imagelist

// ImageReference represents a container image found in charts
type ImageReference struct {
	Image   string   // Full image reference (e.g., rancher/fleet:v0.10.2)
	Sources []string // Chart sources (e.g., ["fleet:103.0.2+up0.10.2"])
	OS      string   // "linux" or "windows"
}

// ImageSet is a map of image -> set of sources
type ImageSet map[string]map[string]struct{}

// ExportConfig contains paths and settings for image list export
type ExportConfig struct {
	ChartsPath string // Path to extracted chart catalogs
	Version    string // Chart image version being exported
	OutputDir  string // Directory to write output files
}

package config

// Config represents the complete configuration from config.yaml
type Config struct {
	ChartVersions map[string]ChartVersionConfig `yaml:"chart-versions"`
	BaseImage     BaseImageConfig               `yaml:"base-image"`
	ClusterRepos  map[string]ClusterRepoConfig  `yaml:"cluster-repos"`
}

// ChartVersionConfig defines a chart major version configuration (e.g., v0, v1)
type ChartVersionConfig struct {
	RancherBranch string      `yaml:"rancher-branch"`
	Prod          BuildConfig `yaml:"prod"`
	Dev           BuildConfig `yaml:"dev"`
}

// BuildConfig defines upstream branch configuration for a build type
type BuildConfig struct {
	ChartsBranch  string `yaml:"charts-branch"`
	PartnerBranch string `yaml:"partner-branch"`
	Rke2Branch    string `yaml:"rke2-branch"`
}

// BaseImageConfig defines base image versions
type BaseImageConfig struct {
	BciVersion string `yaml:"bci-version"`
}

// ClusterRepoConfig defines an upstream repository with its URL and catalog path
type ClusterRepoConfig struct {
	URL  string `yaml:"url"`
	Path string `yaml:"path"`
}

// BuildType represents prod or dev builds
type BuildType string

const (
	BuildTypeProd BuildType = "prod"
	BuildTypeDev  BuildType = "dev"
)

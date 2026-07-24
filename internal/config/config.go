package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration from config.yaml
type Config struct {
	ChartVersions map[string]ChartVersionConfig `yaml:"chart-versions"`
	ClusterRepos  map[string]ClusterRepoConfig  `yaml:"cluster-repos"`
	BaseImage     BaseImageConfig               `yaml:"base-image"`
}

// ChartVersionConfig defines a Rancher minor version configuration (e.g., 2.14, 2.15)
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

// Load reads and parses the config.yaml file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// Validate checks that all required fields are present and valid
func (c *Config) Validate() error {
	if len(c.ChartVersions) == 0 {
		return errors.New("no chart versions defined")
	}

	for major, chartCfg := range c.ChartVersions {
		if chartCfg.RancherBranch == "" {
			return fmt.Errorf("chart version %s: rancher-branch is required", major)
		}

		// Validate prod config
		if err := validateBuildConfig(major, "prod", chartCfg.Prod); err != nil {
			return err
		}

		// Validate dev config
		if err := validateBuildConfig(major, "dev", chartCfg.Dev); err != nil {
			return err
		}
	}

	if c.BaseImage.BciVersion == "" {
		return errors.New("base-image.bci-version is required")
	}

	// Validate cluster repos
	requiredRepos := []string{"charts", "partner", "rke2"}
	for _, repo := range requiredRepos {
		repoConfig, exists := c.ClusterRepos[repo]
		if !exists {
			return clusterRepoKeyErr(repo, "")
		}
		if repoConfig.URL == "" {
			return clusterRepoKeyErr(repo, "url")
		}
		if repoConfig.Path == "" {
			return clusterRepoKeyErr(repo, "path")
		}
	}

	return nil
}

func clusterRepoKeyErr(repo, key string) error {
	text := "cluster-repos." + repo
	if key != "" {
		text += "." + key
	}
	text += " is required"
	return errors.New(text)
}

// validateBuildConfig validates a single build configuration
func validateBuildConfig(major, buildType string, cfg BuildConfig) error {
	if cfg.ChartsBranch == "" {
		return fmt.Errorf("chart version %s: %s.charts-branch is required", major, buildType)
	}
	if cfg.PartnerBranch == "" {
		return fmt.Errorf("chart version %s: %s.partner-branch is required", major, buildType)
	}
	if cfg.Rke2Branch == "" {
		return fmt.Errorf("chart version %s: %s.rke2-branch is required", major, buildType)
	}
	return nil
}

// GetChartVersion returns the chart version config for a specific Rancher minor
func (c *Config) GetChartVersion(major string) (*ChartVersionConfig, error) {
	chartCfg, exists := c.ChartVersions[major]
	if !exists {
		return nil, errors.New("rancher minor " + major + " not found in config")
	}
	return &chartCfg, nil
}

// ListRancherMinors returns a list of all Rancher minor versions
func (c *Config) ListRancherMinors() []string {
	minors := make([]string, 0, len(c.ChartVersions))
	for minor := range c.ChartVersions {
		minors = append(minors, minor)
	}
	return minors
}

// ListChartMajors is deprecated - use ListRancherMinors instead
// Kept for backwards compatibility during migration
func (c *Config) ListChartMajors() []string {
	return c.ListRancherMinors()
}

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

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
		return fmt.Errorf("no chart versions defined")
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
		return fmt.Errorf("base-image.bci-version is required")
	}

	// Validate cluster repos
	requiredRepos := []string{"charts", "partner", "rke2"}
	for _, repo := range requiredRepos {
		repoConfig, exists := c.ClusterRepos[repo]
		if !exists {
			return fmt.Errorf("cluster-repos.%s is required", repo)
		}
		if repoConfig.URL == "" {
			return fmt.Errorf("cluster-repos.%s.url is required", repo)
		}
		if repoConfig.Path == "" {
			return fmt.Errorf("cluster-repos.%s.path is required", repo)
		}
	}

	return nil
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

// GetBuildConfig returns the build configuration for a specific chart major and build type
func (c *Config) GetBuildConfig(major string, buildType BuildType) (*BuildConfig, error) {
	chartCfg, exists := c.ChartVersions[major]
	if !exists {
		return nil, fmt.Errorf("chart major %s not found in config", major)
	}

	switch buildType {
	case BuildTypeProd:
		return &chartCfg.Prod, nil
	case BuildTypeDev:
		return &chartCfg.Dev, nil
	default:
		return nil, fmt.Errorf("invalid build type: %s", buildType)
	}
}

// GetChartVersion returns the chart version config for a specific major
func (c *Config) GetChartVersion(major string) (*ChartVersionConfig, error) {
	chartCfg, exists := c.ChartVersions[major]
	if !exists {
		return nil, fmt.Errorf("chart major %s not found in config", major)
	}
	return &chartCfg, nil
}

// ListChartMajors returns a list of all chart major versions
func (c *Config) ListChartMajors() []string {
	majors := make([]string, 0, len(c.ChartVersions))
	for major := range c.ChartVersions {
		majors = append(majors, major)
	}
	return majors
}

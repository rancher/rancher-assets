package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/rancher/rancher-assets/internal/config"
	"github.com/rancher/rancher-assets/internal/lockfile"
)

// TemplateData holds the data for Dockerfile template rendering
type TemplateData struct {
	BciVersion   string
	RancherMinor string

	// Dev configuration
	DevChartsBranch  string
	DevChartCommit   string
	DevPartnerBranch string
	DevPartnerCommit string
	DevRke2Branch    string
	DevRke2Commit    string

	// Prod configuration
	ProdChartsBranch  string
	ProdChartCommit   string
	ProdPartnerBranch string
	ProdPartnerCommit string
	ProdRke2Branch    string
	ProdRke2Commit    string

	// Common
	ClusterRepos   map[string]config.ClusterRepoConfig
	RancherVersion string
}

// Generate creates TWO Dockerfiles for the specified Rancher minor version:
// - Dockerfile.{minor} (prod/stable)
// - Dockerfile.{minor}-dev (dev/prerelease)
func Generate(cfg *config.Config, lock *lockfile.Lock, minor, outputDir string) error {
	// Get chart version config
	chartCfg, err := cfg.GetChartVersion(minor)
	if err != nil {
		return fmt.Errorf("failed to get chart version: %w", err)
	}

	// Extract Rancher version from rancher-branch (e.g., "release/v2.15" -> "2.15.x")
	rancherVersion := extractRancherVersion(chartCfg.RancherBranch)

	// Get commits from lock file
	chartLock, exists := lock.ChartVersions[minor]
	if !exists {
		return fmt.Errorf("no lock data found for Rancher minor %s", minor)
	}

	// Prepare template data with BOTH dev and prod configurations
	data := TemplateData{
		BciVersion:     cfg.BaseImage.BciVersion,
		RancherMinor:   minor,
		RancherVersion: rancherVersion,
		ClusterRepos:   cfg.ClusterRepos,

		// Dev configuration
		DevChartsBranch:  chartCfg.Dev.ChartsBranch,
		DevChartCommit:   chartLock.UpstreamRefs.Dev.Charts.Commit,
		DevPartnerBranch: chartCfg.Dev.PartnerBranch,
		DevPartnerCommit: chartLock.UpstreamRefs.Dev.Partner.Commit,
		DevRke2Branch:    chartCfg.Dev.Rke2Branch,
		DevRke2Commit:    chartLock.UpstreamRefs.Dev.Rke2.Commit,

		// Prod configuration
		ProdChartsBranch:  chartCfg.Prod.ChartsBranch,
		ProdChartCommit:   chartLock.UpstreamRefs.Prod.Charts.Commit,
		ProdPartnerBranch: chartCfg.Prod.PartnerBranch,
		ProdPartnerCommit: chartLock.UpstreamRefs.Prod.Partner.Commit,
		ProdRke2Branch:    chartCfg.Prod.Rke2Branch,
		ProdRke2Commit:    chartLock.UpstreamRefs.Prod.Rke2.Commit,
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate PROD Dockerfile
	if err := generateDockerfile(outputDir, minor, "", data, DockerfileProdTemplate); err != nil {
		return fmt.Errorf("failed to generate prod Dockerfile: %w", err)
	}

	// Generate DEV Dockerfile
	if err := generateDockerfile(outputDir, minor, "-dev", data, DockerfileDevTemplate); err != nil {
		return fmt.Errorf("failed to generate dev Dockerfile: %w", err)
	}

	return nil
}

// generateDockerfile renders a template to a file
func generateDockerfile(outputDir, minor, suffix string, data TemplateData, tmplStr string) error {
	// Parse template
	tmpl, err := template.New("dockerfile").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse Dockerfile template: %w", err)
	}

	// Create output file
	outputPath := filepath.Join(outputDir, fmt.Sprintf("Dockerfile.%s%s", minor, suffix))
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() {
		err = file.Close()
	}()

	// Render template
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to render Dockerfile template: %w", err)
	}

	return nil
}

// extractRancherVersion extracts version from rancher-branch
// e.g., "release/v2.15" -> "2.15.x"
func extractRancherVersion(branch string) string {
	// Simple extraction - can be enhanced if needed
	if len(branch) > 9 && branch[:9] == "release/v" {
		return branch[9:] + ".x"
	}
	return "unknown"
}

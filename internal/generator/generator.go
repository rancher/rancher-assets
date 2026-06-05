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
	BciVersion           string
	DefaultChartsBranch  string
	DefaultPartnerBranch string
	DefaultRke2Branch    string
	ChartCommit          string
	PartnerCommit        string
	Rke2Commit           string
	ClusterRepos         map[string]config.ClusterRepoConfig
	ChartMajor           string
	RancherVersion       string
}

// Generate creates a Dockerfile for the specified chart major version
func Generate(cfg *config.Config, lock *lockfile.Lock, major string, outputDir string) error {
	// Get chart version config
	chartCfg, err := cfg.GetChartVersion(major)
	if err != nil {
		return err
	}

	// Use dev branches as defaults (most common for local development)
	// CI will override these based on tag format
	buildCfg := chartCfg.Dev

	// Extract Rancher version from rancher-branch (e.g., "release/v2.15" -> "2.15.x")
	rancherVersion := extractRancherVersion(chartCfg.RancherBranch)

	// Get commits from lock file (use dev refs as defaults in Dockerfile)
	var chartCommit, partnerCommit, rke2Commit string
	if chartLock, exists := lock.ChartVersions[major]; exists {
		chartCommit = chartLock.UpstreamRefs.Dev.Charts.Commit
		partnerCommit = chartLock.UpstreamRefs.Dev.Partner.Commit
		rke2Commit = chartLock.UpstreamRefs.Dev.Rke2.Commit
	}

	// Use placeholder if commits not available (shouldn't happen after generation)
	if chartCommit == "" {
		chartCommit = "HEAD"
	}
	if partnerCommit == "" {
		partnerCommit = "HEAD"
	}
	if rke2Commit == "" {
		rke2Commit = "HEAD"
	}

	// Prepare template data
	data := TemplateData{
		BciVersion:           cfg.BaseImage.BciVersion,
		DefaultChartsBranch:  buildCfg.ChartsBranch,
		DefaultPartnerBranch: buildCfg.PartnerBranch,
		DefaultRke2Branch:    buildCfg.Rke2Branch,
		ChartCommit:          chartCommit,
		PartnerCommit:        partnerCommit,
		Rke2Commit:           rke2Commit,
		ClusterRepos:         cfg.ClusterRepos,
		ChartMajor:           major,
		RancherVersion:       rancherVersion,
	}

	// Parse template
	tmpl, err := template.New("dockerfile").Parse(DockerfileTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse Dockerfile template: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	outputPath := filepath.Join(outputDir, fmt.Sprintf("Dockerfile.%s", major))
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

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

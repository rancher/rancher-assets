package lockfile

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Lock represents the complete lock file state
type Lock struct {
	ChartVersions    map[string]ChartVersionLock `yaml:"chart-versions"`
	BaseImage        BaseImageLock               `yaml:"base-image"`
	CopyScriptHash   string                      `yaml:"copy-script-hash"`
	TemplateHashes   TemplateHashes              `yaml:"template-hashes"`
	GeneratedAt      *time.Time                  `yaml:"generated-at"`
	GeneratorVersion string                      `yaml:"generator-version"`
}

// BaseImageLock tracks base image versions used for Dockerfile generation
type BaseImageLock struct {
	BciBase  string `yaml:"bci-base"`
	BciMicro string `yaml:"bci-micro"`
}

// TemplateHashes tracks hashes of template files for reproducibility
type TemplateHashes struct {
	ProdTemplate string `yaml:"prod-template"`
	DevTemplate  string `yaml:"dev-template"`
}

// ChartVersionLock tracks state for a single Rancher minor version
type ChartVersionLock struct {
	LatestStable     *string             `yaml:"latest-stable"`
	LatestPrerelease *string             `yaml:"latest-prerelease"`
	UpdatedAt        *time.Time          `yaml:"updated-at"`
	UpstreamRefs     UpstreamRefsByBuild `yaml:"upstream-refs"`
}

// UpstreamRefsByBuild separates prod and dev upstream refs
type UpstreamRefsByBuild struct {
	Prod UpstreamRefsSet `yaml:"prod"`
	Dev  UpstreamRefsSet `yaml:"dev"`
}

// UpstreamRefsSet holds refs for charts, partner, and rke2
type UpstreamRefsSet struct {
	Charts  UpstreamRef `yaml:"charts"`
	Partner UpstreamRef `yaml:"partner"`
	Rke2    UpstreamRef `yaml:"rke2"`
}

// UpstreamRef tracks an upstream repository reference
type UpstreamRef struct {
	FetchedAt *time.Time `yaml:"fetched-at"`
	Branch    string     `yaml:"branch"`
	Commit    string     `yaml:"commit"`
}

// generatorVersion should be bumped if any major changes to the generator/hash/lock are made.
const generatorVersion = "v0.1.0"

// Load reads and parses the lock.yaml file
// If the file doesn't exist, returns an empty lock structure
func Load(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty lock structure
			return &Lock{
				ChartVersions:    make(map[string]ChartVersionLock),
				GeneratorVersion: generatorVersion,
			}, nil
		}
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	var lock Lock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("failed to parse lock file: %w", err)
	}

	return &lock, nil
}

// Save writes the lock file to disk
func (l *Lock) Save(path string) error {
	now := time.Now().UTC()
	l.GeneratedAt = &now
	l.GeneratorVersion = generatorVersion

	data, err := yaml.Marshal(l)
	if err != nil {
		return fmt.Errorf("failed to marshal lock file: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	return nil
}

// UpdateUpstreamRefs updates upstream commit references for a Rancher minor version
func (l *Lock) UpdateUpstreamRefs(major string, prodRefs, devRefs UpstreamRefsSet) error {
	chartLock, exists := l.ChartVersions[major]
	if !exists {
		// Initialize new chart version lock entry
		chartLock = ChartVersionLock{
			UpstreamRefs: UpstreamRefsByBuild{},
		}
	}

	// Update prod and dev refs
	chartLock.UpstreamRefs.Prod = prodRefs
	chartLock.UpstreamRefs.Dev = devRefs

	l.ChartVersions[major] = chartLock
	return nil
}

// EnsureChartVersion ensures a chart version entry exists in the lock
func (l *Lock) EnsureChartVersion(major string) {
	if _, exists := l.ChartVersions[major]; !exists {
		l.ChartVersions[major] = ChartVersionLock{
			UpstreamRefs: UpstreamRefsByBuild{},
		}
	}
}

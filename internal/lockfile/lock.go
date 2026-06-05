package lockfile

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const GeneratorVersion = "v0.1.0"

// Load reads and parses the lock.yaml file
// If the file doesn't exist, returns an empty lock structure
func Load(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty lock structure
			return &Lock{
				ChartVersions:    make(map[string]ChartVersionLock),
				GeneratorVersion: GeneratorVersion,
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
	l.GeneratorVersion = GeneratorVersion

	data, err := yaml.Marshal(l)
	if err != nil {
		return fmt.Errorf("failed to marshal lock file: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	return nil
}

// UpdateUpstreamRefs updates upstream commit references for a chart major version
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

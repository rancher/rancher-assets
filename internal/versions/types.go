package versions

import "time"

// VersionsFile represents the versions.yaml structure on the orphan branch
type VersionsFile struct {
	ChartMajors map[string]ChartMajorVersions `yaml:",inline"`
}

// ChartMajorVersions tracks stable and prerelease versions for a chart major
type ChartMajorVersions struct {
	Stable     VersionInfo `yaml:"stable"`
	Prerelease VersionInfo `yaml:"prerelease"`
}

// VersionInfo contains version tag and timestamp
type VersionInfo struct {
	Tag       string    `yaml:"tag"`
	UpdatedAt time.Time `yaml:"updated-at"`
}

// ReleasePlan is a JSON plan output for workflows
type ReleasePlan struct {
	Major             string `json:"major"`
	CurrentStable     string `json:"current_stable"`
	CurrentPrerelease string `json:"current_prerelease"`
	NewVersion        string `json:"new_version"`
}

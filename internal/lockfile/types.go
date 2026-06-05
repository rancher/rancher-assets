package lockfile

import "time"

// Lock represents the complete lock file state
type Lock struct {
	ChartVersions    map[string]ChartVersionLock `yaml:"chart-versions"`
	CopyScriptHash   string                      `yaml:"copy-script-hash"`
	GeneratedAt      *time.Time                  `yaml:"generated-at"`
	GeneratorVersion string                      `yaml:"generator-version"`
}

// ChartVersionLock tracks state for a single chart major version
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
	Branch    string     `yaml:"branch"`
	Commit    string     `yaml:"commit"`
	FetchedAt *time.Time `yaml:"fetched-at"`
}

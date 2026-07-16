package versions

// ReleasePlan is a JSON plan output for workflows
// No longer needs versions.yaml - CalVer timestamps are self-generating
type ReleasePlan struct {
	RancherMinor string `json:"rancher_minor"` // e.g., "2.14", "2.15"
	ReleaseType  string `json:"release_type"`  // "stable" or "prerelease"
	Version      string `json:"version"`       // CalVer timestamp: v2.15-20260716T1430Z[-dev]
}

package versions

import (
	"fmt"
	"regexp"
	"time"
)

// ReleasePlan is a JSON plan output for workflows
// No longer needs versions.yaml - CalVer timestamps are self-generating
type ReleasePlan struct {
	RancherMinor string `json:"rancher_minor"` // e.g., "2.14", "2.15"
	ReleaseType  string `json:"release_type"`  // "stable" or "prerelease"
	Version      string `json:"version"`       // CalVer timestamp: v2.15-20260716T1430Z[-dev]
}

// GenerateCalVerTimestamp creates a CalVer version string with UTC timestamp
// Format: v{RANCHER_MINOR}-{YYYYMMDD}T{HHMM}Z[-dev]
func GenerateCalVerTimestamp(rancherMinor, releaseType string) string {
	now := time.Now().UTC()
	timestamp := now.Format("20060102T1504Z") // ISO 8601: YYYYMMDDThhmmZ

	version := fmt.Sprintf("v%s-%s", rancherMinor, timestamp)
	if releaseType == "prerelease" {
		version += "-dev"
	}
	return version
}

// ParseCalVerTag extracts Rancher minor and release type from a CalVer tag
// Examples:
//
//	v2.14-20260716T1430Z     -> ("2.14", "stable", nil)
//	v2.15-20260805T1600Z-dev -> ("2.15", "prerelease", nil)
func ParseCalVerTag(tag string) (rancherMinor, releaseType string, err error) {
	// Regex: v(\d+\.\d+)-\d{8}T\d{4}Z(-dev)?
	re := regexp.MustCompile(`^v(\d+\.\d+)-\d{8}T\d{4}Z(-dev)?$`)
	matches := re.FindStringSubmatch(tag)
	if matches == nil {
		return "", "", fmt.Errorf("invalid CalVer tag format: %s (expected: v{MINOR}-{YYYYMMDD}T{HHMM}Z[-dev])", tag)
	}

	rancherMinor = matches[1] // "2.14" or "2.15"
	if matches[2] == "-dev" {
		releaseType = "prerelease"
	} else {
		releaseType = "stable"
	}
	return rancherMinor, releaseType, nil
}

// PlanAutoPrerelease generates CalVer prerelease versions for changed Rancher minors
// No version arithmetic needed - timestamps are self-generating
func PlanAutoPrerelease(changedMinors []string) ([]ReleasePlan, error) {
	plans := make([]ReleasePlan, 0, len(changedMinors))
	for _, minor := range changedMinors {
		version := GenerateCalVerTimestamp(minor, "prerelease")
		plans = append(plans, ReleasePlan{
			RancherMinor: minor,
			ReleaseType:  "prerelease",
			Version:      version,
		})
	}
	return plans, nil
}

// PlanManualRelease generates CalVer versions for manual releases
// No bump_type needed - timestamp differentiates versions
func PlanManualRelease(rancherMinors []string, releaseType string) ([]ReleasePlan, error) {
	if releaseType != "stable" && releaseType != "prerelease" {
		return nil, fmt.Errorf("invalid release type: %s (must be 'stable' or 'prerelease')", releaseType)
	}

	var plans []ReleasePlan
	for _, minor := range rancherMinors {
		version := GenerateCalVerTimestamp(minor, releaseType)
		plans = append(plans, ReleasePlan{
			RancherMinor: minor,
			ReleaseType:  releaseType,
			Version:      version,
		})
	}
	return plans, nil
}

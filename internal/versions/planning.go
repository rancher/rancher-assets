package versions

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadVersionsFile loads versions.yaml from a file path
func LoadVersionsFile(path string) (*VersionsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions file: %w", err)
	}

	var vf VersionsFile
	if err := yaml.Unmarshal(data, &vf); err != nil {
		return nil, fmt.Errorf("failed to parse versions file: %w", err)
	}

	return &vf, nil
}

// PlanAutoPrerelease plans automatic prerelease bumps for changed majors
func PlanAutoPrerelease(vf *VersionsFile, changedMajors []string) ([]ReleasePlan, error) {
	var plans []ReleasePlan

	for _, major := range changedMajors {
		mv, exists := vf.ChartMajors[major]
		if !exists {
			return nil, fmt.Errorf("chart major %s not found in versions file", major)
		}

		currentStable := mv.Stable.Tag
		currentPrerelease := mv.Prerelease.Tag

		newVersion, err := calculateAutoPrerelease(major, currentStable, currentPrerelease)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate auto prerelease for %s: %w", major, err)
		}

		plans = append(plans, ReleasePlan{
			Major:             major,
			CurrentStable:     currentStable,
			CurrentPrerelease: currentPrerelease,
			NewVersion:        newVersion,
		})
	}

	return plans, nil
}

// PlanManualRelease plans manual release bumps
func PlanManualRelease(vf *VersionsFile, majors []string, bumpType, releaseType string) ([]ReleasePlan, error) {
	var plans []ReleasePlan

	for _, major := range majors {
		mv, exists := vf.ChartMajors[major]
		if !exists {
			return nil, fmt.Errorf("chart major %s not found in versions file", major)
		}

		currentStable := mv.Stable.Tag
		currentPrerelease := mv.Prerelease.Tag

		newVersion, err := calculateManualRelease(major, currentStable, currentPrerelease, bumpType, releaseType)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate manual release for %s: %w", major, err)
		}

		plans = append(plans, ReleasePlan{
			Major:             major,
			CurrentStable:     currentStable,
			CurrentPrerelease: currentPrerelease,
			NewVersion:        newVersion,
		})
	}

	return plans, nil
}

// calculateAutoPrerelease determines next prerelease version for auto workflow
func calculateAutoPrerelease(major, currentStable, currentPrerelease string) (string, error) {
	// If we have a current prerelease
	if currentPrerelease != "" {
		// Check if prerelease is for current stable version
		if currentStable != "" && strings.HasPrefix(currentPrerelease, currentStable+"-rc.") {
			// Bump RC number
			rc, err := extractRCNumber(currentPrerelease)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s-rc.%d", currentStable, rc+1), nil
		}

		// Prerelease is for next version - bump RC
		base := stripRCSuffix(currentPrerelease)
		rc, err := extractRCNumber(currentPrerelease)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s-rc.%d", base, rc+1), nil
	}

	// No current prerelease - create first RC for next patch
	if currentStable == "" {
		// No stable yet - start with v*.0.0-rc.1
		return fmt.Sprintf("%s.0.0-rc.1", major), nil
	}

	// Bump patch and add -rc.1
	majorNum, minorNum, patchNum, err := parseVersion(currentStable)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("v%d.%d.%d-rc.1", majorNum, minorNum, patchNum+1), nil
}

// calculateManualRelease determines next version for manual workflow
func calculateManualRelease(major, currentStable, currentPrerelease, bumpType, releaseType string) (string, error) {
	// Determine base version
	baseVersion := currentStable
	if baseVersion == "" {
		baseVersion = fmt.Sprintf("%s.0.0", major)
	}

	// Parse base version
	majorNum, minorNum, patchNum, err := parseVersion(baseVersion)
	if err != nil {
		return "", err
	}

	// Calculate target version after bump
	var targetVersion string
	if bumpType == "minor" {
		targetVersion = fmt.Sprintf("v%d.%d.0", majorNum, minorNum+1)
	} else { // patch
		targetVersion = fmt.Sprintf("v%d.%d.%d", majorNum, minorNum, patchNum+1)
	}

	// For prereleases, check if we should increment RC or start new series
	if releaseType == "prerelease" {
		if currentPrerelease != "" {
			currentPrereleaseBase := stripRCSuffix(currentPrerelease)

			if currentPrereleaseBase == targetVersion {
				// Same target version - increment RC number
				rc, err := extractRCNumber(currentPrerelease)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%s-rc.%d", targetVersion, rc+1), nil
			}

			// Different target version - start new RC series
			return fmt.Sprintf("%s-rc.1", targetVersion), nil
		}

		// No current prerelease - start new RC series
		return fmt.Sprintf("%s-rc.1", targetVersion), nil
	}

	// Stable release - just use target version
	return targetVersion, nil
}

// parseVersion parses a semantic version string (e.g., "v1.2.3")
func parseVersion(version string) (major, minor, patch int, err error) {
	re := regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(version)
	if matches == nil {
		return 0, 0, 0, fmt.Errorf("invalid version format: %s", version)
	}

	major, _ = strconv.Atoi(matches[1])
	minor, _ = strconv.Atoi(matches[2])
	patch, _ = strconv.Atoi(matches[3])
	return major, minor, patch, nil
}

// stripRCSuffix removes the -rc.N suffix from a version
func stripRCSuffix(version string) string {
	re := regexp.MustCompile(`^(.+)-rc\.\d+$`)
	matches := re.FindStringSubmatch(version)
	if matches == nil {
		return version
	}
	return matches[1]
}

// extractRCNumber extracts the RC number from a version like "v1.0.0-rc.2"
func extractRCNumber(version string) (int, error) {
	re := regexp.MustCompile(`-rc\.(\d+)$`)
	matches := re.FindStringSubmatch(version)
	if matches == nil {
		return 0, fmt.Errorf("no RC number found in version: %s", version)
	}
	return strconv.Atoi(matches[1])
}

package lockfile

import (
	"fmt"
	"os/exec"

	"gopkg.in/yaml.v3"
)

// ChangedMajors compares two lock files and returns chart majors with upstream ref changes.
// Ignores timestamp changes (fetched-at) and only reports actual commit changes.
func ChangedMajors(fromCommit, toCommit string) ([]string, error) {
	// Load lock files from commits
	fromLock, err := loadLockFromCommit(fromCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to load lock from %s: %w", fromCommit, err)
	}

	toLock, err := loadLockFromCommit(toCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to load lock from %s: %w", toCommit, err)
	}

	// Find changed majors
	var changed []string

	for major, toLockData := range toLock.ChartVersions {
		fromLockData, existedBefore := fromLock.ChartVersions[major]

		if !existedBefore {
			// New major added
			changed = append(changed, major)
			continue
		}

		// Compare upstream refs (prod and dev)
		if upstreamRefsChanged(fromLockData.UpstreamRefs, toLockData.UpstreamRefs) {
			changed = append(changed, major)
		}
	}

	return changed, nil
}

// upstreamRefsChanged checks if upstream refs have actual commit changes
// (ignores fetched-at timestamp differences)
func upstreamRefsChanged(from, to UpstreamRefsByBuild) bool {
	// Check prod refs
	if refSetChanged(from.Prod, to.Prod) {
		return true
	}

	// Check dev refs
	if refSetChanged(from.Dev, to.Dev) {
		return true
	}

	return false
}

// refSetChanged checks if any commit in the ref set changed
func refSetChanged(from, to UpstreamRefsSet) bool {
	if from.Charts.Commit != to.Charts.Commit {
		return true
	}
	if from.Partner.Commit != to.Partner.Commit {
		return true
	}
	if from.Rke2.Commit != to.Rke2.Commit {
		return true
	}
	return false
}

// loadLockFromCommit loads lock.yaml from a git commit
func loadLockFromCommit(commit string) (*Lock, error) {
	// Run: git show <commit>:lock.yaml
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:lock.yaml", commit))
	output, err := cmd.Output()
	if err != nil {
		// If lock.yaml doesn't exist at this commit, return empty lock
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return &Lock{ChartVersions: make(map[string]ChartVersionLock)}, nil
		}
		return nil, fmt.Errorf("git show failed: %w", err)
	}

	var lock Lock
	if err := yaml.Unmarshal(output, &lock); err != nil {
		return nil, fmt.Errorf("failed to parse lock.yaml: %w", err)
	}

	return &lock, nil
}

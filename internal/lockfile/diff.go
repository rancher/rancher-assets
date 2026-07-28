package lockfile

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"gopkg.in/yaml.v3"
)

// ChangedMajors compares two lock files and returns Rancher minors with upstream ref changes.
// Ignores timestamp changes (fetched-at) and only reports actual commit changes.
func ChangedMajors(ctx context.Context, fromCommit, toCommit string) ([]string, error) {
	// Load lock files from commits
	fromLock, err := loadLockFromCommit(ctx, fromCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to load lock from %s: %w", fromCommit, err)
	}

	toLock, err := loadLockFromCommit(ctx, toCommit)
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
	if from.RKE2.Commit != to.RKE2.Commit {
		return true
	}
	return false
}

// loadLockFromCommit loads lock.yaml from a git commit
func loadLockFromCommit(ctx context.Context, commit string) (*Lock, error) {
	// Run: git show <commit>:lock.yaml
	cmd := exec.CommandContext(ctx, "git", "show", commit+":lock.yaml")
	output, err := cmd.Output()
	if err != nil {
		// If lock.yaml doesn't exist at this commit, return empty lock
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
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

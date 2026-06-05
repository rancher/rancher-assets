package lockfile

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// QueryUpstreamCommit queries the latest commit SHA for a given repo and branch
func QueryUpstreamCommit(repoURL string, branch string) (string, error) {
	// Use git ls-remote to get the latest commit for the branch
	cmd := exec.Command("git", "ls-remote", repoURL, fmt.Sprintf("refs/heads/%s", branch))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to query upstream %s @ %s: %w", repoURL, branch, err)
	}

	// Output format: "<commit-sha>\trefs/heads/<branch>"
	parts := strings.Fields(string(output))
	if len(parts) < 1 {
		return "", fmt.Errorf("unexpected output from git ls-remote for %s @ %s", repoURL, branch)
	}

	commitSHA := parts[0]
	return commitSHA, nil
}

// QueryUpstreamRef queries upstream and returns an UpstreamRef
func QueryUpstreamRef(repoURL string, branch string) (UpstreamRef, error) {
	commit, err := QueryUpstreamCommit(repoURL, branch)
	if err != nil {
		return UpstreamRef{}, err
	}

	now := time.Now().UTC()
	return UpstreamRef{
		Branch:    branch,
		Commit:    commit,
		FetchedAt: &now,
	}, nil
}

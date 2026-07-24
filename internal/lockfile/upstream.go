package lockfile

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// QueryUpstreamCommit queries the latest commit SHA for a given repo and branch
func QueryUpstreamCommit(ctx context.Context, repoURL, branch string) (string, error) {
	// Use git ls-remote to get the latest commit for the branch
	cmd := exec.CommandContext(ctx, "git", "ls-remote", repoURL, "refs/heads/"+branch)
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
func QueryUpstreamRef(ctx context.Context, repoURL, branch string) (UpstreamRef, error) {
	commit, err := QueryUpstreamCommit(ctx, repoURL, branch)
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

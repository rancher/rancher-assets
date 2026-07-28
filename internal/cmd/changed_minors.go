package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"sort"

	"github.com/rancher/rancher-assets/internal/lockfile"
	"github.com/rancher/rancher-assets/internal/logger"
)

// ChangedMinors will print the list of minor versions with changes
func ChangedMinors(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("changed-minors", flag.ExitOnError)
	fromCommit := fs.String("from", "", "From commit (required)")
	toCommit := fs.String("to", "", "To commit (required)")
	verbose := fs.Bool("verbose", false, "Show detailed change information")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse command flags: %w", err)
	}

	if *fromCommit == "" || *toCommit == "" {
		return errors.New("both --from and --to are required")
	}

	changed, err := lockfile.ChangedMajors(ctx, *fromCommit, *toCommit)
	if err != nil {
		return fmt.Errorf("failed to identify changed versions from lock file: %w", err)
	}

	if changed == nil {
		changed = []string{}
	}

	sort.Strings(changed) // Consistent output order

	if *verbose {
		if len(changed) == 0 {
			logger.Info("No Rancher minors with upstream ref changes detected")
			logger.Info("(Only timestamp changes in lock.yaml)")
		} else {
			logger.Info("Changed Rancher minors (%d):", len(changed))
			for _, minor := range changed {
				logger.Info("  - %s", minor)
			}
		}
	}

	// Always output JSON array (for workflow consumption)
	output, err := json.Marshal(changed)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	logger.Println(string(output))
	return nil
}

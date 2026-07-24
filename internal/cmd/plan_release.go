package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/rancher/rancher-assets/internal/logger"
	"github.com/rancher/rancher-assets/internal/versions"
)

func PlanRelease(ctx context.Context, args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("plan-release", flag.ExitOnError)
	planType := fs.String("type", "", "Plan type: auto or manual (required)")
	changedMinorsJSON := fs.String("changed-minors", "", "JSON array of changed Rancher minors (for auto)")
	minorsJSON := fs.String("minors", "", "JSON array of Rancher minors to release (for manual)")
	releaseType := fs.String("release", "", "Release type: stable or prerelease (for manual)")
	verbose := fs.Bool("verbose", false, "Show detailed output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *planType == "" {
		return errors.New("--type is required")
	}

	var plans []versions.ReleasePlan
	var err error

	switch *planType {
	case "auto":
		if *changedMinorsJSON == "" {
			return errors.New("--changed-minors is required for auto type")
		}

		var changedMinors []string
		if err := json.Unmarshal([]byte(*changedMinorsJSON), &changedMinors); err != nil {
			return fmt.Errorf("failed to parse changed-minors JSON: %w", err)
		}

		plans, err = versions.PlanAutoPrerelease(changedMinors)
		if err != nil {
			return err
		}

	case "manual":
		if *minorsJSON == "" || *releaseType == "" {
			return errors.New("--minors and --release are required for manual type")
		}

		var minors []string
		if err := json.Unmarshal([]byte(*minorsJSON), &minors); err != nil {
			return fmt.Errorf("failed to parse minors JSON: %w", err)
		}

		plans, err = versions.PlanManualRelease(minors, *releaseType)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("invalid type: %s (must be auto or manual)", *planType)
	}

	// Verbose output
	if *verbose {
		if len(plans) == 0 {
			logger.Info("No releases planned")
		} else {
			logger.Info("Planned releases (%d):", len(plans))
			for _, plan := range plans {
				logger.Info("  %s: %s (%s)",
					plan.RancherMinor, plan.Version, plan.ReleaseType)
			}
		}
	}

	// Always output JSON for workflow consumption
	output, err := json.Marshal(plans)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	logger.Println(string(output))
	return nil
}

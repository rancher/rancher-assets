//nolint:cyclop // Package average affected by Generate function's linear workflow
package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/rancher/rancher-assets/internal/config"
	"github.com/rancher/rancher-assets/internal/logger"
)

// ListMinors will print the Rancher minor versions found in the config
func ListMinors(ctx context.Context, args []string) error {
	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get all Rancher minors
	minors := cfg.ListRancherMinors()

	// Sort for consistent output
	sort.Strings(minors)

	// Output each minor on its own line
	for _, minor := range minors {
		logger.Println(minor)
	}

	return nil
}

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
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	minors := cfg.ListRancherMinors()

	sort.Strings(minors) // Consistent output order

	for _, minor := range minors {
		logger.Println(minor)
	}

	return nil
}

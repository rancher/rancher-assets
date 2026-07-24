package cmd

import (
	"context"
	"errors"
	"flag"

	"github.com/rancher/rancher-assets/internal/logger"
	"github.com/rancher/rancher-assets/internal/versions"
)

func CalverDevVersion(ctx context.Context, args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("calver-dev-version", flag.ExitOnError)
	minor := fs.String("minor", "", "Rancher minor (e.g., 2.15)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *minor == "" {
		return errors.New("--minor is required")
	}

	// Generate CalVer dev version
	version := versions.GenerateCalVerTimestamp(*minor, "prerelease")
	logger.Println(version)

	return nil
}

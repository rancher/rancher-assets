// Package main is the cli root command
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rancher/rancher-assets/internal/cmd"
	"github.com/rancher/rancher-assets/internal/logger"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	command := os.Args[1]

	switch command {
	case "generate":
		if err := cmd.Generate(ctx, os.Args[2:]); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "changed-minors":
		if err := cmd.ChangedMinors(ctx, os.Args[2:]); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "list-minors":
		if err := cmd.ListMinors(ctx, os.Args[2:]); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "export-images":
		if err := cmd.ExportImages(ctx, os.Args[2:]); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	default:
		logger.Error("Unknown command: %s", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	logger.Println("Usage: rancher-assets <command>")
	logger.Println("")
	logger.Println("Commands:")
	logger.Println("  generate             Generate Dockerfiles from lock.yaml")
	logger.Println("                       Flags: --update (query upstream repos and update lock.yaml)")
	logger.Println("  changed-minors       Detect Rancher minors with upstream ref changes")
	logger.Println("                       Flags: --from=<commit> --to=<commit>")
	logger.Println("  list-minors          List all Rancher minors from config.yaml")
	logger.Println("  export-images        Generate image lists from chart catalogs")
	logger.Println("                       Flags: --charts-path=<path> --version=<version> --output-dir=<path>")
}

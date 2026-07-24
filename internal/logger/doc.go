// Package logger provides a lightweight logging interface for the rancher-assets CLI tool.
//
// The logger supports different output levels (Info, Warn, Error, Success, Debug) and provides
// specialized methods for CLI-specific patterns like inline status updates and data output.
//
// # Output Destinations
//
// - Info, Success, Warn, Debug, Progress: stdout
// - Error: stderr
// - Print, Println: stdout (for data/JSON output)
//
// # Quiet and Verbose Modes
//
// The logger supports two modes to control output verbosity:
//
// - Quiet mode: Suppresses Info, Success, and Progress messages. Errors and Warnings still appear.
// - Verbose mode: Shows Debug messages in addition to all other output.
//
// # Usage Examples
//
//	// Basic informational output
//	logger.Info("Loading configuration...")
//	logger.Info("  Found %d items", count)
//
//	// Success and warnings with symbols
//	logger.Success("Operation complete!")
//	logger.Warn("Missing configuration file, using defaults")
//
//	// Errors (go to stderr)
//	logger.Error("failed to load file: %v", err)
//
//	// Inline status updates (start a message, complete it later)
//	logger.StartProgress("  Querying upstream... ")
//	// ... do work ...
//	logger.CompleteProgress("OK")  // or logger.CompleteProgress("FAILED (%v)", err)
//
//	// Data output (never suppressed by quiet mode)
//	logger.Println(jsonString)
//	logger.Print("%s", result)
//
//	// Debug output (only shown in verbose mode)
//	logger.Debug("cache hit for key: %s", key)
//
// # Global Logger
//
// The package provides a global Default logger instance that writes to os.Stdout and os.Stderr.
// Package-level functions (Info, Warn, etc.) use this default instance for convenience.
//
// For testing or custom output, create a new logger instance:
//
//	customLogger := logger.New(customStdout, customStderr)
//	customLogger.Info("test message")
package logger

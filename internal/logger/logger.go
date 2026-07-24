package logger

import (
	"fmt"
	"io"
	"os"
)

// Logger handles structured output for the CLI tool
type Logger struct {
	stdout  io.Writer
	stderr  io.Writer
	quiet   bool
	verbose bool
}

// Default is the global default logger instance
var Default = New(os.Stdout, os.Stderr)

// New creates a new logger with specified output streams
func New(stdout, stderr io.Writer) *Logger {
	return &Logger{
		stdout: stdout,
		stderr: stderr,
	}
}

// SetQuiet enables quiet mode (suppresses Info, Success, and Progress output)
func (l *Logger) SetQuiet(quiet bool) {
	l.quiet = quiet
}

// SetVerbose enables verbose mode (shows Debug output)
func (l *Logger) SetVerbose(verbose bool) {
	l.verbose = verbose
}

// Info prints informational messages to stdout (suppressed in quiet mode)
func (l *Logger) Info(format string, args ...any) {
	if l.quiet {
		return
	}
	//nolint:errcheck // Logger output errors are not actionable
	fmt.Fprintf(l.stdout, format+"\n", args...)
}

// Success prints success messages to stdout with ✅ symbol (suppressed in quiet mode)
func (l *Logger) Success(format string, args ...any) {
	if l.quiet {
		return
	}
	//nolint:errcheck // Logger output errors are not actionable
	fmt.Fprintf(l.stdout, "✅ "+format+"\n", args...)
}

// Warn prints warning messages to stdout with ⚠️ symbol (always shown, not suppressed by quiet)
func (l *Logger) Warn(format string, args ...any) {
	//nolint:errcheck // Logger output errors are not actionable
	fmt.Fprintf(l.stdout, "⚠️  "+format+"\n", args...)
}

// Error prints error messages to stderr with "Error: " prefix (always shown)
func (l *Logger) Error(format string, args ...any) {
	//nolint:errcheck // Logger output errors are not actionable
	fmt.Fprintf(l.stderr, "Error: "+format+"\n", args...)
}

// Debug prints debug messages to stdout only when verbose mode is enabled
func (l *Logger) Debug(format string, args ...any) {
	if !l.verbose {
		return
	}
	//nolint:errcheck // Logger output errors are not actionable
	fmt.Fprintf(l.stdout, "[DEBUG] "+format+"\n", args...)
}

// StartProgress prints a message to stdout without a newline for inline status updates
// Use CompleteProgress to finish the line (suppressed in quiet mode)
func (l *Logger) StartProgress(format string, args ...any) {
	if l.quiet {
		return
	}
	//nolint:errcheck // Logger output errors are not actionable
	fmt.Fprintf(l.stdout, format, args...)
}

// CompleteProgress completes an inline status message started with StartProgress
// Adds a newline at the end (suppressed in quiet mode)
func (l *Logger) CompleteProgress(format string, args ...any) {
	if l.quiet {
		return
	}
	//nolint:errcheck // Logger output errors are not actionable
	fmt.Fprintf(l.stdout, format+"\n", args...)
}

// Print writes to stdout without any formatting or newline
// Never suppressed by quiet mode - use for data output like JSON
func (l *Logger) Print(format string, args ...any) {
	//nolint:errcheck // Logger output errors are not actionable
	fmt.Fprintf(l.stdout, format, args...)
}

// Println writes to stdout with a newline
// Never suppressed by quiet mode - use for data output like JSON or lists
func (l *Logger) Println(args ...any) {
	//nolint:errcheck // Logger output errors are not actionable
	fmt.Fprintln(l.stdout, args...)
}

// Package-level convenience functions that use the Default logger

// Info prints informational messages using the default logger
func Info(format string, args ...any) {
	Default.Info(format, args...)
}

// Success prints success messages using the default logger
func Success(format string, args ...any) {
	Default.Success(format, args...)
}

// Warn prints warning messages using the default logger
func Warn(format string, args ...any) {
	Default.Warn(format, args...)
}

// Error prints error messages using the default logger
func Error(format string, args ...any) {
	Default.Error(format, args...)
}

// Debug prints debug messages using the default logger
func Debug(format string, args ...any) {
	Default.Debug(format, args...)
}

// StartProgress prints an inline status message using the default logger
func StartProgress(format string, args ...any) {
	Default.StartProgress(format, args...)
}

// CompleteProgress completes an inline status message using the default logger
func CompleteProgress(format string, args ...any) {
	Default.CompleteProgress(format, args...)
}

// Print writes to stdout using the default logger
func Print(format string, args ...any) {
	Default.Print(format, args...)
}

// Println writes to stdout with newline using the default logger
func Println(args ...any) {
	Default.Println(args...)
}

// SetQuiet enables quiet mode on the default logger
func SetQuiet(quiet bool) {
	Default.SetQuiet(quiet)
}

// SetVerbose enables verbose mode on the default logger
func SetVerbose(verbose bool) {
	Default.SetVerbose(verbose)
}

// Package ui provides user interface utilities for yak-box, including
// colored output functions that respect NO_COLOR environment variable and TTY detection.
package ui

import (
	"os"

	"github.com/fatih/color"
)

// Success prints a green-colored message to stderr.
// Respects NO_COLOR environment variable and TTY detection.
func Success(format string, args ...interface{}) {
	color.New(color.FgGreen).Fprintf(os.Stderr, format, args...)
}

// Warning prints a yellow-colored message to stderr.
// Respects NO_COLOR environment variable and TTY detection.
func Warning(format string, args ...interface{}) {
	color.New(color.FgYellow).Fprintf(os.Stderr, format, args...)
}

// Error prints a red-colored message to stderr.
// Respects NO_COLOR environment variable and TTY detection.
func Error(format string, args ...interface{}) {
	color.New(color.FgRed).Fprintf(os.Stderr, format, args...)
}

// Info prints a cyan-colored message to stderr.
// Respects NO_COLOR environment variable and TTY detection.
func Info(format string, args ...interface{}) {
	color.New(color.FgCyan).Fprintf(os.Stderr, format, args...)
}

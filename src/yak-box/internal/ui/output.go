// Package ui provides user interface utilities for yak-box, including
// colored output functions that respect NO_COLOR environment variable and TTY detection.
package ui

import (
	"os"

	"github.com/fatih/color"
)

func Success(format string, args ...interface{}) {
	color.New(color.FgGreen).Fprintf(os.Stderr, format, args...)
}

func Warning(format string, args ...interface{}) {
	color.New(color.FgYellow).Fprintf(os.Stderr, format, args...)
}

func Error(format string, args ...interface{}) {
	color.New(color.FgRed).Fprintf(os.Stderr, format, args...)
}

func Info(format string, args ...interface{}) {
	color.New(color.FgCyan).Fprintf(os.Stderr, format, args...)
}

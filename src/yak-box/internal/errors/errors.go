// Package errors provides a typed error system for exit code handling.
//
// This package defines specific error types that map to distinct exit codes,
// enabling consistent error handling and reporting throughout the application.
//
// Exit code conventions:
//   - 1: General runtime errors (e.g., file I/O, network issues)
//   - 2: Validation/usage errors (e.g., invalid flags, malformed input)
//
// Example usage:
//
//	if err := validateInput(input); err != nil {
//		return errors.NewValidationError("invalid input format", err)
//	}
//
//	if err := readFile(path); err != nil {
//		return errors.NewRuntimeError("failed to read file", err)
//	}
//
//	exitCode := errors.GetExitCode(err)
package errors

import (
	"errors"
	"fmt"
)

// ValidationError represents a validation or usage error.
// These errors indicate improper input or configuration and should result in exit code 2.
type ValidationError struct {
	Message string
	Cause   error
}

// Error implements the error interface for ValidationError.
func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap implements the error unwrapping interface for error chain inspection.
func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// RuntimeError represents a runtime error.
// These errors indicate failures during execution (I/O, network, etc.) and should result in exit code 1.
type RuntimeError struct {
	Message string
	Cause   error
}

// Error implements the error interface for RuntimeError.
func (e *RuntimeError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap implements the error unwrapping interface for error chain inspection.
func (e *RuntimeError) Unwrap() error {
	return e.Cause
}

// NewValidationError creates a new ValidationError with the given message and cause.
// Returns an error interface to support standard Go error handling.
func NewValidationError(msg string, cause error) error {
	return &ValidationError{
		Message: msg,
		Cause:   cause,
	}
}

// NewRuntimeError creates a new RuntimeError with the given message and cause.
// Returns an error interface to support standard Go error handling.
func NewRuntimeError(msg string, cause error) error {
	return &RuntimeError{
		Message: msg,
		Cause:   cause,
	}
}

// GetExitCode extracts the appropriate exit code from an error.
// Returns:
//   - 2 for ValidationError
//   - 1 for RuntimeError
//   - 1 for unknown errors
func GetExitCode(err error) int {
	var validationErr *ValidationError
	var runtimeErr *RuntimeError

	if errors.As(err, &validationErr) {
		return 2
	}
	if errors.As(err, &runtimeErr) {
		return 1
	}
	return 1
}

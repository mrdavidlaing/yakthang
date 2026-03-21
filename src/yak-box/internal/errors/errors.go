// Package errors provides typed errors that map to CLI exit codes (1=runtime, 2=validation).
package errors

import (
	"errors"
	"fmt"
	"strings"
)

// ValidationError maps to exit code 2 (improper input or configuration).
type ValidationError struct {
	Message string
	Cause   error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// RuntimeError maps to exit code 1 (I/O, network, and other execution failures).
type RuntimeError struct {
	Message string
	Cause   error
}

func (e *RuntimeError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *RuntimeError) Unwrap() error {
	return e.Cause
}

func NewValidationError(msg string, cause error) error {
	return &ValidationError{
		Message: msg,
		Cause:   cause,
	}
}

// CombineValidation aggregates multiple validation errors into a single ValidationError.
func CombineValidation(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	var b string
	for _, e := range errs {
		if b == "" {
			b = "Validation errors:\n"
		}
		b += fmt.Sprintf("  - %s\n", e.Error())
	}
	return NewValidationError(strings.TrimSuffix(b, "\n"), nil)
}

func NewRuntimeError(msg string, cause error) error {
	return &RuntimeError{
		Message: msg,
		Cause:   cause,
	}
}

// GetExitCode returns 2 for ValidationError, 1 for everything else.
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

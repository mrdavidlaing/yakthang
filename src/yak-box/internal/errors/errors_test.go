package errors

import (
	"errors"
	"testing"
)

func TestValidationError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		cause   error
		want    string
	}{
		{
			name:    "validation error with cause",
			message: "invalid input",
			cause:   errors.New("bad format"),
			want:    "invalid input: bad format",
		},
		{
			name:    "validation error without cause",
			message: "invalid input",
			cause:   nil,
			want:    "invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError(tt.message, tt.cause)
			if err.Error() != tt.want {
				t.Errorf("got %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestRuntimeError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		cause   error
		want    string
	}{
		{
			name:    "runtime error with cause",
			message: "failed to read",
			cause:   errors.New("permission denied"),
			want:    "failed to read: permission denied",
		},
		{
			name:    "runtime error without cause",
			message: "failed to read",
			cause:   nil,
			want:    "failed to read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewRuntimeError(tt.message, tt.cause)
			if err.Error() != tt.want {
				t.Errorf("got %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestGetExitCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "validation error returns code 2",
			err:      NewValidationError("bad input", nil),
			wantCode: 2,
		},
		{
			name:     "validation error with cause returns code 2",
			err:      NewValidationError("bad input", errors.New("invalid format")),
			wantCode: 2,
		},
		{
			name:     "runtime error returns code 1",
			err:      NewRuntimeError("runtime failure", nil),
			wantCode: 1,
		},
		{
			name:     "runtime error with cause returns code 1",
			err:      NewRuntimeError("runtime failure", errors.New("i/o error")),
			wantCode: 1,
		},
		{
			name:     "unknown error returns code 1",
			err:      errors.New("unknown error"),
			wantCode: 1,
		},
		{
			name:     "nil error returns code 1",
			err:      nil,
			wantCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := GetExitCode(tt.err)
			if code != tt.wantCode {
				t.Errorf("got code %d, want %d", code, tt.wantCode)
			}
		})
	}
}

func TestErrorUnwrapping(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		checkFunc   func(error) bool
		description string
	}{
		{
			name: "validation error unwraps cause",
			err:  NewValidationError("bad input", errors.New("test cause")),
			checkFunc: func(e error) bool {
				unwrapped := errors.Unwrap(e)
				return unwrapped != nil && unwrapped.Error() == "test cause"
			},
			description: "validation error cause unwraps correctly",
		},
		{
			name: "runtime error unwraps cause",
			err:  NewRuntimeError("runtime failure", errors.New("test cause")),
			checkFunc: func(e error) bool {
				unwrapped := errors.Unwrap(e)
				return unwrapped != nil && unwrapped.Error() == "test cause"
			},
			description: "runtime error cause unwraps correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.checkFunc(tt.err) {
				t.Errorf("%s: unwrap check failed", tt.description)
			}
		})
	}
}

func TestValidationErrorType(t *testing.T) {
	err := NewValidationError("test", nil)
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Errorf("errors.As() failed to extract ValidationError")
	}
	if validationErr.Message != "test" {
		t.Errorf("got message %q, want %q", validationErr.Message, "test")
	}
}

func TestRuntimeErrorType(t *testing.T) {
	err := NewRuntimeError("test", nil)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Errorf("errors.As() failed to extract RuntimeError")
	}
	if runtimeErr.Message != "test" {
		t.Errorf("got message %q, want %q", runtimeErr.Message, "test")
	}
}

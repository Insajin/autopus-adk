package host

import "errors"

// ErrorCode identifies a machine-classifiable host assembly failure.
type ErrorCode string

const (
	ErrorConfigLoad      ErrorCode = "config_load_failed"
	ErrorAuthLoad        ErrorCode = "auth_load_failed"
	ErrorAuthMissing     ErrorCode = "auth_missing"
	ErrorProviderMissing ErrorCode = "provider_missing"
	ErrorProviderResolve ErrorCode = "provider_resolve_failed"
)

// Error represents a structured host assembly failure.
type Error struct {
	Code    ErrorCode
	Message string
	Err     error
}

// Error implements error.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Code)
}

// Unwrap exposes the underlying cause for diagnostics.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// AsError returns the structured host error when present.
func AsError(err error) *Error {
	var target *Error
	if errors.As(err, &target) {
		return target
	}
	return nil
}

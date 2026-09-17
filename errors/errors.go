// Package errors defines custom error types for gobots.
// Allows external code to programmatically handle different error cases.
package errors

import (
	"fmt"
)

// ErrorCode represents the type/category of an error.
type ErrorCode string

// Error codes for common failure scenarios.
const (
	ErrInvalidConnection ErrorCode = "INVALID_CONNECTION_CONFIG"
	ErrNotConnected      ErrorCode = "NOT_CONNECTED"
	ErrUnknownTransport  ErrorCode = "UNKNOWN_TRANSPORT_TYPE"
	ErrWriteFailure      ErrorCode = "WRITE_FAILURE"
	ErrInternal          ErrorCode = "INTERNAL_ERROR"
)

// AppError is a structured error with a code, message, and optional cause.
// This allows external code to handle different error scenarios programmatically.
type AppError struct {
	Code    ErrorCode
	Message string
	Cause   error // Original error for logging/debugging
}

// Ensure that our implementation satisfies native error interface.
var _ error = (*AppError)(nil)

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Is implements error equality checking for use with errors.Is().
// This allows code like: if errors.Is(err, &AppError{Code: ErrInternal})
func (e *AppError) Is(target error) bool {
	if other, ok := target.(*AppError); ok {
		return e.Code == other.Code
	}
	return false
}

// Unwrap returns the underlying cause error.
// This implements error wrapping for use with errors.Unwrap().
func (e *AppError) Unwrap() error {
	return e.Cause
}

// New creates a new AppError with the given code and message.
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   nil,
	}
}

// Newf creates a new AppError with formatted message.
func Newf(code ErrorCode, format string, args ...interface{}) *AppError {
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Cause:   nil,
	}
}

// Wrap creates a new AppError with an underlying cause.
func Wrap(code ErrorCode, message string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// Wrapf creates a new AppError with formatted message and underlying cause.
func Wrapf(code ErrorCode, cause error, format string, args ...interface{}) *AppError {
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Cause:   cause,
	}
}

// IsCode checks if an error has a specific error code.
// This is a convenience function for checking error codes.
func IsCode(err error, code ErrorCode) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == code
	}
	return false
}

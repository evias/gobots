package errors

import (
	"errors"
	"testing"
)

// TestNewError tests creating a basic error
func TestNewError(t *testing.T) {
	msg := "an internal error occured"
	err := New(ErrInternal, msg)

	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if err.Code != ErrInternal {
		t.Errorf("Expected code %s, got %s", ErrInternal, err.Code)
	}

	if err.Message != msg {
		t.Errorf("Expected message '%s', got '%s'", msg, err.Message)
	}

	if err.Cause != nil {
		t.Errorf("Expected no cause, got %v", err.Cause)
	}
}

// TestNewfError tests creating an error with formatted message
func TestNewfError(t *testing.T) {
	err := Newf(ErrInternal, "profile %s not found", "alice")

	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if err.Code != ErrInternal {
		t.Errorf("Expected code %s, got %s", ErrInternal, err.Code)
	}

	if err.Message != "profile alice not found" {
		t.Errorf("Expected message 'profile alice not found', got '%s'", err.Message)
	}
}

// TestWrapError tests wrapping an error
func TestWrapError(t *testing.T) {
	cause := errors.New("file not found")
	err := Wrap(ErrInternal, "failed to load profile", cause)

	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if err.Code != ErrInternal {
		t.Errorf("Expected code %s, got %s", ErrInternal, err.Code)
	}

	if err.Cause != cause {
		t.Errorf("Expected cause to be original error")
	}
}

// TestWrapfError tests wrapping an error with formatted message
func TestWrapfError(t *testing.T) {
	cause := errors.New("file not found")
	err := Wrapf(ErrInternal, cause, "failed to load %s", "profile")

	if err.Message != "failed to load profile" {
		t.Errorf("Expected message 'failed to load profile', got '%s'", err.Message)
	}

	if err.Cause != cause {
		t.Errorf("Expected cause to be preserved")
	}
}

// TestErrorString tests the Error() method
func TestErrorString(t *testing.T) {
	testCases := []struct {
		name     string
		err      *AppError
		expected string
	}{
		{
			name:     "without_cause",
			err:      New(ErrInternal, "profile not found"),
			expected: "INTERNAL_ERROR: profile not found",
		},
		{
			name:     "with_cause",
			err:      Wrap(ErrInvalidConnection, "could not find host", errors.New("empty host")),
			expected: "INVALID_CONNECTION_CONFIG: could not find host (caused by: empty host)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.err.Error()
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

// TestIsMethod tests the Is() method for error comparison
func TestIsMethod(t *testing.T) {
	err1 := New(ErrInternal, "profile not found")
	err2 := New(ErrInternal, "different message")
	err3 := New(ErrInvalidConnection, "contact not found")

	if !err1.Is(err2) {
		t.Errorf("Expected err1.Is(err2) to be true (same code)")
	}

	if err1.Is(err3) {
		t.Errorf("Expected err1.Is(err3) to be false (different code)")
	}

	if err1.Is(errors.New("some other error")) {
		t.Errorf("Expected err1.Is(non-AppError) to be false")
	}
}

// TestUnwrap tests the Unwrap() method
func TestUnwrap(t *testing.T) {
	cause := errors.New("original error")
	err := Wrap(ErrInternal, "storage failed", cause)

	if err.Unwrap() != cause {
		t.Errorf("Expected Unwrap() to return original cause")
	}

	err2 := New(ErrInvalidConnection, "not found")
	if err2.Unwrap() != nil {
		t.Errorf("Expected Unwrap() to return nil for error without cause")
	}
}

// TestIsCode tests the IsCode convenience function
func TestIsCode(t *testing.T) {
	err := New(ErrInternal, "not found")

	if !IsCode(err, ErrInternal) {
		t.Errorf("Expected IsCode(err, ErrInternal) to be true")
	}

	if IsCode(err, ErrInvalidConnection) {
		t.Errorf("Expected IsCode(err, ErrInvalidConnection) to be false")
	}

	if IsCode(errors.New("regular error"), ErrInternal) {
		t.Errorf("Expected IsCode with non-AppError to be false")
	}
}

// TestErrorCodes tests that all error codes are defined
func TestErrorCodes(t *testing.T) {
	codes := []ErrorCode{
		ErrInvalidConnection,
		ErrInternal,
	}

	for _, code := range codes {
		if code == "" {
			t.Errorf("Expected non-empty error code")
		}
	}
}

// TestErrorsIsIntegration tests integration with standard errors.Is()
func TestErrorsIsIntegration(t *testing.T) {
	target := New(ErrInternal, "")
	err := New(ErrInternal, "different message")

	// This tests standard errors.Is() integration
	if !errors.Is(err, target) {
		t.Errorf("Expected errors.Is(err, target) to work with AppError")
	}
}

// TestErrorsAsIntegration tests integration with standard errors.As()
func TestErrorsAsIntegration(t *testing.T) {
	cause := errors.New("underlying error")
	err := Wrap(ErrInternal, "storage failed", cause)

	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Errorf("Expected errors.As() to work with AppError")
	}

	if appErr.Code != ErrInternal {
		t.Errorf("Expected appErr.Code to be ErrInternal")
	}
}

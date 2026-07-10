package apperror

import (
	"errors"
	"fmt"
)

// AppError represents a structured application error with HTTP status code,
// business error code, and user-facing message.
type AppError struct {
	Code       string // Business error code, e.g. "DEVICE_NOT_FOUND"
	Message    string // User-facing message
	StatusCode int    // HTTP status code
	Cause      error  // Original error (optional)
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// New creates a new AppError.
func New(code string, statusCode int, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// Wrap wraps an existing error with AppError context.
func Wrap(err error, code string, statusCode int, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Cause:      err,
	}
}

// WithMessage returns a copy of the error with a different message.
// Useful for creating domain-specific errors from generic ones.
func (e *AppError) WithMessage(message string) *AppError {
	return &AppError{
		Code:       e.Code,
		Message:    message,
		StatusCode: e.StatusCode,
		Cause:      e.Cause,
	}
}

// ---------------------------------------------------------------------------
// Predefined errors
// ---------------------------------------------------------------------------

var (
	ErrNotFound      = New("NOT_FOUND", 404, "resource not found")
	ErrInvalidInput  = New("INVALID_INPUT", 400, "invalid input")
	ErrUnauthorized  = New("UNAUTHORIZED", 401, "unauthorized")
	ErrForbidden     = New("FORBIDDEN", 403, "forbidden")
	ErrConflict      = New("CONFLICT", 409, "resource conflict")
	ErrInternal      = New("INTERNAL", 500, "internal error")
	ErrQuotaExceeded = New("QUOTA_EXCEEDED", 429, "quota exceeded")
)

// ---------------------------------------------------------------------------
// Domain-specific errors
// ---------------------------------------------------------------------------

var (
	ErrDeviceNotFound     = ErrNotFound.WithMessage("device not found")
	ErrDeviceAlreadyExist = ErrConflict.WithMessage("device already exists")
	ErrUserNotFound       = ErrNotFound.WithMessage("user not found")
	ErrInvalidCredentials = ErrUnauthorized.WithMessage("invalid username or password")
	ErrAlarmNotFound      = ErrNotFound.WithMessage("alarm not found")
)

// IsNotFound checks if an error is a "not found" type error.
func IsNotFound(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.StatusCode == 404
	}
	return false
}

// IsInvalidInput checks if an error is an "invalid input" type error.
func IsInvalidInput(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.StatusCode == 400
	}
	return false
}

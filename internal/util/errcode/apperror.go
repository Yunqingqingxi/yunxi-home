package errcode

import (
	"fmt"
	"runtime/debug"
)

// AppError is a structured application error.
type AppError struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"-"`  // debug detail, logged but not sent to client
	Cause   error  `json:"-"`
	Stack   string `json:"-"`  // only filled for Internal errors
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause.
func (e *AppError) Unwrap() error { return e.Cause }

// New creates an AppError with the given code and detail.
func New(code Code, detail string) *AppError {
	return &AppError{
		Code:    code,
		Message: code.Message(),
		Detail:  detail,
	}
}

// Wrap wraps an existing error into an AppError.
func Wrap(code Code, detail string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Message: code.Message(),
		Detail:  detail,
		Cause:   cause,
	}
}

// Internal creates an INTERNAL_ERROR with a captured stack trace.
func Internal(detail string, cause error) *AppError {
	return &AppError{
		Code:    ErrInternal,
		Message: ErrInternal.Message(),
		Detail:  detail,
		Cause:   cause,
		Stack:   string(debug.Stack()),
	}
}

// BadRequest creates a BAD_REQUEST error.
func BadRequest(detail string) *AppError { return New(ErrBadRequest, detail) }

// NotFound creates a NOT_FOUND error.
func NotFound(detail string) *AppError { return New(ErrNotFound, detail) }

// Unauthorized creates an UNAUTHORIZED error.
func Unauthorized(detail string) *AppError { return New(ErrUnauthorized, detail) }

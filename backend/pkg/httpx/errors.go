// Package httpx provides HTTP transport utilities: response writers and
// HTTP-layer error types, following the shared API contract:
//
//	Success: { "data": ... }
//	Error:   { "error": { "code": "...", "message": "..." } }
package httpx

import "net/http"

// HTTPError maps a domain error to an HTTP status code and a standardised
// error code from the API contract.
type HTTPError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *HTTPError) Error() string { return e.Message }

// Common pre-built HTTP errors.
var (
	ErrBadRequest = &HTTPError{
		StatusCode: http.StatusBadRequest,
		Code:       "BAD_REQUEST",
		Message:    "the request was malformed or contained invalid parameters",
	}
	ErrUnauthorized = &HTTPError{
		StatusCode: http.StatusUnauthorized,
		Code:       "UNAUTHORIZED",
		Message:    "sesi tidak valid atau sudah berakhir, silakan masuk kembali",
	}
	ErrForbidden = &HTTPError{
		StatusCode: http.StatusForbidden,
		Code:       "FORBIDDEN",
		Message:    "you do not have permission to perform this action",
	}
	ErrNotFound = &HTTPError{
		StatusCode: http.StatusNotFound,
		Code:       "NOT_FOUND",
		Message:    "the requested resource was not found",
	}
	ErrConflict = &HTTPError{
		StatusCode: http.StatusConflict,
		Code:       "CONFLICT",
		Message:    "the resource already exists or conflicts with another",
	}
	ErrUnprocessable = &HTTPError{
		StatusCode: http.StatusUnprocessableEntity,
		Code:       "UNPROCESSABLE_ENTITY",
		Message:    "the request was well-formed but contained semantic errors",
	}
	// ErrTooManyRequests is the generic response for rate-limited auth attempts.
	// It never reveals whether the account exists.
	ErrTooManyRequests = &HTTPError{
		StatusCode: http.StatusTooManyRequests,
		Code:       "TOO_MANY_REQUESTS",
		Message:    "terlalu banyak percobaan, silakan coba lagi nanti",
	}
	ErrInternal = &HTTPError{
		StatusCode: http.StatusInternalServerError,
		Code:       "INTERNAL_SERVER_ERROR",
		Message:    "an unexpected error occurred",
	}
)

// NewHTTPError constructs a custom HTTPError.
func NewHTTPError(statusCode int, code, message string) *HTTPError {
	return &HTTPError{StatusCode: statusCode, Code: code, Message: message}
}

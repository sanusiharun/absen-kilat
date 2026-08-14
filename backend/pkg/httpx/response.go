package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// DataResponse is the success envelope from the shared API contract.
type DataResponse[T any] struct {
	Data T `json:"data"`
}

// ErrorBody carries the structured error information from the API contract.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the error envelope from the shared API contract.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// WriteOK writes a 200 OK JSON response with a data payload.
func WriteOK[T any](w http.ResponseWriter, data T) {
	writeJSON(w, http.StatusOK, DataResponse[T]{Data: data})
}

// WriteCreated writes a 201 Created JSON response with a data payload.
func WriteCreated[T any](w http.ResponseWriter, data T) {
	writeJSON(w, http.StatusCreated, DataResponse[T]{Data: data})
}

// WriteError writes an error JSON response, deriving the status code and
// error code from the domain error type when possible, falling back to 500.
func WriteError(w http.ResponseWriter, err error) {
	httpErr := mapDomainError(err)
	writeJSON(w, httpErr.StatusCode, ErrorResponse{
		Error: ErrorBody{Code: httpErr.Code, Message: httpErr.Message},
	})
}

// WriteHTTPError writes a known HTTPError response directly.
func WriteHTTPError(w http.ResponseWriter, httpErr *HTTPError) {
	writeJSON(w, httpErr.StatusCode, ErrorResponse{
		Error: ErrorBody{Code: httpErr.Code, Message: httpErr.Message},
	})
}

// writeJSON marshals v as JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v) //nolint:errcheck // headers already sent; nothing to do if the client disconnects mid-write
}

// mapDomainError converts a domain/application error into an HTTPError.
func mapDomainError(err error) *HTTPError {
	if err == nil {
		return ErrInternal
	}

	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr
	}

	switch {
	case pkgerrors.IsNotFound(err):
		return &HTTPError{StatusCode: http.StatusNotFound, Code: "NOT_FOUND", Message: err.Error()}
	case pkgerrors.IsConflict(err):
		return &HTTPError{StatusCode: http.StatusConflict, Code: "CONFLICT", Message: err.Error()}
	case pkgerrors.IsValidation(err):
		return &HTTPError{StatusCode: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR", Message: err.Error()}
	case pkgerrors.IsUnauthorized(err):
		return &HTTPError{StatusCode: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: err.Error()}
	case pkgerrors.IsForbidden(err):
		return &HTTPError{StatusCode: http.StatusForbidden, Code: "FORBIDDEN", Message: err.Error()}
	default:
		// Unexpected/system errors: never leak internals to the client.
		return ErrInternal
	}
}

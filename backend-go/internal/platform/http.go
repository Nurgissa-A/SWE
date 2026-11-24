package platform

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
)

// APIError is a client-facing failure. Code is a stable machine-readable identifier
// (for example SOLD_OUT) that clients translate; Message is the English fallback.
type APIError struct {
	Status  int            `json:"-"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Params  map[string]any `json:"params,omitempty"`
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

// Err builds an APIError; params are optional key/value pairs used for message interpolation.
func Err(status int, code, message string, params ...any) *APIError {
	e := &APIError{Status: status, Code: code, Message: message}
	if len(params) > 0 {
		e.Params = map[string]any{}
		for i := 0; i+1 < len(params); i += 2 {
			if k, ok := params[i].(string); ok {
				e.Params[k] = params[i+1]
			}
		}
	}
	return e
}

func BadRequest(code, message string, params ...any) *APIError {
	return Err(http.StatusBadRequest, code, message, params...)
}
func Conflict(code, message string, params ...any) *APIError {
	return Err(http.StatusConflict, code, message, params...)
}
func NotFound(message string) *APIError { return Err(http.StatusNotFound, "NOT_FOUND", message) }
func Forbidden(code, message string) *APIError {
	return Err(http.StatusForbidden, code, message)
}

var ErrUnauthenticated = Err(http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")

// Handler is an HTTP handler that returns an error; *APIError values become JSON
// error responses and anything else becomes a logged 500 without internal detail.
type Handler func(w http.ResponseWriter, r *http.Request) error

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			WriteError(w, apiErr)
			return
		}
		log.Printf("internal error %s %s: %v", r.Method, r.URL.Path, err)
		WriteError(w, Err(http.StatusInternalServerError, "INTERNAL", "Something went wrong. Please try again."))
	}
}

func WriteError(w http.ResponseWriter, e *APIError) {
	if e.Status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	WriteJSON(w, e.Status, map[string]any{"error": e})
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Decode strictly parses one JSON object: unknown fields and trailing data are rejected.
func Decode(r *http.Request, v any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return BadRequest("INVALID_BODY", "Invalid request body")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return BadRequest("INVALID_BODY", "Invalid request body")
	}
	return nil
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID reports whether s is a canonical UUID; path IDs are checked before they reach SQL
// so malformed IDs give 404 rather than a database cast error.
func IsUUID(s string) bool { return uuidPattern.MatchString(s) }

// PathID returns the named path value if it is a UUID, or a 404 APIError.
func PathID(r *http.Request, name, what string) (string, error) {
	id := r.PathValue(name)
	if !IsUUID(id) {
		return "", NotFound(what + " not found")
	}
	return id, nil
}

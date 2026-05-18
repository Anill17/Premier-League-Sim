// Package response is the single source of truth for the HTTP response
// envelope. Every handler in the project MUST write through these
// helpers — never call w.Write directly, never build envelopes inline.
package response

import (
	"encoding/json"
	"log"
	"net/http"
)

// Envelope is the universal JSON shape returned by every endpoint:
//
//	{ "success": bool, "data": <any>, "error": <APIError|null> }
//
// data and error are always present; one is nil. Keeping the shape
// consistent makes the API trivial to consume from any client.
type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   *APIError   `json:"error"`
}

// APIError is the error payload returned to clients. The Code field is
// a stable machine-readable identifier; the Message is human-readable.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Canonical error codes — referenced everywhere. New codes go here
// (and only here), never inline strings.
const (
	CodeValidation        = "VALIDATION_ERROR"
	CodeInvalidJSON       = "INVALID_JSON"
	CodeNotFound          = "NOT_FOUND"
	CodeSeasonNotFound    = "SEASON_NOT_FOUND"
	CodeFixtureNotFound   = "FIXTURE_NOT_FOUND"
	CodeConflict          = "CONFLICT"
	CodeWeekAlreadyPlayed = "WEEK_ALREADY_PLAYED"
	CodeSeasonComplete    = "SEASON_COMPLETE"
	CodeUnprocessable     = "UNPROCESSABLE"
	CodeInternal          = "INTERNAL_ERROR"
)

// Success writes a 2xx envelope.
func Success(w http.ResponseWriter, status int, data interface{}) {
	write(w, status, Envelope{Success: true, Data: data, Error: nil})
}

// Created is a tiny convenience wrapper for 201 Created responses.
func Created(w http.ResponseWriter, data interface{}) {
	Success(w, http.StatusCreated, data)
}

// OK is a tiny convenience wrapper for 200 OK responses.
func OK(w http.ResponseWriter, data interface{}) {
	Success(w, http.StatusOK, data)
}

// Error writes a 4xx/5xx envelope.
func Error(w http.ResponseWriter, status int, code, message string) {
	write(w, status, Envelope{
		Success: false,
		Data:    nil,
		Error:   &APIError{Code: code, Message: message},
	})
}

// BadRequest is shorthand for a 400 with VALIDATION_ERROR code.
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, CodeValidation, message)
}

// NotFound is shorthand for a 404 with a caller-supplied code.
func NotFound(w http.ResponseWriter, code, message string) {
	Error(w, http.StatusNotFound, code, message)
}

// Conflict is shorthand for a 409 with a caller-supplied code.
func Conflict(w http.ResponseWriter, code, message string) {
	Error(w, http.StatusConflict, code, message)
}

// Unprocessable is shorthand for a 422 with a caller-supplied code.
func Unprocessable(w http.ResponseWriter, code, message string) {
	Error(w, http.StatusUnprocessableEntity, code, message)
}

// Internal is shorthand for a 500. The actual error is logged by the
// recoverer / handler; only a generic message goes back to the client.
func Internal(w http.ResponseWriter, message string) {
	if message == "" {
		message = "internal server error"
	}
	Error(w, http.StatusInternalServerError, CodeInternal, message)
}

func write(w http.ResponseWriter, status int, env Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(env); err != nil {
		// We've already written headers — best we can do is log.
		log.Printf("response: failed to encode envelope: %v", err)
	}
}

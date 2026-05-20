// Package handler exposes the HTTP surface of the application. Each
// handler file owns a small, related group of endpoints and contains
// no business logic — that lives in internal/service.
package handler

import (
	"context"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"

	"github.com/Anill17/league-api/pkg/response"
)

// ctxKey is unexported so external packages cannot accidentally inject
// values into the same slot.
type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
)

// HeaderRequestID is the canonical request-id header used in both
// directions: clients may supply one, and the server always echoes it.
const HeaderRequestID = "X-Request-ID"

// RequestID is middleware that ensures every request has a unique id.
// The id is exposed via the response header and embedded in the
// request context so handlers and the logger can reference it.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if id == "" {
			id = uuid.New().String()
		}
		w.Header().Set(HeaderRequestID, id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFrom returns the id injected by the RequestID middleware,
// or the empty string if no id is present.
func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

// Logger is access-log middleware: method, path, status, duration,
// request id. Single-line format keeps it grep-friendly.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		log.Printf(
			"%s %s status=%d duration=%s request_id=%s",
			r.Method, r.URL.Path, rec.status,
			time.Since(start), RequestIDFrom(r.Context()),
		)
	})
}

// Recoverer is panic-safety middleware. It logs the stack trace and
// returns a generic 500 so the connection never leaks an unhandled
// panic. Place it ABOVE every other middleware in the chain.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf(
					"PANIC %s %s request_id=%s: %v\n%s",
					r.Method, r.URL.Path,
					RequestIDFrom(r.Context()),
					rec, debug.Stack(),
				)
				response.Internal(w, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder captures the status code for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

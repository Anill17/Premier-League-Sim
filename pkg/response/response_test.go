package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSuccessEnvelope(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	OK(w, map[string]int{"x": 1})

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	ct := w.Header().Get("Content-Type")
	if ct == "" || ct[:16] != "application/json" {
		t.Fatalf("content-type: got %q, want application/json...", ct)
	}

	var env Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !env.Success {
		t.Fatalf("Success: got %v, want true", env.Success)
	}
	if env.Error != nil {
		t.Fatalf("Error: got %+v, want nil", env.Error)
	}
	if env.Data == nil {
		t.Fatalf("Data is nil")
	}
}

func TestErrorEnvelope(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		writer   func(w http.ResponseWriter)
		wantCode int
		wantErr  string
	}{
		{
			"bad_request",
			func(w http.ResponseWriter) { BadRequest(w, "bad") },
			http.StatusBadRequest,
			CodeValidation,
		},
		{
			"not_found",
			func(w http.ResponseWriter) { NotFound(w, CodeSeasonNotFound, "x") },
			http.StatusNotFound,
			CodeSeasonNotFound,
		},
		{
			"conflict",
			func(w http.ResponseWriter) { Conflict(w, CodeWeekAlreadyPlayed, "x") },
			http.StatusConflict,
			CodeWeekAlreadyPlayed,
		},
		{
			"unprocessable",
			func(w http.ResponseWriter) { Unprocessable(w, CodeUnprocessable, "x") },
			http.StatusUnprocessableEntity,
			CodeUnprocessable,
		},
		{
			"internal",
			func(w http.ResponseWriter) { Internal(w, "") },
			http.StatusInternalServerError,
			CodeInternal,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			c.writer(w)
			if w.Code != c.wantCode {
				t.Fatalf("status: got %d, want %d", w.Code, c.wantCode)
			}
			var env Envelope
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if env.Success {
				t.Fatalf("Success: got true, want false")
			}
			if env.Data != nil {
				t.Fatalf("Data: got %+v, want nil", env.Data)
			}
			if env.Error == nil {
				t.Fatalf("Error: got nil")
			}
			if env.Error.Code != c.wantErr {
				t.Fatalf("code: got %q, want %q", env.Error.Code, c.wantErr)
			}
		})
	}
}

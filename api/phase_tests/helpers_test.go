// Package phase_tests holds the acceptance tests for each phase of Program Control. They are
// provided; do not edit them to make your implementation pass. This file has helpers shared by
// every phase's tests.
package phase_tests

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/look-itsaxiom/program-control/api/internal/httpapi"
)

// newHandler builds the API handler under test, wired to DATABASE_URL from the environment.
func newHandler(t *testing.T) http.Handler {
	t.Helper()
	h, err := httpapi.NewHandler(t.Context(), httpapi.Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("httpapi.NewHandler: %v", err)
	}
	return h
}

// do issues a single request against h and returns the recorded response.
func do(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = strings.NewReader(string(buf))
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decode unmarshals the response body of rec into a T, failing the test on error.
func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
	}
	return out
}

// errorCode extracts the `error.code` field from a JSON error envelope response.
func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error envelope %q: %v", rec.Body.String(), err)
	}
	return payload.Error.Code
}

// phaseGate skips a test when the CI-controlled PHASE environment variable is set and lower than
// n. Locally (PHASE unset) it never skips, so every test you write is always live for you.
func phaseGate(t *testing.T, n int) {
	t.Helper()
	raw := os.Getenv("PHASE")
	if raw == "" {
		return
	}
	current, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("PHASE=%q is not an integer", raw)
	}
	if current < n {
		t.Skipf("PHASE=%d < %d; skipping", current, n)
	}
}

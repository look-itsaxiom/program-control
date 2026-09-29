// Package httpapi is your REST layer. This file is a stub: it compiles and serves a health check,
// but every other route answers 501 until you build it.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// Config configures the HTTP API.
type Config struct {
	DatabaseURL string
	Logger      *slog.Logger

	// QueryTracer, when set, is installed on every connection the handler opens (phase 4's
	// query-count contract depends on this: a test builds a counting tracer, passes it here,
	// and asserts on how many queries a GraphQL request issues).
	QueryTracer pgx.QueryTracer
}

// NewHandler returns the HTTP API. This stub answers every route with 501 until you build it.
func NewHandler(ctx context.Context, cfg Config) (http.Handler, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"build me"}}`))
	})

	return mux, nil
}

// Package httpapi is your REST layer. This file is a stub: it compiles and serves a health check,
// but every other route answers 501 until you build it.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
)

// Config configures the HTTP API.
type Config struct {
	DatabaseURL string
	Logger      *slog.Logger
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

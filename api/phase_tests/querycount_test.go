// This file gives the phase 4 GraphQL acceptance tests a way to observe how many SQL statements a
// request issues. It exercises only the contract in phases/4-graphql/STORIES.md (S4.4): the
// handler installs Config.QueryTracer, when set, on every database connection it opens. It does
// not assume anything about how the handler opens connections (one pool, several pools, which
// package) beyond that contract.
package phase_tests

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/look-itsaxiom/program-control/api/internal/httpapi"
)

// countingTracer implements pgx.QueryTracer and counts every query traced across every
// connection it is installed on. It is safe for concurrent use: a single GraphQL request can
// legitimately hand out work to more than one pooled connection at once (for example, if a
// resolver fans out dataloader batches concurrently), so the count must not race.
type countingTracer struct {
	n atomic.Int64
}

func (c *countingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}

func (c *countingTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// Count returns the number of queries traced so far.
func (c *countingTracer) Count() int64 {
	return c.n.Load()
}

// newHandlerWithTracer builds the API handler under test the same way newHandler does, but also
// passes tracer through httpapi.Config.QueryTracer, so the query-count contract can be observed
// from outside the handler.
func newHandlerWithTracer(t *testing.T, tracer pgx.QueryTracer) http.Handler {
	t.Helper()
	h, err := httpapi.NewHandler(t.Context(), httpapi.Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		QueryTracer: tracer,
	})
	if err != nil {
		t.Fatalf("httpapi.NewHandler: %v", err)
	}
	return h
}

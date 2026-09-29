// Package store is the data layer. Every SQL statement this service runs lives here, behind
// methods that speak in domain types rather than rows, so the HTTP and GraphQL layers above it
// never build a query of their own.
package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors callers match with errors.Is to choose a status code. Anything else coming out
// of this package is an unexpected failure and belongs in a 500.
var (
	// ErrNotFound means the row does not exist, is soft-deleted, or is invisible to the caller.
	// The three are deliberately indistinguishable: telling them apart leaks tenancy.
	ErrNotFound = errors.New("store: not found")

	// ErrDuplicate means the row already exists exactly as given.
	ErrDuplicate = errors.New("store: already exists")

	// ErrDuplicateTitle means another active task in the same program already has this title.
	ErrDuplicateTitle = errors.New("store: duplicate task title")
)

// CycleError reports that an edge was refused because it would close a cycle, and names every
// task on the path the cycle would run through so the caller can explain itself.
type CycleError struct {
	Path []string
}

func (e *CycleError) Error() string {
	return "store: dependency would create a cycle through " + strings.Join(e.Path, " -> ")
}

// Store owns the connection pool and exposes one method per thing the service needs to do.
type Store struct {
	pool *pgxpool.Pool
}

// New opens a pool against databaseURL and verifies it answers. The tracer, when non-nil, is
// installed on every connection the pool opens; phase 4's query-count contract depends on it.
func New(ctx context.Context, databaseURL string, tracer pgx.QueryTracer) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.ConnConfig.Tracer = tracer

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	// Acquire proves the database is reachable at startup. It is deliberately not Ping, which
	// would execute a statement: phase 4's query-count contract counts every statement the
	// handler's connections run, and opening the store should not spend one of them.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	conn.Release()

	return &Store{pool: pool}, nil
}

// Close releases the pool. It is safe to call more than once.
func (s *Store) Close() { s.pool.Close() }

// Pool exposes the underlying pool for callers that need to run their own statements, such as a
// background worker polling an outbox.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// isUniqueViolation reports whether err is Postgres' unique_violation, optionally narrowed to one
// constraint or index name. An empty constraint matches any of them.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}

// notFound maps pgx's no-rows sentinel onto ours and passes anything else through wrapped.
func notFound(err error, what string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", what, err)
}

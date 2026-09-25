// Package testdb is a test helper: it connects to the Postgres instance named by DATABASE_URL,
// migrates it up once per process, and truncates every table between tests so each test starts
// from a clean, migrated database. You should not need to change this package; it is provided so
// your own tests (and the phase acceptance tests) have something reliable to build on.
package testdb

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver used by goose
	"github.com/pressly/goose/v3"

	"github.com/look-itsaxiom/program-control/api/migrations"
)

const skipHint = "DATABASE_URL not set; run `pnpm db:up`"

var (
	// mu serializes access across tests as a belt-and-suspenders alongside `go test -p 1`.
	mu sync.Mutex

	migrateOnce sync.Once
	migrateErr  error
)

// Migrations returns the embedded filesystem of goose migration files.
func Migrations() embed.FS {
	return migrations.FS
}

// Open connects to DATABASE_URL, ensures migrations have been applied (once per test process),
// and registers a cleanup that truncates every table so the next test starts clean.
//
// It skips the test with a clear hint when DATABASE_URL is unset, and fails loudly with the
// underlying error when DATABASE_URL is set but the database is unreachable: local runs should
// explain themselves, and CI must fail hard rather than silently skip.
func Open(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip(skipHint)
	}

	mu.Lock()
	t.Cleanup(mu.Unlock)

	migrateOnce.Do(func() {
		migrateErr = migrateUp(databaseURL)
	})
	if migrateErr != nil {
		t.Fatalf("testdb: migrate up: %v\n%s", migrateErr, skipHint)
	}

	ctx := t.Context()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("testdb: connect to database: %v\n%s", err, skipHint)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("testdb: ping database: %v\n%s", err, skipHint)
	}

	t.Cleanup(func() {
		Truncate(t, pool)
		pool.Close()
	})

	return pool
}

// Truncate empties every user table in the public schema (everything except goose's own
// bookkeeping table). It is exported for tests that need a clean slate partway through.
//
// It deliberately uses a background context rather than t.Context(): Truncate is normally called
// from a t.Cleanup callback, and t.Context() is already canceled by the time cleanups run.
func Truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	rows, err := pool.Query(ctx, `
		select tablename from pg_tables
		where schemaname = 'public' and tablename <> 'goose_db_version'
	`)
	if err != nil {
		t.Fatalf("testdb: list tables: %v", err)
	}

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			t.Fatalf("testdb: scan table name: %v", err)
		}
		tables = append(tables, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("testdb: list tables: %v", err)
	}
	if len(tables) == 0 {
		return
	}

	quoted := make([]string, len(tables))
	for i, name := range tables {
		quoted[i] = fmt.Sprintf("%q", name)
	}
	stmt := "truncate table " + strings.Join(quoted, ", ") + " restart identity cascade"
	if _, err := pool.Exec(ctx, stmt); err != nil {
		t.Fatalf("testdb: truncate tables: %v", err)
	}
}

func migrateUp(databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open sql.DB: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// Command migrate manages the database schema with goose, running against the migrations
// embedded from api/migrations. It is provided and complete: you should not need to edit it.
//
// Usage:
//
//	go run ./cmd/migrate up             # apply all pending migrations
//	go run ./cmd/migrate down           # roll back one migration
//	go run ./cmd/migrate status         # show which migrations are applied
//	go run ./cmd/migrate reset          # roll back to zero, then reapply all (dev only)
//	go run ./cmd/migrate create NAME    # scaffold migrations/000N_NAME.sql
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/look-itsaxiom/program-control/api/migrations"
)

const migrationsDir = "migrations"

// migrationPrefix matches the four-digit numbering used across this project's migration files
// (0001_bootstrap.sql, 0002_whatever.sql, ...).
var migrationPrefix = regexp.MustCompile(`^(\d{4})_`)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	command := os.Args[1]

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("migrate: set dialect: %v", err)
	}

	if command == "create" {
		if len(os.Args) < 3 {
			log.Fatal("migrate: create requires a name, e.g. `go run ./cmd/migrate create add_users`")
		}
		if err := createMigration(os.Args[2]); err != nil {
			log.Fatalf("migrate: create: %v", err)
		}
		return
	}

	db := mustOpenDB()
	defer db.Close()

	switch command {
	case "up":
		if err := goose.Up(db, "."); err != nil {
			log.Fatalf("migrate: up: %v", err)
		}
	case "down":
		if err := goose.Down(db, "."); err != nil {
			log.Fatalf("migrate: down: %v", err)
		}
	case "status":
		if err := goose.Status(db, "."); err != nil {
			log.Fatalf("migrate: status: %v", err)
		}
	case "reset":
		if err := goose.DownTo(db, ".", 0); err != nil {
			log.Fatalf("migrate: reset (down to zero): %v", err)
		}
		if err := goose.Up(db, "."); err != nil {
			log.Fatalf("migrate: reset (up): %v", err)
		}
	default:
		usage()
	}
}

// createMigration scaffolds a new migration file directly on the real filesystem (not the
// embedded FS baked in at build time, so the new file shows up for the next build), numbered
// NNNN_name.sql with four digits, one higher than the highest existing migration.
//
// goose's own Create (with SetSequential) pads to five digits, which sorts after this project's
// four-digit 0001_bootstrap.sql; scanning and formatting ourselves keeps the numbering consistent.
func createMigration(name string) error {
	next, err := nextMigrationNumber()
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%04d_%s.sql", next, sanitizeMigrationName(name))
	path := filepath.Join(migrationsDir, filename)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return fmt.Errorf("create migration file: %w", err)
	}
	defer f.Close()

	const template = "-- +goose Up\nSELECT 'up SQL query';\n\n-- +goose Down\nSELECT 'down SQL query';\n"
	if _, err := f.WriteString(template); err != nil {
		return fmt.Errorf("write migration template: %w", err)
	}

	log.Printf("Created new file: %s", path)
	return nil
}

// nextMigrationNumber scans migrationsDir on the real filesystem for the highest NNNN_ prefix
// among existing .sql files and returns one more than that (or 1 if there are none yet).
func nextMigrationNumber() (int, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return 0, fmt.Errorf("read migrations dir: %w", err)
	}

	highest := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		match := migrationPrefix.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		n, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if n > highest {
			highest = n
		}
	}
	return highest + 1, nil
}

// sanitizeMigrationName lowercases name and replaces anything that isn't a letter, digit, or
// underscore with an underscore, matching the shape goose's own snake_case naming produces.
func sanitizeMigrationName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func mustOpenDB() *sql.DB {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("migrate: DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("migrate: open database: %v", err)
	}
	return db
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: migrate <up|down|status|reset|create NAME>")
	os.Exit(1)
}

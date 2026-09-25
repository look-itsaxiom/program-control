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

	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/look-itsaxiom/program-control/api/migrations"
)

const migrationsDir = "migrations"

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	command := os.Args[1]

	goose.SetBaseFS(migrations.FS)
	goose.SetSequential(true)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("migrate: set dialect: %v", err)
	}

	if command == "create" {
		if len(os.Args) < 3 {
			log.Fatal("migrate: create requires a name, e.g. `go run ./cmd/migrate create add_users`")
		}
		// Create writes to the real filesystem migrations directory, not the embedded FS baked
		// in at build time, so the new file shows up for the next build.
		if err := goose.Create(nil, migrationsDir, os.Args[2], "sql"); err != nil {
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

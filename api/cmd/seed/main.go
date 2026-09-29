// Command seed fills a migrated database with the demo world: one customer org running an engine
// test campaign, two vendors it shares the program with, and a task graph with enough shape to
// show a ready list, a cycle rejection, and a vendor's narrowed view.
//
// It is idempotent. Every row carries a fixed id and is inserted with "on conflict do nothing",
// and the history is only written for a task that has none, so running it twice leaves the same
// world rather than a second copy of it. That matters because the command someone runs before a
// demo is the one they run again when the demo goes sideways.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Fixed ids, so a second run recognizes the world it already made.
const (
	customerOrg = "10000000-0000-4000-8000-000000000001"
	vendorAOrg  = "10000000-0000-4000-8000-000000000002"
	vendorBOrg  = "10000000-0000-4000-8000-000000000003"

	aliceID = "20000000-0000-4000-8000-000000000001"
	bobID   = "20000000-0000-4000-8000-000000000002"
	carolID = "20000000-0000-4000-8000-000000000003"
	daveID  = "20000000-0000-4000-8000-000000000004"

	programID = "30000000-0000-4000-8000-000000000001"

	taskSpec        = "40000000-0000-4000-8000-000000000001"
	taskHousing     = "40000000-0000-4000-8000-000000000002"
	taskManifold    = "40000000-0000-4000-8000-000000000003"
	taskAssemble    = "40000000-0000-4000-8000-000000000004"
	taskGaskets     = "40000000-0000-4000-8000-000000000005"
	taskExport      = "40000000-0000-4000-8000-000000000006"
	taskDynoRun     = "40000000-0000-4000-8000-000000000007"
	taskTeardown    = "40000000-0000-4000-8000-000000000008"
	taskFinalReport = "40000000-0000-4000-8000-000000000009"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("seed: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is not set; run `pnpm db:up` first")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("open pool: %w", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, step := range []struct {
		name string
		fn   func(context.Context, pgx.Tx) error
	}{
		{"organizations", seedOrganizations},
		{"users and memberships", seedPeople},
		{"program and shares", seedProgram},
		{"tasks", seedTasks},
		{"dependencies", seedDependencies},
		{"history", seedHistory},
	} {
		if err := step.fn(ctx, tx); err != nil {
			return fmt.Errorf("seed %s: %w", step.name, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	fmt.Printf("Seeded the demo world.\n")
	fmt.Printf("  program id: %s\n", programID)
	fmt.Printf("  sign in as: %s (customer), %s (vendor A), %s (vendor B), %s (stranger)\n",
		aliceID, bobID, carolID, daveID)
	fmt.Printf("  open:       http://localhost:5173/programs/%s\n", programID)
	return nil
}

func seedOrganizations(ctx context.Context, tx pgx.Tx) error {
	const q = `
		insert into organizations (id, name) values ($1, $2)
		on conflict (id) do nothing
	`
	for _, org := range []struct{ id, name string }{
		{customerOrg, "Meridian Propulsion"},
		{vendorAOrg, "Halden Machine Works"},
		{vendorBOrg, "Castile Foundry"},
	} {
		if _, err := tx.Exec(ctx, q, org.id, org.name); err != nil {
			return err
		}
	}
	return nil
}

func seedPeople(ctx context.Context, tx pgx.Tx) error {
	const insertUser = `
		insert into users (id, email, display_name) values ($1, $2, $3)
		on conflict (id) do nothing
	`
	const insertMembership = `
		insert into memberships (user_id, org_id, role) values ($1, $2, $3)
		on conflict (user_id, org_id) do nothing
	`

	people := []struct{ id, email, name, org, role string }{
		{aliceID, "alice@meridian.example", "Alice Nakamura", customerOrg, "owner"},
		{bobID, "bob@halden.example", "Bob Ferreira", vendorAOrg, "member"},
		{carolID, "carol@castile.example", "Carol Nwosu", vendorBOrg, "member"},
		// Dave belongs to no organization: he is the stranger the tenant rules are aimed at.
		{daveID, "dave@elsewhere.example", "Dave Osei", "", ""},
	}
	for _, p := range people {
		if _, err := tx.Exec(ctx, insertUser, p.id, p.email, p.name); err != nil {
			return err
		}
		if p.org == "" {
			continue
		}
		if _, err := tx.Exec(ctx, insertMembership, p.id, p.org, p.role); err != nil {
			return err
		}
	}
	return nil
}

func seedProgram(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
		insert into programs (id, org_id, name) values ($1, $2, $3)
		on conflict (id) do nothing
	`, programID, customerOrg, "Engine Overhaul 2026")
	if err != nil {
		return err
	}

	const share = `
		insert into program_shares (program_id, org_id, role) values ($1, $2, $3)
		on conflict (program_id, org_id) do nothing
	`
	for _, s := range []struct{ org, role string }{
		{vendorAOrg, "editor"},
		{vendorBOrg, "viewer"},
	} {
		if _, err := tx.Exec(ctx, share, programID, s.org, s.role); err != nil {
			return err
		}
	}
	return nil
}

type seedTask struct {
	id       string
	org      string
	title    string
	status   string
	estimate *int
	assignee *string
	dueOn    *string
	note     *string
}

func seedTasks(ctx context.Context, tx pgx.Tx) error {
	const q = `
		insert into tasks (id, program_id, org_id, title, status, estimate_minutes, assignee_id, due_on, internal_note)
		values ($1, $2, $3, $4, $5, $6, $7, $8::date, $9)
		on conflict (id) do nothing
	`
	for _, t := range demoTasks() {
		_, err := tx.Exec(ctx, q, t.id, programID, t.org, t.title, t.status,
			t.estimate, t.assignee, t.dueOn, t.note)
		if err != nil {
			return err
		}
	}
	return nil
}

// demoTasks is the board: a diamond the customer owns the ends of, two vendor-owned branches, a
// blocked task, an independent one, and a short tail, so a demo can show a ready list that
// changes as work finishes.
func demoTasks() []seedTask {
	return []seedTask{
		{taskSpec, customerOrg, "Draft engine spec", "done", intPtr(480), strPtr(aliceID), strPtr("2026-01-12"),
			strPtr("Castile under-bid this contract; do not discuss the target margin with them.")},
		{taskHousing, vendorAOrg, "Machine the housing", "doing", intPtr(960), strPtr(bobID), strPtr("2026-02-02"), nil},
		{taskManifold, vendorBOrg, "Cast the manifold", "todo", intPtr(720), strPtr(carolID), strPtr("2026-02-09"), nil},
		{taskAssemble, customerOrg, "Assemble and test", "todo", intPtr(600), strPtr(aliceID), strPtr("2026-03-02"), nil},
		{taskGaskets, vendorAOrg, "Order replacement gaskets", "todo", intPtr(60), strPtr(bobID), nil, nil},
		{taskExport, customerOrg, "Certify for export", "blocked", intPtr(240), nil, strPtr("2026-03-16"),
			strPtr("Waiting on the updated end-use certificate from legal.")},
		{taskDynoRun, customerOrg, "Dyno run at full load", "todo", intPtr(300), strPtr(aliceID), strPtr("2026-03-09"), nil},
		{taskTeardown, vendorAOrg, "Teardown and inspection", "todo", intPtr(480), strPtr(bobID), strPtr("2026-03-23"), nil},
		{taskFinalReport, customerOrg, "Final report", "todo", intPtr(180), strPtr(aliceID), strPtr("2026-03-30"), nil},
	}
}

func seedDependencies(ctx context.Context, tx pgx.Tx) error {
	const q = `
		insert into task_dependencies (predecessor_id, successor_id) values ($1, $2)
		on conflict (predecessor_id, successor_id) do nothing
	`
	edges := [][2]string{
		{taskSpec, taskHousing},
		{taskSpec, taskManifold},
		{taskHousing, taskAssemble},
		{taskManifold, taskAssemble},
		{taskAssemble, taskDynoRun},
		{taskDynoRun, taskTeardown},
		{taskTeardown, taskFinalReport},
	}
	for _, e := range edges {
		if _, err := tx.Exec(ctx, q, e[0], e[1]); err != nil {
			return err
		}
	}
	return nil
}

// seedHistory gives the audit trail something to show. task_events has no natural key to conflict
// on, so instead of upserting, each task's history is written only when that task has none.
func seedHistory(ctx context.Context, tx pgx.Tx) error {
	const q = `
		insert into task_events (task_id, actor_id, kind, from_status, to_status, created_at)
		select $1, $2, $3, $4, $5, $6
		where not exists (select 1 from task_events where task_id = $1 and kind = $3 and to_status = $5)
	`
	base := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	history := []struct {
		task   string
		actor  *string
		kind   string
		from   string
		to     string
		offset time.Duration
	}{
		{taskSpec, strPtr(aliceID), "status_changed", "todo", "doing", 0},
		{taskSpec, strPtr(aliceID), "status_changed", "doing", "done", 72 * time.Hour},
		{taskHousing, nil, "webhook", "todo", "doing", 96 * time.Hour},
		{taskExport, strPtr(aliceID), "status_changed", "todo", "blocked", 120 * time.Hour},
	}
	for _, h := range history {
		if _, err := tx.Exec(ctx, q, h.task, h.actor, h.kind, h.from, h.to, base.Add(h.offset)); err != nil {
			return err
		}
	}
	return nil
}

func intPtr(n int) *int       { return &n }
func strPtr(s string) *string { return &s }

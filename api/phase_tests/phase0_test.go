package phase_tests

import (
	"testing"

	"github.com/look-itsaxiom/program-control/api/internal/testdb"
)

// TestPhase0_HealthEndpoint checks that the skeleton API serves a working health check without
// any of Chase's code.
func TestPhase0_HealthEndpoint(t *testing.T) {
	phaseGate(t, 0)

	h := newHandler(t)
	rec := do(t, h, "GET", "/healthz", nil, nil)

	if rec.Code != 200 {
		t.Fatalf("GET /healthz: got status %d, want 200", rec.Code)
	}

	body := decode[struct {
		Status string `json:"status"`
	}](t, rec)
	if body.Status != "ok" {
		t.Fatalf("GET /healthz: got status field %q, want %q", body.Status, "ok")
	}
}

// TestPhase0_DatabaseReachable checks that the dev environment's Postgres is reachable and that
// testdb.Open works end to end.
func TestPhase0_DatabaseReachable(t *testing.T) {
	phaseGate(t, 0)

	pool := testdb.Open(t)
	var one int
	if err := pool.QueryRow(t.Context(), "select 1").Scan(&one); err != nil {
		t.Fatalf("select 1: %v", err)
	}
	if one != 1 {
		t.Fatalf("select 1: got %d, want 1", one)
	}
}

// TestPhase0_MigrationsTableExists checks that testdb.Open has run goose's migrations, which
// creates the goose_db_version bookkeeping table.
func TestPhase0_MigrationsTableExists(t *testing.T) {
	phaseGate(t, 0)

	pool := testdb.Open(t)
	var exists bool
	err := pool.QueryRow(t.Context(), `
		select exists (
			select 1 from information_schema.tables
			where table_schema = 'public' and table_name = 'goose_db_version'
		)
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("check goose_db_version: %v", err)
	}
	if !exists {
		t.Fatal("goose_db_version table does not exist; migrations did not run")
	}
}

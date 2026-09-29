package phase_tests

import (
	"net/http"
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

// TestPhase0_UnknownRouteReturnsJsonEnvelope checks that a route the API never defines answers
// with the shared JSON error envelope rather than a plain-text error.
//
// The status is deliberately not pinned to one number, because the correct one changes as Chase
// builds. The scaffold's catch-all answers 501 ("build me") for everything; from phase 2 on,
// S2.7 says a path the API never defined is a 404. Both are right in their own phase, so this
// test asserts what does not change: the envelope, its content type, and a machine-readable
// error code inside it.
func TestPhase0_UnknownRouteReturnsJsonEnvelope(t *testing.T) {
	phaseGate(t, 0)

	const path = "/definitely-not-a-route"

	h := newHandler(t)
	rec := do(t, h, "GET", path, nil, nil)

	if rec.Code != http.StatusNotImplemented && rec.Code != http.StatusNotFound {
		t.Fatalf("GET %s: got status %d, want %d (scaffold) or %d (from phase 2 on)",
			path, rec.Code, http.StatusNotImplemented, http.StatusNotFound)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("GET %s: got Content-Type %q, want %q (body %s)", path, ct, "application/json", rec.Body.String())
	}
	if code := errorCode(t, rec); code == "" {
		t.Fatalf("GET %s: got an empty error.code, want a non-empty code in the envelope (body %s)", path, rec.Body.String())
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

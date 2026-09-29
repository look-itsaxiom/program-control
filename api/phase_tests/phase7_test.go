// This file holds the acceptance tests for phase 7 (ship), a stretch phase. They exercise only
// the contract in phases/7-ship/STORIES.md.
package phase_tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/look-itsaxiom/program-control/api/internal/testdb"
)

// apiDir returns the api/ directory (this file's own directory's parent), so cmd/seed can be
// located and run regardless of the working directory `go test` was invoked from.
func apiDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("S7.1: runtime.Caller failed to locate this test file")
	}
	// this file lives at api/phase_tests/phase7_test.go
	return filepath.Dir(filepath.Dir(file))
}

func TestPhase7_SeedIsIdempotent(t *testing.T) {
	phaseGate(t, 7)

	dir := apiDir(t)
	seedDir := filepath.Join(dir, "cmd", "seed")
	if _, err := os.Stat(seedDir); os.IsNotExist(err) {
		// Not yet written is the normal state before this phase is done, and it is a red test,
		// not a skip: a skipped test would let `pnpm check 7` go green on an unbuilt story.
		t.Fatal("S7.1: api/cmd/seed does not exist yet")
	}

	pool := testdb.Open(t)

	countRows := func() int {
		var n int
		if err := pool.QueryRow(t.Context(), `select count(*) from tasks`).Scan(&n); err != nil {
			t.Fatalf("S7.1: counting tasks after seeding: %v", err)
		}
		return n
	}

	runSeed := func() {
		cmd := exec.Command("go", "run", "./cmd/seed")
		cmd.Dir = dir
		// DATABASE_URL is already in this process's environment (testdb.Open above would have
		// skipped the test otherwise), so the child process inherits it via os.Environ().
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("S7.1: go run ./cmd/seed: %v\n%s", err, out)
		}
	}

	runSeed()
	firstCount := countRows()
	if firstCount == 0 {
		t.Fatal("S7.1: after seeding once, the tasks table is still empty")
	}

	runSeed()
	secondCount := countRows()
	if secondCount != firstCount {
		t.Fatalf("S7.1: seed is not idempotent: got %d tasks after the first run and %d after the second, want equal counts", firstCount, secondCount)
	}
}

// repoRoot returns the repository root (this file lives at api/phase_tests/phase7_test.go).
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("S7.1: runtime.Caller failed to locate this test file")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func TestPhase7_DemoScriptExists(t *testing.T) {
	phaseGate(t, 7)

	demoPath := filepath.Join(repoRoot(t), "scripts", "demo.md")
	contents, err := os.ReadFile(demoPath)
	if err != nil {
		t.Fatalf("S7.1: scripts/demo.md does not exist yet: %v", err)
	}
	if !strings.Contains(string(contents), "pnpm seed") {
		t.Fatal("S7.1: scripts/demo.md never mentions `pnpm seed`, so it does not tell a reader how to fill the database the walkthrough assumes")
	}
}

func TestPhase7_ReadmeHasDecisionsAndNotDone(t *testing.T) {
	phaseGate(t, 7)

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("S7.3: runtime.Caller failed to locate this test file")
	}
	// this file lives at api/phase_tests/phase7_test.go; README.md is two directories up.
	readmePath := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(file))), "README.md")

	contents, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("S7.3: reading %s: %v", readmePath, err)
	}
	text := string(contents)

	if !strings.Contains(text, "## Decisions") {
		t.Fatalf("S7.3: README.md is missing a %q heading", "## Decisions")
	}
	if !strings.Contains(text, "## Not done") {
		t.Fatalf("S7.3: README.md is missing a %q heading", "## Not done")
	}
}

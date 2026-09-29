package phase_tests

// This file holds the fixture shared by phase 2's acceptance tests (and available to any later
// phase's tests that want the same world). It inserts directly through pool, bypassing the HTTP
// API entirely: phase 2's tests exercise the API's read and write paths, not its seeding.

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// World is the fixture seedWorld builds:
//
//   - a customer organization that owns one program, and two vendor organizations the program is
//     shared with (A as editor, B as viewer);
//   - four users: Alice owns the customer org, Bob belongs to vendor A, Carol belongs to vendor B,
//     and Dave belongs to no organization at all, a stranger to the program;
//   - six tasks in that program, forming a diamond dependency graph (Root, depended on by both
//     BranchA and BranchB, both of which are depended on by Join) plus two tasks outside the
//     diamond (Independent, with no dependency relationships at all, and Blocked, whose status is
//     "blocked" despite also having no predecessors). Every task starts "todo" except Blocked,
//     which starts "blocked". Root carries the fixture's one internal note, and task ownership is
//     deliberately spread across all three organizations so that S2.8's tenant rules are
//     exercised on more than one owner. The six tasks are created a minute apart, in the order
//     listed above, so S2.2's created_at tiebreak has something real to order.
//
// Nothing about how these rows got into the database is part of what a test may assume; only the
// ids, the relationships, and the statuses named here are.
type World struct {
	CustomerOrgID string
	VendorAOrgID  string
	VendorBOrgID  string

	AliceID string // customer org owner; can see every task's internal note on this program
	BobID   string // vendor A member; program shared to vendor A as editor
	CarolID string // vendor B member; program shared to vendor B as viewer
	DaveID  string // belongs to no organization; a stranger to the program

	ProgramID string

	// The diamond: Root has no predecessor. BranchA and BranchB each depend on Root. Join depends
	// on both BranchA and BranchB, so it becomes ready only once both branches are done.
	RootTaskID    string
	BranchATaskID string
	BranchBTaskID string
	JoinTaskID    string

	// Two tasks outside the diamond. Independent has no predecessors and nothing depends on it, so
	// it is ready as soon as it exists. Blocked also has no predecessors, but its status is
	// "blocked", which must never appear on the ready list regardless of its (lack of)
	// predecessors.
	IndependentTaskID string
	BlockedTaskID     string
}

// seedWorld builds and returns the fixture described by World.
func seedWorld(t *testing.T, pool *pgxpool.Pool) World {
	t.Helper()

	var w World
	w.CustomerOrgID = insertOrg(t, pool, "Customer Co")
	w.VendorAOrgID = insertOrg(t, pool, "Vendor A Engines")
	w.VendorBOrgID = insertOrg(t, pool, "Vendor B Fabrication")

	w.AliceID = insertUser(t, pool, "alice@customer.example", "Alice Customer")
	w.BobID = insertUser(t, pool, "bob@vendora.example", "Bob VendorA")
	w.CarolID = insertUser(t, pool, "carol@vendorb.example", "Carol VendorB")
	w.DaveID = insertUser(t, pool, "dave@nowhere.example", "Dave Stranger")

	mustInsertMembership(t, pool, w.AliceID, w.CustomerOrgID, "owner")
	mustInsertMembership(t, pool, w.BobID, w.VendorAOrgID, "member")
	mustInsertMembership(t, pool, w.CarolID, w.VendorBOrgID, "member")
	// Dave deliberately joins no organization at all.

	w.ProgramID = insertProgram(t, pool, w.CustomerOrgID, "Engine Overhaul")
	mustInsertProgramShare(t, pool, w.ProgramID, w.VendorAOrgID, "editor")
	mustInsertProgramShare(t, pool, w.ProgramID, w.VendorBOrgID, "viewer")

	// The fixture's tasks are created a minute apart, oldest first, so S2.2's created_at tiebreak
	// is a real ordering an implementation can get wrong. Seeded within one instant, as they were
	// before, any order at all would satisfy the test by accident.
	base := time.Now().Add(-time.Hour).Truncate(time.Second)
	at := func(minutes int) *time.Time {
		moment := base.Add(time.Duration(minutes) * time.Minute)
		return &moment
	}

	w.RootTaskID = seedTask(t, pool, seedTaskArgs{
		ProgramID:    w.ProgramID,
		OrgID:        w.CustomerOrgID,
		Title:        "Draft engine spec",
		Status:       "todo",
		InternalNote: strPtr("Vendor B under-bid this contract; do not disclose the target margin."),
		CreatedAt:    at(0),
	})
	w.BranchATaskID = seedTask(t, pool, seedTaskArgs{
		ProgramID: w.ProgramID,
		OrgID:     w.VendorAOrgID,
		Title:     "Machine the housing",
		Status:    "todo",
		CreatedAt: at(1),
	})
	w.BranchBTaskID = seedTask(t, pool, seedTaskArgs{
		ProgramID: w.ProgramID,
		OrgID:     w.VendorBOrgID,
		Title:     "Cast the manifold",
		Status:    "todo",
		CreatedAt: at(2),
	})
	w.JoinTaskID = seedTask(t, pool, seedTaskArgs{
		ProgramID: w.ProgramID,
		OrgID:     w.CustomerOrgID,
		Title:     "Assemble and test",
		Status:    "todo",
		CreatedAt: at(3),
	})
	w.IndependentTaskID = seedTask(t, pool, seedTaskArgs{
		ProgramID: w.ProgramID,
		OrgID:     w.VendorAOrgID,
		Title:     "Order replacement gaskets",
		Status:    "todo",
		CreatedAt: at(4),
	})
	w.BlockedTaskID = seedTask(t, pool, seedTaskArgs{
		ProgramID: w.ProgramID,
		OrgID:     w.CustomerOrgID,
		Title:     "Certify for export",
		Status:    "blocked",
		CreatedAt: at(5),
	})

	mustInsertDependency(t, pool, w.RootTaskID, w.BranchATaskID)
	mustInsertDependency(t, pool, w.RootTaskID, w.BranchBTaskID)
	mustInsertDependency(t, pool, w.BranchATaskID, w.JoinTaskID)
	mustInsertDependency(t, pool, w.BranchBTaskID, w.JoinTaskID)

	return w
}

// mustInsertProgramShare inserts a program_shares row, failing the test on error. Phase 1's
// helpers include tryInsertProgramShare (used there to test the role check) but nothing that
// fails loudly on an unexpected error, which is what fixture setup wants here.
func mustInsertProgramShare(t *testing.T, pool *pgxpool.Pool, programID, orgID, role string) {
	t.Helper()
	if err := tryInsertProgramShare(t, pool, programID, orgID, role); err != nil {
		t.Fatalf("fixture: insert program share: %v", err)
	}
}

// seedTaskArgs is the full set of columns seedTask can set, beyond what phase 1's insertTask
// helper supports (which only ever inserts a bare, "todo" task by title).
type seedTaskArgs struct {
	ProgramID       string
	OrgID           string
	Title           string
	Status          string
	EstimateMinutes *int
	AssigneeID      *string
	DueOn           *time.Time
	InternalNote    *string
	// CreatedAt sets both created_at and updated_at. When nil, seedTask uses time.Now(), which
	// is what a one-off task in a single test wants; the fixture below sets it explicitly so its
	// rows are ordered rather than all landing within the same instant.
	CreatedAt *time.Time
}

// seedTask inserts a task with every fixture-relevant column, failing the test on error.
func seedTask(t *testing.T, pool *pgxpool.Pool, args seedTaskArgs) string {
	t.Helper()
	now := time.Now()
	if args.CreatedAt != nil {
		now = *args.CreatedAt
	}
	var id string
	err := pool.QueryRow(t.Context(), `
		insert into tasks (
			id, program_id, org_id, title, status, estimate_minutes, assignee_id, due_on,
			internal_note, created_at, updated_at
		)
		values (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		returning id
	`, args.ProgramID, args.OrgID, args.Title, args.Status, args.EstimateMinutes, args.AssigneeID,
		args.DueOn, args.InternalNote, now).Scan(&id)
	if err != nil {
		t.Fatalf("fixture: insert task %q: %v", args.Title, err)
	}
	return id
}

// strPtr is a small convenience for building the *string fields seedTaskArgs takes.
func strPtr(s string) *string { return &s }

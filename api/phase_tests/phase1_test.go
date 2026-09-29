package phase_tests

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/look-itsaxiom/program-control/api/internal/testdb"
)

// This file holds the acceptance tests for phase 1 (schema). They exercise only the contract in
// phases/1-schema/STORIES.md: table and column names, nullability, and the behavioural rules
// (constraints, cascades, the automatic updated_at, and the indexed board query). They do not
// assume anything about how a rule is implemented (trigger vs. application code, enum vs. text,
// which index type), only that the database behaves the way the contract promises.

// --- shape helpers -----------------------------------------------------

// wantColumn is one column this phase's contract requires a table to have.
type wantColumn struct {
	name     string
	dataType string // as reported by information_schema.columns.data_type
	nullable bool
}

// assertTableShape fails the test, naming story, with a message identifying the missing or
// mismatched column, if table does not have every column in want with the right type and
// nullability.
func assertTableShape(t *testing.T, pool *pgxpool.Pool, story, table string, want []wantColumn) {
	t.Helper()
	for _, w := range want {
		var dataType, isNullable string
		err := pool.QueryRow(t.Context(), `
			select data_type, is_nullable
			from information_schema.columns
			where table_schema = 'public' and table_name = $1 and column_name = $2
		`, table, w.name).Scan(&dataType, &isNullable)
		if err != nil {
			t.Fatalf("%s: %s table is missing column %q", story, table, w.name)
		}
		if !strings.EqualFold(dataType, w.dataType) {
			t.Fatalf("%s: %s.%s has type %q, want %q", story, table, w.name, dataType, w.dataType)
		}
		gotNullable := isNullable == "YES"
		if gotNullable != w.nullable {
			t.Fatalf("%s: %s.%s is nullable=%v, want nullable=%v", story, table, w.name, gotNullable, w.nullable)
		}
	}
}

// --- fixture helpers -----------------------------------------------------
//
// Each of these inserts exactly the columns the contract names, explicitly, so a test never
// relies on a default the contract does not promise (id generation in particular: tests always
// supply gen_random_uuid() themselves rather than assuming the schema has a column default).

func insertOrg(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(t.Context(), `
		insert into organizations (id, name, created_at) values (gen_random_uuid(), $1, $2)
		returning id
	`, name, time.Now()).Scan(&id)
	if err != nil {
		t.Fatalf("fixture: insert organization %q: %v", name, err)
	}
	return id
}

func tryInsertOrg(t *testing.T, pool *pgxpool.Pool, name string) error {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
		insert into organizations (id, name, created_at) values (gen_random_uuid(), $1, $2)
	`, name, time.Now())
	return err
}

func insertUser(t *testing.T, pool *pgxpool.Pool, email, displayName string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(t.Context(), `
		insert into users (id, email, display_name) values (gen_random_uuid(), $1, $2)
		returning id
	`, email, displayName).Scan(&id)
	if err != nil {
		t.Fatalf("fixture: insert user %q: %v", email, err)
	}
	return id
}

func tryInsertUser(t *testing.T, pool *pgxpool.Pool, email, displayName string) error {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
		insert into users (id, email, display_name) values (gen_random_uuid(), $1, $2)
	`, email, displayName)
	return err
}

func tryInsertMembership(t *testing.T, pool *pgxpool.Pool, userID, orgID, role string) error {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
		insert into memberships (user_id, org_id, role) values ($1, $2, $3)
	`, userID, orgID, role)
	return err
}

func mustInsertMembership(t *testing.T, pool *pgxpool.Pool, userID, orgID, role string) {
	t.Helper()
	if err := tryInsertMembership(t, pool, userID, orgID, role); err != nil {
		t.Fatalf("fixture: insert membership: %v", err)
	}
}

func tryInsertProgram(t *testing.T, pool *pgxpool.Pool, orgID, name string) error {
	t.Helper()
	now := time.Now()
	_, err := pool.Exec(t.Context(), `
		insert into programs (id, org_id, name, created_at, updated_at)
		values (gen_random_uuid(), $1, $2, $3, $3)
	`, orgID, name, now)
	return err
}

func insertProgram(t *testing.T, pool *pgxpool.Pool, orgID, name string) string {
	t.Helper()
	now := time.Now()
	var id string
	err := pool.QueryRow(t.Context(), `
		insert into programs (id, org_id, name, created_at, updated_at)
		values (gen_random_uuid(), $1, $2, $3, $3)
		returning id
	`, orgID, name, now).Scan(&id)
	if err != nil {
		t.Fatalf("fixture: insert program %q: %v", name, err)
	}
	return id
}

func tryInsertProgramShare(t *testing.T, pool *pgxpool.Pool, programID, orgID, role string) error {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
		insert into program_shares (program_id, org_id, role) values ($1, $2, $3)
	`, programID, orgID, role)
	return err
}

func insertTask(t *testing.T, pool *pgxpool.Pool, programID, orgID, title string) string {
	t.Helper()
	now := time.Now()
	var id string
	err := pool.QueryRow(t.Context(), `
		insert into tasks (id, program_id, org_id, title, status, created_at, updated_at)
		values (gen_random_uuid(), $1, $2, $3, 'todo', $4, $4)
		returning id
	`, programID, orgID, title, now).Scan(&id)
	if err != nil {
		t.Fatalf("fixture: insert task %q: %v", title, err)
	}
	return id
}

func tryInsertTaskWithTitle(t *testing.T, pool *pgxpool.Pool, programID, orgID, title string) error {
	t.Helper()
	now := time.Now()
	_, err := pool.Exec(t.Context(), `
		insert into tasks (id, program_id, org_id, title, status, created_at, updated_at)
		values (gen_random_uuid(), $1, $2, $3, 'todo', $4, $4)
	`, programID, orgID, title, now)
	return err
}

func tryInsertDependency(t *testing.T, pool *pgxpool.Pool, predecessorID, successorID string) error {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
		insert into task_dependencies (predecessor_id, successor_id) values ($1, $2)
	`, predecessorID, successorID)
	return err
}

func mustInsertDependency(t *testing.T, pool *pgxpool.Pool, predecessorID, successorID string) {
	t.Helper()
	if err := tryInsertDependency(t, pool, predecessorID, successorID); err != nil {
		t.Fatalf("fixture: insert dependency: %v", err)
	}
}

func tryInsertTaskEvent(t *testing.T, pool *pgxpool.Pool, taskID, kind string) (int64, error) {
	t.Helper()
	var id int64
	err := pool.QueryRow(t.Context(), `
		insert into task_events (task_id, kind, created_at) values ($1, $2, $3)
		returning id
	`, taskID, kind, time.Now()).Scan(&id)
	return id, err
}

// --- S1.1: organizations, users, and memberships -----------------------

func TestPhase1_OrganizationsTableShape(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)
	assertTableShape(t, pool, "S1.1", "organizations", []wantColumn{
		{"id", "uuid", false},
		{"name", "text", false},
		{"created_at", "timestamp with time zone", false},
	})
}

func TestPhase1_UsersTableShape(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)
	assertTableShape(t, pool, "S1.1", "users", []wantColumn{
		{"id", "uuid", false},
		{"email", "text", false},
		{"display_name", "text", false},
	})
}

func TestPhase1_MembershipsTableShape(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)
	assertTableShape(t, pool, "S1.1", "memberships", []wantColumn{
		{"user_id", "uuid", false},
		{"org_id", "uuid", false},
		{"role", "text", false},
	})
}

func TestPhase1_OrganizationNameMustBeUnique(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	insertOrg(t, pool, "Acme Engines")
	if err := tryInsertOrg(t, pool, "Acme Engines"); err == nil {
		t.Fatal("S1.1: inserting a second organization named \"Acme Engines\" should have been refused")
	}
}

func TestPhase1_UserEmailMustBeUnique(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	insertUser(t, pool, "engineer@example.com", "Pat Engineer")
	if err := tryInsertUser(t, pool, "engineer@example.com", "Someone Else"); err == nil {
		t.Fatal("S1.1: inserting a second user with email \"engineer@example.com\" should have been refused")
	}
}

func TestPhase1_MembershipAcceptsValidRoles(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	for _, role := range []string{"owner", "admin", "member", "viewer"} {
		t.Run(role, func(t *testing.T) {
			org := insertOrg(t, pool, "Org for "+role)
			user := insertUser(t, pool, role+"@example.com", "Test User")
			if err := tryInsertMembership(t, pool, user, org, role); err != nil {
				t.Fatalf("S1.1: membership with role %q should have been accepted: %v", role, err)
			}
		})
	}
}

func TestPhase1_MembershipRoleRejectsInvalidValue(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	org := insertOrg(t, pool, "Acme Engines")
	user := insertUser(t, pool, "engineer@example.com", "Pat Engineer")
	if err := tryInsertMembership(t, pool, user, org, "superadmin"); err == nil {
		t.Fatal("S1.1: a membership with role \"superadmin\" should have been refused")
	}
}

func TestPhase1_MembershipPreventsDuplicateForSamePair(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	org := insertOrg(t, pool, "Acme Engines")
	user := insertUser(t, pool, "engineer@example.com", "Pat Engineer")
	mustInsertMembership(t, pool, user, org, "member")
	if err := tryInsertMembership(t, pool, user, org, "admin"); err == nil {
		t.Fatal("S1.1: a second membership row for the same user and organization should have been refused")
	}
}

// --- S1.2: programs, owned and shared -----------------------------------

func TestPhase1_ProgramsTableShape(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)
	assertTableShape(t, pool, "S1.2", "programs", []wantColumn{
		{"id", "uuid", false},
		{"org_id", "uuid", false},
		{"name", "text", false},
		{"created_at", "timestamp with time zone", false},
		{"updated_at", "timestamp with time zone", false},
		{"deleted_at", "timestamp with time zone", true},
	})
}

func TestPhase1_ProgramSharesTableShape(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)
	assertTableShape(t, pool, "S1.2", "program_shares", []wantColumn{
		{"program_id", "uuid", false},
		{"org_id", "uuid", false},
		{"role", "text", false},
	})
}

func TestPhase1_ProgramNameUniquePerOrganization(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	insertProgram(t, pool, customer, "Engine Test Campaign")
	if err := tryInsertProgram(t, pool, customer, "Engine Test Campaign"); err == nil {
		t.Fatal("S1.2: a second active program named \"Engine Test Campaign\" owned by the same organization should have been refused")
	}

	otherOrg := insertOrg(t, pool, "Other Co")
	if err := tryInsertProgram(t, pool, otherOrg, "Engine Test Campaign"); err != nil {
		t.Fatalf("S1.2: a program with the same name owned by a different organization should have been accepted: %v", err)
	}
}

func TestPhase1_ProgramShareAcceptsValidRoles(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	for _, role := range []string{"editor", "viewer"} {
		t.Run(role, func(t *testing.T) {
			customer := insertOrg(t, pool, "Customer for "+role)
			vendor := insertOrg(t, pool, "Vendor for "+role)
			program := insertProgram(t, pool, customer, "Campaign")
			if err := tryInsertProgramShare(t, pool, program, vendor, role); err != nil {
				t.Fatalf("S1.2: a program share with role %q should have been accepted: %v", role, err)
			}
		})
	}
}

func TestPhase1_ProgramShareRoleRejectsInvalidValue(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	vendor := insertOrg(t, pool, "Vendor Co")
	program := insertProgram(t, pool, customer, "Campaign")
	if err := tryInsertProgramShare(t, pool, program, vendor, "owner"); err == nil {
		t.Fatal("S1.2: a program share with role \"owner\" should have been refused")
	}
}

func TestPhase1_ProgramDeleteCascadesToTasksAndShares(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	vendor := insertOrg(t, pool, "Vendor Co")
	program := insertProgram(t, pool, customer, "Campaign")
	if err := tryInsertProgramShare(t, pool, program, vendor, "viewer"); err != nil {
		t.Fatalf("fixture: insert program share: %v", err)
	}
	insertTask(t, pool, program, customer, "Task one")
	insertTask(t, pool, program, customer, "Task two")

	if _, err := pool.Exec(t.Context(), `delete from programs where id = $1`, program); err != nil {
		t.Fatalf("S1.2: deleting the program failed: %v", err)
	}

	var taskCount, shareCount int
	if err := pool.QueryRow(t.Context(), `select count(*) from tasks where program_id = $1`, program).Scan(&taskCount); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("S1.2: deleting a program should remove its tasks; %d remain", taskCount)
	}
	if err := pool.QueryRow(t.Context(), `select count(*) from program_shares where program_id = $1`, program).Scan(&shareCount); err != nil {
		t.Fatalf("count program_shares: %v", err)
	}
	if shareCount != 0 {
		t.Fatalf("S1.2: deleting a program should remove its shares; %d remain", shareCount)
	}
}

func TestPhase1_OrganizationDeleteRestrictedWhileOwningPrograms(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	insertProgram(t, pool, customer, "Campaign")

	if _, err := pool.Exec(t.Context(), `delete from organizations where id = $1`, customer); err == nil {
		t.Fatal("S1.2: deleting an organization that still owns a program should have been refused")
	}
}

// --- S1.3: tasks ----------------------------------------------------------

func TestPhase1_TasksTableShape(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)
	assertTableShape(t, pool, "S1.3", "tasks", []wantColumn{
		{"id", "uuid", false},
		{"program_id", "uuid", false},
		{"org_id", "uuid", false},
		{"title", "text", false},
		{"status", "text", false},
		{"estimate_minutes", "integer", true},
		{"assignee_id", "uuid", true},
		{"due_on", "date", true},
		{"internal_note", "text", true},
		{"created_at", "timestamp with time zone", false},
		{"updated_at", "timestamp with time zone", false},
		{"deleted_at", "timestamp with time zone", true},
	})
}

func TestPhase1_TaskTitleLengthConstraint(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")

	if err := tryInsertTaskWithTitle(t, pool, program, customer, ""); err == nil {
		t.Fatal("S1.3: a task with an empty title should have been refused")
	}
	if err := tryInsertTaskWithTitle(t, pool, program, customer, strings.Repeat("x", 201)); err == nil {
		t.Fatal("S1.3: a task with a 201-character title should have been refused")
	}
	if err := tryInsertTaskWithTitle(t, pool, program, customer, "Reasonable title"); err != nil {
		t.Fatalf("S1.3: a task with a reasonable title should have been accepted: %v", err)
	}
}

func TestPhase1_TaskStatusRejectsInvalidValue(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")

	_, err := pool.Exec(t.Context(), `
		insert into tasks (id, program_id, org_id, title, status, created_at, updated_at)
		values (gen_random_uuid(), $1, $2, 'Bad status task', 'archived', $3, $3)
	`, program, customer, time.Now())
	if err == nil {
		t.Fatal("S1.3: a task with status \"archived\" should have been refused")
	}
}

func TestPhase1_TaskStatusDefaultsToTodo(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")

	var id string
	err := pool.QueryRow(t.Context(), `
		insert into tasks (id, program_id, org_id, title, created_at, updated_at)
		values (gen_random_uuid(), $1, $2, 'No status given', $3, $3)
		returning id
	`, program, customer, time.Now()).Scan(&id)
	if err != nil {
		t.Fatalf("S1.3: inserting a task without a status should have been accepted: %v", err)
	}

	var status string
	if err := pool.QueryRow(t.Context(), `select status from tasks where id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("read task status: %v", err)
	}
	if status != "todo" {
		t.Fatalf("S1.3: a task recorded without a status should default to \"todo\", got %q", status)
	}
}

func TestPhase1_TaskEstimateRejectsNegative(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")

	_, err := pool.Exec(t.Context(), `
		insert into tasks (id, program_id, org_id, title, status, estimate_minutes, created_at, updated_at)
		values (gen_random_uuid(), $1, $2, 'Negative estimate', 'todo', -1, $3, $3)
	`, program, customer, time.Now())
	if err == nil {
		t.Fatal("S1.3: a task with estimate_minutes = -1 should have been refused")
	}

	_, err = pool.Exec(t.Context(), `
		insert into tasks (id, program_id, org_id, title, status, estimate_minutes, created_at, updated_at)
		values (gen_random_uuid(), $1, $2, 'Zero estimate', 'todo', 0, $3, $3)
	`, program, customer, time.Now())
	if err != nil {
		t.Fatalf("S1.3: a task with estimate_minutes = 0 should have been accepted: %v", err)
	}
}

func TestPhase1_TaskAssigneeClearedWhenUserDeleted(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")
	user := insertUser(t, pool, "assignee@example.com", "Assignee")

	var taskID string
	now := time.Now()
	err := pool.QueryRow(t.Context(), `
		insert into tasks (id, program_id, org_id, title, status, assignee_id, created_at, updated_at)
		values (gen_random_uuid(), $1, $2, 'Assigned task', 'todo', $3, $4, $4)
		returning id
	`, program, customer, user, now).Scan(&taskID)
	if err != nil {
		t.Fatalf("fixture: insert assigned task: %v", err)
	}

	if _, err := pool.Exec(t.Context(), `delete from users where id = $1`, user); err != nil {
		t.Fatalf("S1.3: deleting the assigned user should have been accepted, not blocked by the task: %v", err)
	}

	var assignee *string
	if err := pool.QueryRow(t.Context(), `select assignee_id from tasks where id = $1`, taskID).Scan(&assignee); err != nil {
		t.Fatalf("read task assignee: %v", err)
	}
	if assignee != nil {
		t.Fatalf("S1.3: a task's assignee should become empty once that user is deleted, got %q", *assignee)
	}
}

// --- S1.6: the database refuses bad data (active title uniqueness) ------

func TestPhase1_DuplicateActiveTaskTitleRejected(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")
	insertTask(t, pool, program, customer, "Inspect fixture")

	if err := tryInsertTaskWithTitle(t, pool, program, customer, "Inspect fixture"); err == nil {
		t.Fatal("S1.6: a second active task titled \"Inspect fixture\" in the same program should have been refused")
	}
}

func TestPhase1_TaskTitleReusableAfterSoftDelete(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")
	original := insertTask(t, pool, program, customer, "Inspect fixture")

	if _, err := pool.Exec(t.Context(), `update tasks set deleted_at = $1 where id = $2`, time.Now(), original); err != nil {
		t.Fatalf("fixture: soft-delete task: %v", err)
	}

	if err := tryInsertTaskWithTitle(t, pool, program, customer, "Inspect fixture"); err != nil {
		t.Fatalf("S1.6: reusing the title \"Inspect fixture\" after the original task was soft-deleted should have been accepted: %v", err)
	}
}

// --- S1.4: dependencies between tasks ------------------------------------

func TestPhase1_TaskDependenciesTableShape(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)
	assertTableShape(t, pool, "S1.4", "task_dependencies", []wantColumn{
		{"predecessor_id", "uuid", false},
		{"successor_id", "uuid", false},
	})
}

func TestPhase1_TaskDependencySelfEdgeRejected(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")
	task := insertTask(t, pool, program, customer, "Solo task")

	if err := tryInsertDependency(t, pool, task, task); err == nil {
		t.Fatal("S1.4: a task depending on itself should have been refused")
	}
}

func TestPhase1_TaskDependencyPreventsDuplicatePair(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")
	predecessor := insertTask(t, pool, program, customer, "First task")
	successor := insertTask(t, pool, program, customer, "Second task")
	mustInsertDependency(t, pool, predecessor, successor)

	if err := tryInsertDependency(t, pool, predecessor, successor); err == nil {
		t.Fatal("S1.4: recording the same predecessor/successor pair twice should have been refused")
	}
}

func TestPhase1_TaskDependencyCascadesOnTaskDelete(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")
	predecessor := insertTask(t, pool, program, customer, "First task")
	successor := insertTask(t, pool, program, customer, "Second task")
	mustInsertDependency(t, pool, predecessor, successor)

	if _, err := pool.Exec(t.Context(), `delete from tasks where id = $1`, predecessor); err != nil {
		t.Fatalf("S1.4: deleting the predecessor task failed: %v", err)
	}

	var count int
	err := pool.QueryRow(t.Context(), `
		select count(*) from task_dependencies where predecessor_id = $1 or successor_id = $1
	`, predecessor).Scan(&count)
	if err != nil {
		t.Fatalf("count task_dependencies: %v", err)
	}
	if count != 0 {
		t.Fatalf("S1.4: deleting a task should remove any dependency row it was part of; %d remain", count)
	}
}

// --- S1.5: audit events ----------------------------------------------------

func TestPhase1_TaskEventsTableShape(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)
	assertTableShape(t, pool, "S1.5", "task_events", []wantColumn{
		{"id", "bigint", false},
		{"task_id", "uuid", false},
		{"actor_id", "uuid", true},
		{"kind", "text", false},
		{"from_status", "text", true},
		{"to_status", "text", true},
		{"created_at", "timestamp with time zone", false},
	})
}

func TestPhase1_TaskEventIdentityIDAutoAssigned(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")
	task := insertTask(t, pool, program, customer, "Audited task")

	firstID, err := tryInsertTaskEvent(t, pool, task, "created")
	if err != nil {
		t.Fatalf("S1.5: inserting a task event without an id should have been accepted: %v", err)
	}
	if firstID == 0 {
		t.Fatal("S1.5: a task event's id should be assigned automatically and not be zero")
	}

	secondID, err := tryInsertTaskEvent(t, pool, task, "status_changed")
	if err != nil {
		t.Fatalf("S1.5: inserting a second task event should have been accepted: %v", err)
	}
	if secondID <= firstID {
		t.Fatalf("S1.5: later task events should get later ids; got first=%d second=%d", firstID, secondID)
	}
}

func TestPhase1_TaskEventCascadesOnTaskDelete(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")
	task := insertTask(t, pool, program, customer, "Audited task")
	if _, err := tryInsertTaskEvent(t, pool, task, "created"); err != nil {
		t.Fatalf("fixture: insert task event: %v", err)
	}

	if _, err := pool.Exec(t.Context(), `delete from tasks where id = $1`, task); err != nil {
		t.Fatalf("S1.5: deleting the task failed: %v", err)
	}

	var count int
	if err := pool.QueryRow(t.Context(), `select count(*) from task_events where task_id = $1`, task).Scan(&count); err != nil {
		t.Fatalf("count task_events: %v", err)
	}
	if count != 0 {
		t.Fatalf("S1.5: deleting a task should remove its events; %d remain", count)
	}
}

// --- S1.7: updated_at maintains itself -------------------------------------

func TestPhase1_ProgramUpdatedAtChangesOnUpdate(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	past := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	var programID string
	err := pool.QueryRow(t.Context(), `
		insert into programs (id, org_id, name, created_at, updated_at)
		values (gen_random_uuid(), $1, 'Campaign', $2, $2)
		returning id
	`, customer, past).Scan(&programID)
	if err != nil {
		t.Fatalf("fixture: insert program with past updated_at: %v", err)
	}

	if _, err := pool.Exec(t.Context(), `update programs set name = 'Campaign renamed' where id = $1`, programID); err != nil {
		t.Fatalf("S1.7: updating the program failed: %v", err)
	}

	var updatedAt time.Time
	if err := pool.QueryRow(t.Context(), `select updated_at from programs where id = $1`, programID).Scan(&updatedAt); err != nil {
		t.Fatalf("read program updated_at: %v", err)
	}
	if !updatedAt.After(past) {
		t.Fatalf("S1.7: updating a program should move its updated_at forward; still %v", updatedAt)
	}
}

func TestPhase1_TaskUpdatedAtChangesOnUpdate(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")
	past := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	var taskID string
	err := pool.QueryRow(t.Context(), `
		insert into tasks (id, program_id, org_id, title, status, created_at, updated_at)
		values (gen_random_uuid(), $1, $2, 'Timestamped task', 'todo', $3, $3)
		returning id
	`, program, customer, past).Scan(&taskID)
	if err != nil {
		t.Fatalf("fixture: insert task with past updated_at: %v", err)
	}

	if _, err := pool.Exec(t.Context(), `update tasks set title = 'Timestamped task renamed' where id = $1`, taskID); err != nil {
		t.Fatalf("S1.7: updating the task failed: %v", err)
	}

	var updatedAt time.Time
	if err := pool.QueryRow(t.Context(), `select updated_at from tasks where id = $1`, taskID).Scan(&updatedAt); err != nil {
		t.Fatalf("read task updated_at: %v", err)
	}
	if !updatedAt.After(past) {
		t.Fatalf("S1.7: updating a task should move its updated_at forward; still %v", updatedAt)
	}
}

// --- S1.8: the board query is indexed --------------------------------------

func TestPhase1_TaskBoardQueryUsesIndex(t *testing.T) {
	phaseGate(t, 1)
	pool := testdb.Open(t)

	customer := insertOrg(t, pool, "Customer Co")
	program := insertProgram(t, pool, customer, "Campaign")

	const taskCount = 3000
	_, err := pool.Exec(t.Context(), `
		insert into tasks (id, program_id, org_id, title, status, due_on, created_at, updated_at)
		select
			gen_random_uuid(),
			$1,
			$2,
			'bulk task ' || g,
			(array['todo', 'doing', 'blocked', 'done'])[1 + (g % 4)],
			date '2026-01-01' + (g % 365),
			now(),
			now()
		from generate_series(1, $3) as g
	`, program, customer, taskCount)
	if err != nil {
		t.Fatalf("fixture: bulk insert %d tasks: %v", taskCount, err)
	}

	if _, err := pool.Exec(t.Context(), `analyze tasks`); err != nil {
		t.Fatalf("analyze tasks: %v", err)
	}

	rows, err := pool.Query(t.Context(), `
		explain select * from tasks
		where program_id = $1 and status = $2 and deleted_at is null
		order by due_on
		limit 20
	`, program, "todo")
	if err != nil {
		t.Fatalf("S1.8: explaining the board query failed: %v", err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan explain output: %v", err)
		}
		plan.WriteString(line)
		plan.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read explain output: %v", err)
	}

	planText := plan.String()
	if strings.Contains(planText, "Seq Scan on tasks") {
		t.Fatalf("S1.8: the board query should not scan the whole tasks table; plan was:\n%s", planText)
	}
	if !strings.Contains(planText, "Index") {
		t.Fatalf("S1.8: the board query should use an index; plan was:\n%s", planText)
	}
}

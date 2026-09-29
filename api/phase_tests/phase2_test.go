package phase_tests

// This file holds the acceptance tests for phase 2 (REST API). They exercise only the contract in
// phases/2-rest-api/STORIES.md: routes, request/response shapes, status codes, and the error
// envelope. They call httpapi.NewHandler (through newHandler, in helpers_test.go) and nothing
// else from that package, and they build their fixtures with seedWorld (seed_test.go), never
// through the API itself.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/look-itsaxiom/program-control/api/internal/testdb"
)

// --- shape helpers -----------------------------------------------------

// taskFields is the phase 2 Task JSON shape, decoded via Go's ordinary struct tags. It is not
// used for the "internalNote key is absent, not null" assertions (S2.8), because an absent key
// and an explicit JSON null both decode to a nil pointer here; those assertions decode into a
// map[string]any instead, where key presence can be checked directly.
type taskFields struct {
	ID              string  `json:"id"`
	ProgramID       string  `json:"programId"`
	OrgID           string  `json:"orgId"`
	Title           string  `json:"title"`
	Status          string  `json:"status"`
	EstimateMinutes *int    `json:"estimateMinutes"`
	AssigneeID      *string `json:"assigneeId"`
	DueOn           *string `json:"dueOn"`
	InternalNote    *string `json:"internalNote"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type taskListResponse struct {
	Tasks []taskFields `json:"tasks"`
}

func decodeTask(t *testing.T, rec *httptest.ResponseRecorder) taskFields {
	t.Helper()
	return decode[taskFields](t, rec)
}

func decodeTaskListFields(t *testing.T, rec *httptest.ResponseRecorder) []taskFields {
	t.Helper()
	return decode[taskListResponse](t, rec).Tasks
}

func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	return decode[map[string]any](t, rec)
}

type validationDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func decodeValidationDetails(t *testing.T, rec *httptest.ResponseRecorder) []validationDetail {
	t.Helper()
	var payload struct {
		Error struct {
			Details []validationDetail `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error envelope %q: %v", rec.Body.String(), err)
	}
	return payload.Error.Details
}

// errorMessage extracts the `error.message` field from a JSON error envelope response.
func errorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error envelope %q: %v", rec.Body.String(), err)
	}
	return payload.Error.Message
}

// assertErrorEnvelope fails the test, naming story, unless rec has status wantStatus, a
// Content-Type of application/json, and a body shaped {"error":{"code":"...","message":"..."}}
// with both fields non-empty. It does not check error.code's value; use assertErrorCode when the
// contract fixes it.
func assertErrorEnvelope(t *testing.T, rec *httptest.ResponseRecorder, story string, wantStatus int) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("%s: got status %d, want %d (body %s)", story, rec.Code, wantStatus, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("%s: got Content-Type %q, want %q (body %s)", story, ct, "application/json", rec.Body.String())
	}
	body := decodeMap(t, rec)
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("%s: response body %q has no \"error\" object", story, rec.Body.String())
	}
	if code, _ := errObj["code"].(string); code == "" {
		t.Fatalf("%s: error.code is empty in body %q", story, rec.Body.String())
	}
	if msg, _ := errObj["message"].(string); msg == "" {
		t.Fatalf("%s: error.message is empty in body %q", story, rec.Body.String())
	}
}

// assertErrorCode is assertErrorEnvelope plus a check that error.code is exactly wantCode.
func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, story string, wantStatus int, wantCode string) {
	t.Helper()
	assertErrorEnvelope(t, rec, story, wantStatus)
	if code := errorCode(t, rec); code != wantCode {
		t.Fatalf("%s: got error.code %q, want %q (body %s)", story, code, wantCode, rec.Body.String())
	}
}

// authHeader is the request header map identifying userID as the caller (S2.1).
func authHeader(userID string) map[string]string {
	return map[string]string{"X-User-Id": userID}
}

// readyIDs turns a ready-list response into a set of task ids, for membership checks.
func readyIDs(t *testing.T, rec *httptest.ResponseRecorder) map[string]bool {
	t.Helper()
	tasks := decodeTaskListFields(t, rec)
	ids := make(map[string]bool, len(tasks))
	for _, tk := range tasks {
		ids[tk.ID] = true
	}
	return ids
}

func mustContain(t *testing.T, set map[string]bool, id, msg string) {
	t.Helper()
	if !set[id] {
		t.Fatal(msg)
	}
}

func mustNotContain(t *testing.T, set map[string]bool, id, msg string) {
	t.Helper()
	if set[id] {
		t.Fatal(msg)
	}
}

// --- S2.1: identify the caller -------------------------------------------

func TestPhase2_S2_1_MissingUserHeaderIsUnauthenticated(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks", nil, nil)
	assertErrorCode(t, rec, "S2.1", http.StatusUnauthorized, "unauthenticated")
}

func TestPhase2_S2_1_UnknownUserIsUnauthenticated(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks", nil,
		authHeader("00000000-0000-0000-0000-000000000000"))
	assertErrorCode(t, rec, "S2.1", http.StatusUnauthorized, "unauthenticated")
}

// --- S2.2: list a program's tasks -----------------------------------------

func TestPhase2_S2_2_ListOrdersByDueDateThenCreatedAt(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	due1 := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	due2 := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	early := seedTask(t, pool, seedTaskArgs{
		ProgramID: w.ProgramID, OrgID: w.CustomerOrgID, Title: "Earliest due", Status: "todo", DueOn: &due1,
	})
	later := seedTask(t, pool, seedTaskArgs{
		ProgramID: w.ProgramID, OrgID: w.CustomerOrgID, Title: "Later due", Status: "todo", DueOn: &due2,
	})

	rec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks", nil, authHeader(w.AliceID))
	if rec.Code != http.StatusOK {
		t.Fatalf("S2.2: GET tasks: got status %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	tasks := decodeTaskListFields(t, rec)

	pos := map[string]int{}
	for i, tk := range tasks {
		pos[tk.ID] = i
	}
	if _, ok := pos[early]; !ok {
		t.Fatal("S2.2: the earlier-due task is missing from the list")
	}
	if _, ok := pos[later]; !ok {
		t.Fatal("S2.2: the later-due task is missing from the list")
	}
	if pos[early] >= pos[later] {
		t.Fatal("S2.2: a task with an earlier due date should sort before one with a later due date")
	}
	for _, tk := range tasks {
		if tk.DueOn == nil && pos[tk.ID] < pos[later] {
			t.Fatalf("S2.2: task %s has no due date and should sort after every dated task", tk.ID)
		}
	}
}

func TestPhase2_S2_2_ListFiltersByStatus(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks?status=blocked", nil, authHeader(w.AliceID))
	if rec.Code != http.StatusOK {
		t.Fatalf("S2.2: GET tasks?status=blocked: got status %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	tasks := decodeTaskListFields(t, rec)
	if len(tasks) != 1 || tasks[0].ID != w.BlockedTaskID {
		t.Fatalf("S2.2: status=blocked should return exactly the one blocked task, got %+v", tasks)
	}
	for _, tk := range tasks {
		if tk.Status != "blocked" {
			t.Fatalf("S2.2: status=blocked returned a task with status %q", tk.Status)
		}
	}
}

// --- S2.3: create a task ----------------------------------------------------

func TestPhase2_S2_3_CreateTaskSucceeds(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	body := map[string]any{
		"title":           "Torque test the housing",
		"estimateMinutes": 45,
		"dueOn":           "2026-03-01",
	}
	rec := do(t, h, "POST", "/programs/"+w.ProgramID+"/tasks", body, authHeader(w.AliceID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("S2.3: POST task: got status %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatal("S2.3: POST task: response is missing a Location header")
	}
	created := decodeTask(t, rec)
	if created.Title != "Torque test the housing" {
		t.Fatalf("S2.3: created task title = %q, want %q", created.Title, "Torque test the housing")
	}
	if created.ProgramID != w.ProgramID {
		t.Fatalf("S2.3: created task programId = %q, want %q", created.ProgramID, w.ProgramID)
	}
	if loc != "/tasks/"+created.ID {
		t.Fatalf("S2.3: Location header = %q, want %q", loc, "/tasks/"+created.ID)
	}

	getRec := do(t, h, "GET", "/tasks/"+created.ID, nil, authHeader(w.AliceID))
	if getRec.Code != http.StatusOK {
		t.Fatalf("S2.3: GET the created task: got status %d, want 200 (body %s)", getRec.Code, getRec.Body.String())
	}
	if fetched := decodeTask(t, getRec); fetched.ID != created.ID {
		t.Fatalf("S2.3: fetched task id = %q, want %q", fetched.ID, created.ID)
	}
}

func TestPhase2_S2_3_GetUnknownTaskIs404(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "GET", "/tasks/00000000-0000-0000-0000-000000000000", nil, authHeader(w.AliceID))
	assertErrorEnvelope(t, rec, "S2.3", http.StatusNotFound)
}

func TestPhase2_S2_3_CreateTaskValidatesFields(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	cases := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"empty title", map[string]any{"title": ""}, "title"},
		{"negative estimate", map[string]any{"title": "Valid title", "estimateMinutes": -5}, "estimateMinutes"},
		{"bad due date", map[string]any{"title": "Valid title", "dueOn": "not-a-date"}, "dueOn"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, h, "POST", "/programs/"+w.ProgramID+"/tasks", tc.body, authHeader(w.AliceID))
			assertErrorCode(t, rec, "S2.3", http.StatusBadRequest, "validation")
			details := decodeValidationDetails(t, rec)
			found := false
			for _, d := range details {
				if d.Field == tc.field {
					found = true
					if d.Message == "" {
						t.Fatalf("S2.3: details entry for field %q has an empty message", tc.field)
					}
				}
			}
			if !found {
				t.Fatalf("S2.3: validation details %+v do not name field %q", details, tc.field)
			}
		})
	}
}

func TestPhase2_S2_3_DuplicateTitleIsConflict(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	body := map[string]any{"title": "Draft engine spec"} // matches the seeded Root task's title
	rec := do(t, h, "POST", "/programs/"+w.ProgramID+"/tasks", body, authHeader(w.AliceID))
	assertErrorCode(t, rec, "S2.3", http.StatusConflict, "duplicate_title")
}

// --- S2.4: change status with audit ----------------------------------------

func TestPhase2_S2_4_StatusChangeAppendsAuditEvent(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "PATCH", "/tasks/"+w.BranchATaskID, map[string]any{"status": "doing"}, authHeader(w.BobID))
	if rec.Code != http.StatusOK {
		t.Fatalf("S2.4: PATCH status: got status %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if updated := decodeTask(t, rec); updated.Status != "doing" {
		t.Fatalf("S2.4: updated task status = %q, want %q", updated.Status, "doing")
	}

	var kind, fromStatus, toStatus, actorID string
	err := pool.QueryRow(t.Context(), `
		select kind, from_status, to_status, actor_id
		from task_events
		where task_id = $1
		order by id desc
		limit 1
	`, w.BranchATaskID).Scan(&kind, &fromStatus, &toStatus, &actorID)
	if err != nil {
		t.Fatalf("S2.4: query task_events for the changed task: %v", err)
	}
	if kind != "status_changed" {
		t.Fatalf("S2.4: task_events.kind = %q, want %q", kind, "status_changed")
	}
	if fromStatus != "todo" {
		t.Fatalf("S2.4: task_events.from_status = %q, want %q", fromStatus, "todo")
	}
	if toStatus != "doing" {
		t.Fatalf("S2.4: task_events.to_status = %q, want %q", toStatus, "doing")
	}
	if actorID != w.BobID {
		t.Fatalf("S2.4: task_events.actor_id = %q, want the caller's id %q", actorID, w.BobID)
	}
}

func TestPhase2_S2_4_UnknownStatusIsValidationError(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "PATCH", "/tasks/"+w.BranchATaskID, map[string]any{"status": "archived"}, authHeader(w.AliceID))
	assertErrorCode(t, rec, "S2.4", http.StatusBadRequest, "validation")

	var status string
	if err := pool.QueryRow(t.Context(), `select status from tasks where id = $1`, w.BranchATaskID).Scan(&status); err != nil {
		t.Fatalf("query task status: %v", err)
	}
	if status != "todo" {
		t.Fatalf("S2.4: an invalid status update should leave the task unchanged; status is now %q", status)
	}
}

// --- S2.5: add and remove dependencies with cycle rejection ----------------

func TestPhase2_S2_5_CreateDependencySucceeds(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	body := map[string]any{"predecessorId": w.BlockedTaskID}
	rec := do(t, h, "POST", "/tasks/"+w.IndependentTaskID+"/dependencies", body, authHeader(w.AliceID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("S2.5: POST dependency: got status %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	var payload struct {
		PredecessorID string `json:"predecessorId"`
		SuccessorID   string `json:"successorId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("S2.5: decode dependency response %q: %v", rec.Body.String(), err)
	}
	if payload.PredecessorID != w.BlockedTaskID || payload.SuccessorID != w.IndependentTaskID {
		t.Fatalf("S2.5: dependency response = %+v, want predecessorId %q and successorId %q",
			payload, w.BlockedTaskID, w.IndependentTaskID)
	}

	var count int
	if err := pool.QueryRow(t.Context(), `
		select count(*) from task_dependencies where predecessor_id = $1 and successor_id = $2
	`, w.BlockedTaskID, w.IndependentTaskID).Scan(&count); err != nil {
		t.Fatalf("count task_dependencies: %v", err)
	}
	if count != 1 {
		t.Fatalf("S2.5: expected the dependency row to exist after a successful create, found %d", count)
	}
}

func TestPhase2_S2_5_DuplicateDependencyIsConflict(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	// Root -> BranchA already exists in the seeded fixture.
	body := map[string]any{"predecessorId": w.RootTaskID}
	rec := do(t, h, "POST", "/tasks/"+w.BranchATaskID+"/dependencies", body, authHeader(w.AliceID))
	assertErrorCode(t, rec, "S2.5", http.StatusConflict, "duplicate")
}

func TestPhase2_S2_5_UnknownTaskIs404(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	body := map[string]any{"predecessorId": w.RootTaskID}
	rec := do(t, h, "POST", "/tasks/00000000-0000-0000-0000-000000000000/dependencies", body, authHeader(w.AliceID))
	assertErrorEnvelope(t, rec, "S2.5", http.StatusNotFound)
}

func TestPhase2_S2_5_DirectCycleIsRejected(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	// Root -> BranchA already exists. BranchA -> Root reverses it directly: a two-node cycle.
	body := map[string]any{"predecessorId": w.BranchATaskID}
	rec := do(t, h, "POST", "/tasks/"+w.RootTaskID+"/dependencies", body, authHeader(w.AliceID))
	assertErrorCode(t, rec, "S2.5", http.StatusConflict, "cycle")

	msg := errorMessage(t, rec)
	for _, id := range []string{w.RootTaskID, w.BranchATaskID} {
		if !strings.Contains(msg, id) {
			t.Fatalf("S2.5: cycle error message %q should name task %q, part of the cycle's path", msg, id)
		}
	}
}

func TestPhase2_S2_5_TransitiveCycleIsRejected(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	// Extend the seeded Root -> BranchA edge into a three-link chain: Root -> BranchA ->
	// Independent, with no other predecessor of Independent, so the cycle this provokes has
	// exactly one possible path, unlike the diamond's BranchA/BranchB fork.
	mustInsertDependency(t, pool, w.BranchATaskID, w.IndependentTaskID)

	body := map[string]any{"predecessorId": w.IndependentTaskID}
	rec := do(t, h, "POST", "/tasks/"+w.RootTaskID+"/dependencies", body, authHeader(w.AliceID))
	assertErrorCode(t, rec, "S2.5", http.StatusConflict, "cycle")

	msg := errorMessage(t, rec)
	for _, id := range []string{w.RootTaskID, w.BranchATaskID, w.IndependentTaskID} {
		if !strings.Contains(msg, id) {
			t.Fatalf("S2.5: cycle error message %q should name task %q, part of the cycle's only possible path", msg, id)
		}
	}
}

func TestPhase2_S2_5_DeleteDependencySucceeds(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "DELETE", "/tasks/"+w.BranchATaskID+"/dependencies/"+w.RootTaskID, nil, authHeader(w.AliceID))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("S2.5: DELETE dependency: got status %d, want 204 (body %s)", rec.Code, rec.Body.String())
	}

	var count int
	if err := pool.QueryRow(t.Context(), `
		select count(*) from task_dependencies where predecessor_id = $1 and successor_id = $2
	`, w.RootTaskID, w.BranchATaskID).Scan(&count); err != nil {
		t.Fatalf("count task_dependencies: %v", err)
	}
	if count != 0 {
		t.Fatalf("S2.5: expected the dependency row to be gone after a successful delete, found %d", count)
	}
}

// --- S2.6: ready list --------------------------------------------------------

func TestPhase2_S2_6_ReadyListBeforeAnyProgress(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks/ready", nil, authHeader(w.AliceID))
	if rec.Code != http.StatusOK {
		t.Fatalf("S2.6: GET ready: got status %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	ready := readyIDs(t, rec)

	mustContain(t, ready, w.RootTaskID, "S2.6: Root has no predecessors and is todo; it should be ready")
	mustContain(t, ready, w.IndependentTaskID, "S2.6: Independent has no predecessors and is todo; it should be ready")
	mustNotContain(t, ready, w.BranchATaskID, "S2.6: BranchA's predecessor Root is not done yet; it should not be ready")
	mustNotContain(t, ready, w.BranchBTaskID, "S2.6: BranchB's predecessor Root is not done yet; it should not be ready")
	mustNotContain(t, ready, w.JoinTaskID, "S2.6: Join's predecessors are not done yet; it should not be ready")
	mustNotContain(t, ready, w.BlockedTaskID, "S2.6: Blocked's own status rules it out no matter its predecessors")
}

func TestPhase2_S2_6_ReadyListAfterMarkingRootDone(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "PATCH", "/tasks/"+w.RootTaskID, map[string]any{"status": "done"}, authHeader(w.AliceID))
	if rec.Code != http.StatusOK {
		t.Fatalf("S2.6: PATCH Root to done: got status %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}

	readyRec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks/ready", nil, authHeader(w.AliceID))
	ready := readyIDs(t, readyRec)

	mustContain(t, ready, w.BranchATaskID, "S2.6: BranchA's only predecessor is now done; it should be ready")
	mustContain(t, ready, w.BranchBTaskID, "S2.6: BranchB's only predecessor is now done; it should be ready")
	mustContain(t, ready, w.IndependentTaskID, "S2.6: Independent should stay ready")
	mustNotContain(t, ready, w.RootTaskID, "S2.6: Root is done, not todo; done tasks are never ready")
	mustNotContain(t, ready, w.JoinTaskID, "S2.6: Join still has undone predecessors; it should not be ready")
	mustNotContain(t, ready, w.BlockedTaskID, "S2.6: Blocked should never appear on the ready list")
}

func TestPhase2_S2_6_ReadyListAfterMarkingBothBranchesDone(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	for _, id := range []string{w.RootTaskID, w.BranchATaskID, w.BranchBTaskID} {
		rec := do(t, h, "PATCH", "/tasks/"+id, map[string]any{"status": "done"}, authHeader(w.AliceID))
		if rec.Code != http.StatusOK {
			t.Fatalf("S2.6: PATCH task %s to done: got status %d, want 200 (body %s)", id, rec.Code, rec.Body.String())
		}
	}

	readyRec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks/ready", nil, authHeader(w.AliceID))
	ready := readyIDs(t, readyRec)

	mustContain(t, ready, w.JoinTaskID, "S2.6: Join's predecessors are both done now; it should be ready")
	mustContain(t, ready, w.IndependentTaskID, "S2.6: Independent should stay ready")
	mustNotContain(t, ready, w.RootTaskID, "S2.6: Root is done, not todo")
	mustNotContain(t, ready, w.BranchATaskID, "S2.6: BranchA is done, not todo")
	mustNotContain(t, ready, w.BranchBTaskID, "S2.6: BranchB is done, not todo")
	mustNotContain(t, ready, w.BlockedTaskID, "S2.6: Blocked should never appear on the ready list")
}

// --- S2.7: request ids and consistent errors --------------------------------

func TestPhase2_S2_7_RequestIdIsEchoed(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks", nil, map[string]string{
		"X-User-Id":    w.AliceID,
		"X-Request-Id": "test-request-id-123",
	})
	if got := rec.Header().Get("X-Request-Id"); got != "test-request-id-123" {
		t.Fatalf("S2.7: X-Request-Id = %q, want the echoed value %q", got, "test-request-id-123")
	}
}

func TestPhase2_S2_7_RequestIdIsGeneratedWhenAbsent(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks", nil, authHeader(w.AliceID))
	if got := rec.Header().Get("X-Request-Id"); got == "" {
		t.Fatal("S2.7: a response with no incoming X-Request-Id should still carry a generated one")
	}
}

func TestPhase2_S2_7_WrongMethodIs405WithAllow(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	// PUT is not one of this route's accepted methods (GET, POST).
	rec := do(t, h, "PUT", "/programs/"+w.ProgramID+"/tasks", nil, authHeader(w.AliceID))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("S2.7: PUT on the tasks list route: got status %d, want 405 (body %s)", rec.Code, rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); allow == "" {
		t.Fatal("S2.7: a 405 response should carry an Allow header naming the accepted methods")
	}
}

func TestPhase2_S2_7_UnknownRouteIs404Envelope(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "GET", "/this/route/does/not/exist", nil, authHeader(w.AliceID))
	assertErrorEnvelope(t, rec, "S2.7", http.StatusNotFound)
}

// --- S2.8: tenant safety -----------------------------------------------------

func TestPhase2_S2_8_StrangerGets404OnProgramRoutes(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	listRec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks", nil, authHeader(w.DaveID))
	assertErrorEnvelope(t, listRec, "S2.8", http.StatusNotFound)

	readyRec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks/ready", nil, authHeader(w.DaveID))
	assertErrorEnvelope(t, readyRec, "S2.8", http.StatusNotFound)

	createRec := do(t, h, "POST", "/programs/"+w.ProgramID+"/tasks", map[string]any{"title": "Should never exist"}, authHeader(w.DaveID))
	assertErrorEnvelope(t, createRec, "S2.8", http.StatusNotFound)

	getRec := do(t, h, "GET", "/tasks/"+w.RootTaskID, nil, authHeader(w.DaveID))
	assertErrorEnvelope(t, getRec, "S2.8", http.StatusNotFound)
}

func TestPhase2_S2_8_VendorOmitsInternalNoteKeyInList(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks", nil, authHeader(w.BobID))
	if rec.Code != http.StatusOK {
		t.Fatalf("S2.8: GET tasks as vendor A: got status %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Tasks []map[string]any `json:"tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("S2.8: decode task list %q: %v", rec.Body.String(), err)
	}
	found := false
	for _, tk := range body.Tasks {
		id, _ := tk["id"].(string)
		if id != w.RootTaskID {
			continue
		}
		found = true
		if _, ok := tk["internalNote"]; ok {
			t.Fatalf("S2.8: Root task's JSON should have no \"internalNote\" key for a vendor A caller, got %+v", tk)
		}
	}
	if !found {
		t.Fatal("S2.8: expected the Root task to appear in vendor A's task list")
	}
}

func TestPhase2_S2_8_VendorOmitsInternalNoteKeyOnSingleTask(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "GET", "/tasks/"+w.RootTaskID, nil, authHeader(w.CarolID))
	if rec.Code != http.StatusOK {
		t.Fatalf("S2.8: GET Root task as vendor B: got status %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	m := decodeMap(t, rec)
	if _, ok := m["internalNote"]; ok {
		t.Fatalf("S2.8: Root task's JSON should have no \"internalNote\" key for a vendor B caller, got %+v", m)
	}
}

func TestPhase2_S2_8_OwnerSeesInternalNoteKey(t *testing.T) {
	phaseGate(t, 2)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	rec := do(t, h, "GET", "/tasks/"+w.RootTaskID, nil, authHeader(w.AliceID))
	if rec.Code != http.StatusOK {
		t.Fatalf("S2.8: GET Root task as customer owner: got status %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	m := decodeMap(t, rec)
	note, ok := m["internalNote"]
	if !ok {
		t.Fatal("S2.8: Root task's JSON should include the \"internalNote\" key for a caller in the task's own organization")
	}
	noteStr, isString := note.(string)
	if !isString || noteStr == "" {
		t.Fatalf("S2.8: Root task's internalNote should be the seeded note string, got %v", note)
	}
}

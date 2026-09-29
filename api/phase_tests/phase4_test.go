package phase_tests

// This file holds the acceptance tests for phase 4 (GraphQL). They exercise only the contract in
// phases/4-graphql/STORIES.md: the SDL served at POST /graphql on the same handler, behind the
// same X-User-Id header; GraphQL errors carrying extensions.code (UNAUTHENTICATED, NOT_FOUND,
// CYCLE); and the query-count bound. They do not assume anything about how a resolver is
// implemented (gqlgen or otherwise, one pool or several, which dataloader library, if any), only
// that the handler answers the schema the contract names and installs Config.QueryTracer on every
// connection it opens.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/look-itsaxiom/program-control/api/internal/testdb"
)

// --- GraphQL request/response helpers -----------------------------------

type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphqlError struct {
	Message    string         `json:"message"`
	Extensions map[string]any `json:"extensions"`
}

type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphqlError  `json:"errors"`
}

// doGraphQL issues a single POST /graphql request as userID (omitted from the headers entirely
// when userID is ""), and decodes the GraphQL response envelope.
func doGraphQL(t *testing.T, h http.Handler, userID, query string, variables map[string]any) graphqlResponse {
	t.Helper()
	headers := map[string]string{}
	if userID != "" {
		headers["X-User-Id"] = userID
	}
	rec := do(t, h, "POST", "/graphql", graphqlRequest{Query: query, Variables: variables}, headers)
	return decode[graphqlResponse](t, rec)
}

// errorCodes collects extensions.code from every top-level GraphQL error in resp.
func errorCodes(resp graphqlResponse) []string {
	codes := make([]string, 0, len(resp.Errors))
	for _, e := range resp.Errors {
		if c, ok := e.Extensions["code"].(string); ok {
			codes = append(codes, c)
		}
	}
	return codes
}

func hasErrorCode(resp graphqlResponse, code string) bool {
	for _, c := range errorCodes(resp) {
		if c == code {
			return true
		}
	}
	return false
}

func unmarshalData(t *testing.T, resp graphqlResponse, out any) {
	t.Helper()
	if err := json.Unmarshal(resp.Data, out); err != nil {
		t.Fatalf("decode data %q: %v", string(resp.Data), err)
	}
}

// --- S4.1: expose the graph -----------------------------------------------

func TestPhase4_Me(t *testing.T) {
	phaseGate(t, 4)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	resp := doGraphQL(t, h, w.AliceID, `{ me { id displayName email } }`, nil)
	if len(resp.Errors) > 0 {
		t.Fatalf("S4.1: me query as alice returned errors: %+v", resp.Errors)
	}
	var body struct {
		Me struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			Email       string `json:"email"`
		} `json:"me"`
	}
	unmarshalData(t, resp, &body)
	if body.Me.ID != w.AliceID {
		t.Fatalf("S4.1: me.id = %q, want %q (alice)", body.Me.ID, w.AliceID)
	}
	if body.Me.DisplayName == "" || body.Me.Email == "" {
		t.Fatalf("S4.1: me returned an empty displayName or email: %+v", body.Me)
	}
}

func TestPhase4_Unauthenticated(t *testing.T) {
	phaseGate(t, 4)
	pool := testdb.Open(t)
	seedWorld(t, pool) // a world must exist; this test never uses its ids

	h := newHandler(t)
	resp := doGraphQL(t, h, "", `{ me { id } }`, nil)
	if !hasErrorCode(resp, "UNAUTHENTICATED") {
		t.Fatalf("S4.1: a request with no X-User-Id should fail with extensions.code UNAUTHENTICATED, got errors %+v", resp.Errors)
	}
}

func TestPhase4_UnknownProgramNotFound(t *testing.T) {
	phaseGate(t, 4)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	resp := doGraphQL(t, h, w.AliceID, `query($id: ID!) { program(id: $id) { id } }`,
		map[string]any{"id": "00000000-0000-0000-0000-000000000000"})
	if !hasErrorCode(resp, "NOT_FOUND") {
		t.Fatalf("S4.1: program(id) for an id nothing owns should fail with extensions.code NOT_FOUND, got errors %+v", resp.Errors)
	}
}

// --- S4.2: the board in one request ----------------------------------------

const programNestedQuery = `
	query($id: ID!) {
		program(id: $id) {
			id
			name
			tasks {
				id
				title
				status
				internalNote
				assignee { id displayName }
				predecessors { id title }
			}
		}
	}`

type taskOut struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Status       string  `json:"status"`
	InternalNote *string `json:"internalNote"`
	Assignee     *struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	} `json:"assignee"`
	Predecessors []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"predecessors"`
}

type programOut struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	Tasks []taskOut `json:"tasks"`
}

func TestPhase4_ProgramNestedSelectionAndVendorVisibility(t *testing.T) {
	phaseGate(t, 4)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	// Give the diamond's BranchA task an assignee, so the nested `assignee` selection below has
	// something real to resolve. seedWorld itself never sets an assignee; this only extends what
	// this one test does with the world it already built.
	if _, err := pool.Exec(t.Context(), `update tasks set assignee_id = $2 where id = $1`, w.BranchATaskID, w.BobID); err != nil {
		t.Fatalf("fixture: assign branch A task to bob: %v", err)
	}

	h := newHandler(t)

	// Alice owns the customer org that owns Root, so she sees its internal note.
	resp := doGraphQL(t, h, w.AliceID, programNestedQuery, map[string]any{"id": w.ProgramID})
	if len(resp.Errors) > 0 {
		t.Fatalf("S4.2: program query as alice returned errors: %+v", resp.Errors)
	}
	var aliceView struct {
		Program programOut `json:"program"`
	}
	unmarshalData(t, resp, &aliceView)

	byID := map[string]taskOut{}
	for _, tk := range aliceView.Program.Tasks {
		byID[tk.ID] = tk
	}

	root, ok := byID[w.RootTaskID]
	if !ok {
		t.Fatalf("S4.2: program.tasks (as alice) did not include the Root task; got %d tasks", len(aliceView.Program.Tasks))
	}
	if root.InternalNote == nil || *root.InternalNote == "" {
		t.Fatalf("S4.2: alice, who owns the org that owns Root, should see its internalNote, got null")
	}

	branchA, ok := byID[w.BranchATaskID]
	if !ok {
		t.Fatal("S4.2: program.tasks (as alice) did not include the BranchA task")
	}
	if branchA.Assignee == nil || branchA.Assignee.ID != w.BobID {
		t.Fatalf("S4.2: BranchA task's assignee should be bob, got %+v", branchA.Assignee)
	}

	join, ok := byID[w.JoinTaskID]
	if !ok {
		t.Fatal("S4.2: program.tasks (as alice) did not include the Join task")
	}
	foundBranchA, foundBranchB := false, false
	for _, p := range join.Predecessors {
		if p.ID == w.BranchATaskID {
			foundBranchA = true
		}
		if p.ID == w.BranchBTaskID {
			foundBranchB = true
		}
	}
	if !foundBranchA || !foundBranchB {
		t.Fatalf("S4.2: Join task's predecessors should include both BranchA and BranchB, got %+v", join.Predecessors)
	}

	// Bob belongs to vendor A, which the program is shared with as an editor, but he does not
	// belong to the customer org that owns Root, so its internal note must be null for him.
	respBob := doGraphQL(t, h, w.BobID, programNestedQuery, map[string]any{"id": w.ProgramID})
	if len(respBob.Errors) > 0 {
		t.Fatalf("S4.2: program query as bob returned errors: %+v", respBob.Errors)
	}
	var bobView struct {
		Program programOut `json:"program"`
	}
	unmarshalData(t, respBob, &bobView)
	for _, tk := range bobView.Program.Tasks {
		if tk.ID == w.RootTaskID && tk.InternalNote != nil {
			t.Fatalf("S4.2: bob, outside Root's owning org, should see internalNote as null, got %q", *tk.InternalNote)
		}
	}
}

func TestPhase4_ReadyTasks(t *testing.T) {
	phaseGate(t, 4)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	query := `query($id: ID!) { program(id: $id) { readyTasks { id } } }`
	readySet := func() map[string]bool {
		resp := doGraphQL(t, h, w.AliceID, query, map[string]any{"id": w.ProgramID})
		if len(resp.Errors) > 0 {
			t.Fatalf("S4.2: readyTasks query returned errors: %+v", resp.Errors)
		}
		var body struct {
			Program struct {
				ReadyTasks []struct {
					ID string `json:"id"`
				} `json:"readyTasks"`
			} `json:"program"`
		}
		unmarshalData(t, resp, &body)
		set := map[string]bool{}
		for _, tk := range body.Program.ReadyTasks {
			set[tk.ID] = true
		}
		return set
	}

	before := readySet()
	for _, want := range []string{w.RootTaskID, w.IndependentTaskID} {
		if !before[want] {
			t.Fatalf("S4.2: readyTasks should include %s (todo, no undone predecessor), got %v", want, before)
		}
	}
	for _, notWant := range []string{w.BranchATaskID, w.BranchBTaskID, w.JoinTaskID, w.BlockedTaskID} {
		if before[notWant] {
			t.Fatalf("S4.2: readyTasks should not include %s yet, got %v", notWant, before)
		}
	}

	if _, err := pool.Exec(t.Context(), `update tasks set status = 'done' where id = $1`, w.RootTaskID); err != nil {
		t.Fatalf("fixture: mark Root done: %v", err)
	}

	after := readySet()
	for _, want := range []string{w.BranchATaskID, w.BranchBTaskID, w.IndependentTaskID} {
		if !after[want] {
			t.Fatalf("S4.2: after Root is done, readyTasks should include %s, got %v", want, after)
		}
	}
	if after[w.RootTaskID] {
		t.Fatalf("S4.2: readyTasks should never include an already-done task, got %v", after)
	}
	if after[w.JoinTaskID] {
		t.Fatalf("S4.2: Join should not be ready until both BranchA and BranchB are done, got %v", after)
	}
	if after[w.BlockedTaskID] {
		t.Fatalf("S4.2: a blocked task must never appear on the ready list, got %v", after)
	}
}

// --- S4.3: mutate through the graph -----------------------------------------

func TestPhase4_SetTaskStatusAppendsEvent(t *testing.T) {
	phaseGate(t, 4)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	var before int
	if err := pool.QueryRow(t.Context(), `select count(*) from task_events where task_id = $1`, w.IndependentTaskID).Scan(&before); err != nil {
		t.Fatalf("count events before: %v", err)
	}

	mutation := `mutation($id: ID!, $status: TaskStatus!) { setTaskStatus(taskId: $id, status: $status) { id status } }`
	resp := doGraphQL(t, h, w.AliceID, mutation, map[string]any{"id": w.IndependentTaskID, "status": "DOING"})
	if len(resp.Errors) > 0 {
		t.Fatalf("S4.3: setTaskStatus returned errors: %+v", resp.Errors)
	}
	var body struct {
		SetTaskStatus struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"setTaskStatus"`
	}
	unmarshalData(t, resp, &body)
	if body.SetTaskStatus.Status != "DOING" {
		t.Fatalf("S4.3: setTaskStatus returned status %q, want %q", body.SetTaskStatus.Status, "DOING")
	}

	var after int
	if err := pool.QueryRow(t.Context(), `select count(*) from task_events where task_id = $1`, w.IndependentTaskID).Scan(&after); err != nil {
		t.Fatalf("count events after: %v", err)
	}
	if after != before+1 {
		t.Fatalf("S4.3: setTaskStatus should append exactly one task_events row, had %d before, %d after", before, after)
	}

	var kind, toStatus string
	err := pool.QueryRow(t.Context(), `
		select kind, to_status from task_events where task_id = $1 order by id desc limit 1
	`, w.IndependentTaskID).Scan(&kind, &toStatus)
	if err != nil {
		t.Fatalf("read latest event: %v", err)
	}
	if kind != "status_changed" || toStatus != "doing" {
		t.Fatalf("S4.3: latest task_events row = {kind: %q, to_status: %q}, want {kind: %q, to_status: %q}",
			kind, toStatus, "status_changed", "doing")
	}
}

func TestPhase4_AddDependencyCycle(t *testing.T) {
	phaseGate(t, 4)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)
	h := newHandler(t)

	// Join is already a transitive successor of Root (Root -> BranchA -> Join). Making Root
	// depend on Join closes a loop back to Root.
	mutation := `mutation($id: ID!, $pred: ID!) { addDependency(taskId: $id, predecessorId: $pred) { id } }`
	resp := doGraphQL(t, h, w.AliceID, mutation, map[string]any{
		"id":   w.RootTaskID,
		"pred": w.JoinTaskID,
	})
	if !hasErrorCode(resp, "CYCLE") {
		t.Fatalf("S4.3: making Root depend on Join, already its transitive successor, should fail with extensions.code CYCLE, got errors %+v", resp.Errors)
	}
}

// --- S4.4: no N+1 -----------------------------------------------------------

func TestPhase4_QueryCountBound(t *testing.T) {
	phaseGate(t, 4)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	// seedWorld leaves 6 tasks on the program; extend it (inside this test only, not seedWorld
	// itself) to 12, each with a real predecessor and most with a real assignee, so the nested
	// selection below actually has assignee and predecessor edges to resolve on every task, not
	// just the original six.
	extra := []struct {
		orgID      string
		title      string
		assigneeID *string
		predID     string
	}{
		{w.CustomerOrgID, "Extra: inspect tolerances", &w.AliceID, w.RootTaskID},
		{w.VendorAOrgID, "Extra: source fasteners", &w.BobID, w.BranchATaskID},
		{w.VendorBOrgID, "Extra: pressure test", &w.CarolID, w.BranchBTaskID},
		{w.CustomerOrgID, "Extra: sign off", &w.AliceID, w.JoinTaskID},
		{w.VendorAOrgID, "Extra: label crates", nil, w.IndependentTaskID},
		{w.CustomerOrgID, "Extra: file export paperwork", nil, w.BlockedTaskID},
	}
	for i, e := range extra {
		id := seedTask(t, pool, seedTaskArgs{
			ProgramID:  w.ProgramID,
			OrgID:      e.orgID,
			Title:      fmt.Sprintf("%s (%d)", e.title, i),
			Status:     "todo",
			AssigneeID: e.assigneeID,
		})
		mustInsertDependency(t, pool, e.predID, id)
	}

	var total int
	if err := pool.QueryRow(t.Context(), `select count(*) from tasks where program_id = $1`, w.ProgramID).Scan(&total); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if total != 12 {
		t.Fatalf("S4.4: fixture setup should leave 12 tasks on the program, got %d", total)
	}

	tracer := &countingTracer{}
	h := newHandlerWithTracer(t, tracer)

	query := `
		query($id: ID!) {
			program(id: $id) {
				tasks {
					id
					assignee { displayName }
					predecessors { title }
				}
			}
		}`
	resp := doGraphQL(t, h, w.AliceID, query, map[string]any{"id": w.ProgramID})
	if len(resp.Errors) > 0 {
		t.Fatalf("S4.4: program query returned errors: %+v", resp.Errors)
	}

	if got := tracer.Count(); got > 4 {
		t.Fatalf("S4.4: loading a 12-task program with tasks, their assignees, and their predecessors issued %d SQL statements, want at most 4 (no N+1 per task)", got)
	}
}

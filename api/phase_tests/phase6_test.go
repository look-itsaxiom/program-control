// This file holds the acceptance tests for phase 6 (permissions and realtime), a stretch phase.
// They exercise only the contract in phases/6-permissions-and-realtime/STORIES.md.
package phase_tests

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/look-itsaxiom/program-control/api/internal/testdb"
)

// --- S6.1: vendors see only what they need ----------------------------------------------------

func TestPhase6_VendorListIsLimitedToOwnedAndPredecessorTasks(t *testing.T) {
	phaseGate(t, 6)

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	rec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks", nil, map[string]string{"X-User-Id": w.BobID})
	if rec.Code != http.StatusOK {
		t.Fatalf("S6.1: vendor A (Bob) lists tasks: got status %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Tasks []map[string]json.RawMessage `json:"tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("S6.1: decode task list: %v", err)
	}

	byID := map[string]map[string]json.RawMessage{}
	for _, task := range body.Tasks {
		var id string
		if err := json.Unmarshal(task["id"], &id); err != nil {
			t.Fatalf("S6.1: task in list has no usable id: %v", err)
		}
		byID[id] = task
	}

	// Bob (vendor A) owns BranchA and Independent: both must appear at full detail.
	for _, owned := range []string{w.BranchATaskID, w.IndependentTaskID} {
		task, ok := byID[owned]
		if !ok {
			t.Fatalf("S6.1: vendor A's own task %s is missing from Bob's list entirely", owned)
		}
		if _, ok := task["estimateMinutes"]; !ok {
			t.Fatalf("S6.1: vendor A's own task %s should carry full detail (estimateMinutes present), got keys %v", owned, keysOf(task))
		}
	}

	// Root is a predecessor of BranchA (which Bob owns): reduced shape only.
	reduced, ok := byID[w.RootTaskID]
	if !ok {
		t.Fatalf("S6.1: Root (a predecessor of vendor A's BranchA) is missing from Bob's list entirely")
	}
	wantKeys := map[string]bool{"id": true, "title": true, "status": true}
	for k := range reduced {
		if !wantKeys[k] {
			t.Fatalf("S6.1: Root, shown to Bob as a predecessor, has extra key %q; reduced shape must be exactly id/title/status, got keys %v", k, keysOf(reduced))
		}
	}
	for k := range wantKeys {
		if _, ok := reduced[k]; !ok {
			t.Fatalf("S6.1: Root, shown to Bob as a predecessor, is missing required key %q, got keys %v", k, keysOf(reduced))
		}
	}

	// Join (a successor of BranchA, not a predecessor of anything Bob owns) and BranchB / Blocked
	// (neither owned by vendor A nor a predecessor of a vendor-A-owned task) must not appear at
	// all.
	for _, hidden := range []string{w.JoinTaskID, w.BranchBTaskID, w.BlockedTaskID} {
		if _, ok := byID[hidden]; ok {
			t.Fatalf("S6.1: task %s should be entirely absent from Bob's list (not owned by vendor A, not a predecessor of a vendor A task)", hidden)
		}
	}
}

func TestPhase6_VendorDirectFetchOfUnrelatedTaskIs404(t *testing.T) {
	phaseGate(t, 6)

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	// Join is a successor of Bob's BranchA task, not a predecessor of anything Bob owns, and is
	// owned by the customer org: entirely outside vendor A's visibility.
	rec := do(t, h, "GET", "/tasks/"+w.JoinTaskID, nil, map[string]string{"X-User-Id": w.BobID})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("S6.1: GET /tasks/{id} for a task outside vendor A's visibility: got status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestPhase6_CustomerMemberSeesEverything(t *testing.T) {
	phaseGate(t, 6)

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	rec := do(t, h, "GET", "/programs/"+w.ProgramID+"/tasks", nil, map[string]string{"X-User-Id": w.AliceID})
	if rec.Code != http.StatusOK {
		t.Fatalf("S6.1: customer owner (Alice) lists tasks: got status %d, want %d", rec.Code, http.StatusOK)
	}
	var body struct {
		Tasks []map[string]json.RawMessage `json:"tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("S6.1: decode task list: %v", err)
	}
	if len(body.Tasks) != 6 {
		t.Fatalf("S6.1: customer owner should see all 6 seeded tasks, got %d", len(body.Tasks))
	}
}

// --- S6.2: audit trail --------------------------------------------------------------------------

func TestPhase6_EventsEndpointPaginatesWithSince(t *testing.T) {
	phaseGate(t, 6)

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	// Produce three ordered events so `since` has something meaningful to skip.
	statuses := []string{"doing", "blocked", "doing"}
	for _, status := range statuses {
		rec := do(t, h, "PATCH", "/tasks/"+w.RootTaskID, map[string]string{"status": status}, map[string]string{"X-User-Id": w.AliceID})
		if rec.Code != http.StatusOK {
			t.Fatalf("S6.2: fixture PATCH to %q: got status %d, want %d", status, rec.Code, http.StatusOK)
		}
	}

	all := do(t, h, "GET", "/programs/"+w.ProgramID+"/events", nil, map[string]string{"X-User-Id": w.AliceID})
	if all.Code != http.StatusOK {
		t.Fatalf("S6.2: GET events with no since: got status %d, want %d, body %s", all.Code, http.StatusOK, all.Body.String())
	}
	type event struct {
		ID     int64  `json:"id"`
		TaskID string `json:"taskId"`
		Kind   string `json:"kind"`
	}
	allBody := decode[struct {
		Events []event `json:"events"`
	}](t, all)
	if len(allBody.Events) < 3 {
		t.Fatalf("S6.2: expected at least 3 events after 3 status changes, got %d", len(allBody.Events))
	}
	for i := 1; i < len(allBody.Events); i++ {
		if allBody.Events[i].ID <= allBody.Events[i-1].ID {
			t.Fatalf("S6.2: events are not ascending by id: %d then %d", allBody.Events[i-1].ID, allBody.Events[i].ID)
		}
	}

	since := allBody.Events[0].ID
	filtered := do(t, h, "GET", "/programs/"+w.ProgramID+"/events?since="+strconv.FormatInt(since, 10), nil, map[string]string{"X-User-Id": w.AliceID})
	if filtered.Code != http.StatusOK {
		t.Fatalf("S6.2: GET events?since=%d: got status %d, want %d", since, filtered.Code, http.StatusOK)
	}
	filteredBody := decode[struct {
		Events []event `json:"events"`
	}](t, filtered)
	for _, e := range filteredBody.Events {
		if e.ID <= since {
			t.Fatalf("S6.2: events?since=%d returned event id %d, which is not strictly greater than since", since, e.ID)
		}
	}
	if len(filteredBody.Events) != len(allBody.Events)-1 {
		t.Fatalf("S6.2: events?since=%d returned %d events, want %d (one fewer than the unfiltered list)", since, len(filteredBody.Events), len(allBody.Events)-1)
	}
}

// --- S6.3: live board ----------------------------------------------------------------------------

func TestPhase6_StreamEmitsTaskUpdatedOnStatusChange(t *testing.T) {
	phaseGate(t, 6)

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	srv := httptest.NewServer(h)
	defer srv.Close()

	req, err := http.NewRequest("GET", srv.URL+"/programs/"+w.ProgramID+"/stream", nil)
	if err != nil {
		t.Fatalf("S6.3: build stream request: %v", err)
	}
	req.Header.Set("X-User-Id", w.AliceID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("S6.3: open stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("S6.3: GET /programs/{id}/stream: got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("S6.3: GET /programs/{id}/stream: got Content-Type %q, want a text/event-stream prefix", ct)
	}

	type sseResult struct {
		event string
		data  string
	}
	results := make(chan sseResult, 1)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		var currentEvent string
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event:"):
				currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				results <- sseResult{event: currentEvent, data: strings.TrimSpace(strings.TrimPrefix(line, "data:"))}
				return
			}
		}
	}()

	// Give the stream a moment to actually subscribe before triggering the change it should see.
	time.Sleep(150 * time.Millisecond)
	patch := do(t, h, "PATCH", "/tasks/"+w.RootTaskID, map[string]string{"status": "doing"}, map[string]string{"X-User-Id": w.AliceID})
	if patch.Code != http.StatusOK {
		t.Fatalf("S6.3: PATCH to trigger a stream event: got status %d, want %d", patch.Code, http.StatusOK)
	}

	select {
	case result := <-results:
		if result.event != "task.updated" {
			t.Fatalf("S6.3: got SSE event type %q, want %q", result.event, "task.updated")
		}
		var task struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(result.data), &task); err != nil {
			t.Fatalf("S6.3: SSE data was not the Task JSON: %v (data: %s)", err, result.data)
		}
		if task.ID != w.RootTaskID {
			t.Fatalf("S6.3: SSE task.updated named task %q, want %q", task.ID, w.RootTaskID)
		}
		if task.Status != "doing" {
			t.Fatalf("S6.3: SSE task.updated carried status %q, want %q", task.Status, "doing")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("S6.3: no task.updated SSE event arrived within 2s of the PATCH")
	}
}

// keysOf returns the keys of a decoded JSON object, for failure messages.
func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

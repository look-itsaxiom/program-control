// This file holds the acceptance tests for phase 5 (integrations), a stretch phase. They exercise
// only the contract in phases/5-integrations/STORIES.md; they do not assume anything about how a
// rule is implemented.
//
// S5.2's contract configures the downstream notification target through the WEBHOOK_NOTIFY_URL
// environment variable rather than a Go-level dependency on httpapi.Config, deliberately: the
// whole phase_tests package (every phase's file) has to compile together at every point in the
// project's history, including before this phase's own handler exists, so a provided test can
// never reference a struct field the current httpapi.Config does not yet declare. An env-var
// target keeps the notifier fully swappable for a test (an httptest.Server) the same way
// WEBHOOK_SECRET already keeps the signing secret swappable, without adding a compile-time
// dependency from this phase's tests onto a symbol only phase 5's own implementation introduces.
package phase_tests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/look-itsaxiom/program-control/api/internal/testdb"
)

// sign computes the X-Signature this phase's contract requires: hex(HMAC-SHA256("<ts>.<body>")).
func sign(secret, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(body)))
	return hex.EncodeToString(mac.Sum(nil))
}

// webhookBody builds the S5.1 request body for one task.status message.
func webhookBody(id, taskID, status string) []byte {
	buf, _ := json.Marshal(map[string]string{
		"id":     id,
		"type":   "task.status",
		"taskId": taskID,
		"status": status,
	})
	return buf
}

// sendWebhook issues a POST /webhooks/vendor signed with secret at timestamp ts.
func sendWebhook(t *testing.T, h http.Handler, secret string, ts time.Time, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	tsStr := strconv.FormatInt(ts.Unix(), 10)
	sig := sign(secret, tsStr, body)

	req := httptest.NewRequest("POST", "/webhooks/vendor", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", tsStr)
	req.Header.Set("X-Signature", sig)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// --- S5.1: accept a signed vendor webhook -----------------------------------------------------

func TestPhase5_SignatureHappyPathAccepted(t *testing.T) {
	phaseGate(t, 5)
	t.Setenv("WEBHOOK_SECRET", "current-secret")

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	rec := sendWebhook(t, h, "current-secret", time.Now(), webhookBody("msg-1", w.RootTaskID, "doing"))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("S5.1: signed webhook: got status %d, want %d, body %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	got := decode[struct {
		Status string `json:"status"`
	}](t, rec)
	if got.Status != "accepted" {
		t.Fatalf("S5.1: signed webhook: got status field %q, want %q", got.Status, "accepted")
	}

	var status string
	if err := pool.QueryRow(t.Context(), `select status from tasks where id = $1`, w.RootTaskID).Scan(&status); err != nil {
		t.Fatalf("S5.1: querying task status: %v", err)
	}
	if status != "doing" {
		t.Fatalf("S5.1: signed webhook: task status is %q, want %q", status, "doing")
	}
}

func TestPhase5_BadSignatureRejected(t *testing.T) {
	phaseGate(t, 5)
	t.Setenv("WEBHOOK_SECRET", "current-secret")

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	body := webhookBody("msg-bad-sig", w.RootTaskID, "doing")
	ts := time.Now()
	tsStr := strconv.FormatInt(ts.Unix(), 10)
	sig := sign("totally-wrong-secret", tsStr, body)

	req := httptest.NewRequest("POST", "/webhooks/vendor", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", tsStr)
	req.Header.Set("X-Signature", sig)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("S5.1: bad signature: got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestPhase5_StaleTimestampRejected(t *testing.T) {
	phaseGate(t, 5)
	t.Setenv("WEBHOOK_SECRET", "current-secret")

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	staleTS := time.Now().Add(-6 * time.Minute)
	rec := sendWebhook(t, h, "current-secret", staleTS, webhookBody("msg-stale", w.RootTaskID, "doing"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("S5.1: stale timestamp (6 minutes old, valid signature): got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPhase5_RotationAcceptsPreviousSecret(t *testing.T) {
	phaseGate(t, 5)
	t.Setenv("WEBHOOK_SECRET", "new-secret")
	t.Setenv("WEBHOOK_SECRET_PREVIOUS", "old-secret")

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	// Signed with the OLD secret, during rotation; WEBHOOK_SECRET has already moved to the new one.
	rec := sendWebhook(t, h, "old-secret", time.Now(), webhookBody("msg-rotation", w.RootTaskID, "doing"))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("S5.1: webhook signed with WEBHOOK_SECRET_PREVIOUS during rotation: got status %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestPhase5_DuplicateMessageIdSuppressed(t *testing.T) {
	phaseGate(t, 5)
	t.Setenv("WEBHOOK_SECRET", "current-secret")

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	body := webhookBody("msg-dup", w.RootTaskID, "doing")
	ts := time.Now()

	first := sendWebhook(t, h, "current-secret", ts, body)
	if first.Code != http.StatusAccepted {
		t.Fatalf("S5.1: duplicate suppression: first delivery: got status %d, want %d", first.Code, http.StatusAccepted)
	}

	second := sendWebhook(t, h, "current-secret", ts, body)
	if second.Code != http.StatusOK {
		t.Fatalf("S5.1: duplicate suppression: second delivery of the same id: got status %d, want %d", second.Code, http.StatusOK)
	}
	got := decode[struct {
		Status string `json:"status"`
	}](t, second)
	if got.Status != "duplicate" {
		t.Fatalf("S5.1: duplicate suppression: second delivery status field is %q, want %q", got.Status, "duplicate")
	}

	var eventCount int
	if err := pool.QueryRow(t.Context(), `select count(*) from task_events where task_id = $1 and kind = 'webhook'`, w.RootTaskID).Scan(&eventCount); err != nil {
		t.Fatalf("S5.1: counting webhook events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("S5.1: duplicate suppression: got %d webhook task_events rows after 2 deliveries of the same id, want 1", eventCount)
	}
}

func TestPhase5_WebhookEventKindIsWebhook(t *testing.T) {
	phaseGate(t, 5)
	t.Setenv("WEBHOOK_SECRET", "current-secret")

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	rec := sendWebhook(t, h, "current-secret", time.Now(), webhookBody("msg-kind", w.RootTaskID, "doing"))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("S5.1: webhook event kind: webhook delivery: got status %d, want %d", rec.Code, http.StatusAccepted)
	}

	var kind, fromStatus, toStatus string
	var actorID *string
	err := pool.QueryRow(t.Context(), `
		select kind, from_status, to_status, actor_id from task_events
		where task_id = $1 order by id desc limit 1
	`, w.RootTaskID).Scan(&kind, &fromStatus, &toStatus, &actorID)
	if err != nil {
		t.Fatalf("S5.1: reading the webhook's task_events row: %v", err)
	}
	if kind != "webhook" {
		t.Fatalf("S5.1: webhook event kind: got %q, want %q", kind, "webhook")
	}
	if fromStatus != "todo" || toStatus != "doing" {
		t.Fatalf("S5.1: webhook event kind: got from_status=%q to_status=%q, want from_status=%q to_status=%q", fromStatus, toStatus, "todo", "doing")
	}
	if actorID != nil {
		t.Fatalf("S5.1: webhook event kind: got actor_id=%q, want unset (no person performed a webhook-driven change)", *actorID)
	}
}

// --- S5.2 / S5.3: notify downstream without blocking, off the request path -------------------

// notifyReceiver is a downstream stand-in for WEBHOOK_NOTIFY_URL: it counts calls, can be told to
// sleep before answering, and can be told to fail its first N calls.
type notifyReceiver struct {
	srv        *httptest.Server
	calls      int64
	failFirstN int64
	sleep      time.Duration
}

func newNotifyReceiver(t *testing.T) *notifyReceiver {
	t.Helper()
	r := &notifyReceiver{}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		n := atomic.AddInt64(&r.calls, 1)
		if r.sleep > 0 {
			time.Sleep(r.sleep)
		}
		if n <= atomic.LoadInt64(&r.failFirstN) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(r.srv.Close)
	return r
}

func (r *notifyReceiver) callCount() int64 { return atomic.LoadInt64(&r.calls) }

func TestPhase5_NotifierCalledAfterStatusChange(t *testing.T) {
	phaseGate(t, 5)
	receiver := newNotifyReceiver(t)
	t.Setenv("WEBHOOK_NOTIFY_URL", receiver.srv.URL)

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	// A plain PATCH (S2.4), not just a webhook, must trigger the notifier: S5.2's contract covers
	// every status change, however it was made.
	rec := do(t, h, "PATCH", "/tasks/"+w.RootTaskID, map[string]string{"status": "doing"}, map[string]string{"X-User-Id": w.AliceID})
	if rec.Code != http.StatusOK {
		t.Fatalf("S5.2: PATCH /tasks/{id}: got status %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	deadline := time.Now().Add(2 * time.Second)
	for receiver.callCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if receiver.callCount() == 0 {
		t.Fatal("S5.2: notifier: WEBHOOK_NOTIFY_URL received no call within 2s of a successful PATCH /tasks/{id}")
	}
}

func TestPhase5_SlowNotifierDoesNotDelayResponse(t *testing.T) {
	phaseGate(t, 5)
	receiver := newNotifyReceiver(t)
	// Two seconds is far longer than any honest request handler takes, so a response that comes
	// back promptly can only mean the handler did not wait on delivery.
	receiver.sleep = 2 * time.Second
	t.Setenv("WEBHOOK_NOTIFY_URL", receiver.srv.URL)

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	start := time.Now()
	rec := do(t, h, "PATCH", "/tasks/"+w.RootTaskID, map[string]string{"status": "doing"}, map[string]string{"X-User-Id": w.AliceID})
	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Fatalf("S5.2: PATCH /tasks/{id} with a slow notifier: got status %d, want %d", rec.Code, http.StatusOK)
	}
	if elapsed >= 1*time.Second {
		t.Fatalf("S5.2/S5.3: response took %s against a notifier that sleeps 2s; the request must answer without waiting on delivery, well inside 1s", elapsed)
	}

	deadline := time.Now().Add(5 * time.Second)
	for receiver.callCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if receiver.callCount() == 0 {
		t.Fatal("S5.2/S5.3: the slow notifier was still never called within 5s; delivery must still eventually happen off the request path")
	}
}

func TestPhase5_FailingNotifierRetriedThreeTimes(t *testing.T) {
	phaseGate(t, 5)
	receiver := newNotifyReceiver(t)
	receiver.failFirstN = 100 // fail every attempt this test will see
	t.Setenv("WEBHOOK_NOTIFY_URL", receiver.srv.URL)

	h := newHandler(t)
	pool := testdb.Open(t)
	w := seedWorld(t, pool)

	rec := do(t, h, "PATCH", "/tasks/"+w.RootTaskID, map[string]string{"status": "doing"}, map[string]string{"X-User-Id": w.AliceID})
	if rec.Code != http.StatusOK {
		t.Fatalf("S5.2: PATCH /tasks/{id} with a failing notifier: got status %d, want %d", rec.Code, http.StatusOK)
	}

	// Give the retries (3 attempts, with backoff between them) time to play out, then confirm the
	// count settles at exactly 3 and does not keep climbing forever. Ten seconds is the bound
	// S5.2's contract states for all three attempts, so waiting any less would fail an
	// implementation the contract allows.
	deadline := time.Now().Add(10 * time.Second)
	for receiver.callCount() < 3 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if got := receiver.callCount(); got != 3 {
		t.Fatalf("S5.2: failing notifier: got %d attempts within 10s, want exactly 3", got)
	}

	time.Sleep(300 * time.Millisecond)
	if got := receiver.callCount(); got != 3 {
		t.Fatalf("S5.2: failing notifier: got %d attempts after settling, want exactly 3 (retries must be bounded, not infinite)", got)
	}
}

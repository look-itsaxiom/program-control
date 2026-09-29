// Package notify delivers status changes to a downstream system, off the request path. Nothing a
// caller does here blocks: a change is handed to a worker and the request goes on its way.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/look-itsaxiom/program-control/api/internal/store"
)

// attemptBackoff is the pause before each retry. Its length fixes how many attempts a delivery
// gets: one for the first try, one per entry here, and then it is abandoned rather than retried
// forever, because a downstream system that has been down for a minute is an operator's problem,
// not something to keep a queue busy over.
var attemptBackoff = []time.Duration{100 * time.Millisecond, 300 * time.Millisecond}

const (
	queueDepth     = 256
	workerCount    = 4
	requestTimeout = 5 * time.Second
)

// payload is the JSON body a downstream system receives.
type payload struct {
	TaskID     string `json:"taskId"`
	FromStatus string `json:"fromStatus"`
	ToStatus   string `json:"toStatus"`
	Kind       string `json:"kind"`
	OccurredAt string `json:"occurredAt"`
}

// Notifier posts status changes to one URL. A nil *Notifier is a working no-op, so a caller with
// nothing configured never has to check.
type Notifier struct {
	url    string
	client *http.Client
	log    *slog.Logger
	queue  chan store.StatusChange
}

// New returns a notifier for url, or nil when url is empty: nothing is configured to be told, so
// nothing is delivered and nothing fails.
func New(url string, log *slog.Logger) *Notifier {
	if url == "" {
		return nil
	}
	return &Notifier{
		url:    url,
		client: &http.Client{Timeout: requestTimeout},
		log:    log,
		queue:  make(chan store.StatusChange, queueDepth),
	}
}

// Start launches the workers that drain the queue. They stop when ctx is done.
func (n *Notifier) Start(ctx context.Context) {
	if n == nil {
		return
	}
	for range workerCount {
		go n.work(ctx)
	}
}

// Notify hands a change to a worker and returns immediately. It never blocks the caller, so a
// downstream system that is slow, or a queue that is full, can delay a delivery but never a
// response. A dropped change is logged, not silently lost.
func (n *Notifier) Notify(change store.StatusChange) {
	if n == nil {
		return
	}
	select {
	case n.queue <- change:
	default:
		n.log.Warn("notify queue full, dropping change",
			"task_id", change.TaskID, "to_status", change.ToStatus)
	}
}

func (n *Notifier) work(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case change := <-n.queue:
			n.deliver(ctx, change)
		}
	}
}

// deliver posts one change, retrying a bounded number of times with a growing pause.
func (n *Notifier) deliver(ctx context.Context, change store.StatusChange) {
	body, err := json.Marshal(payload{
		TaskID:     change.TaskID,
		FromStatus: change.FromStatus,
		ToStatus:   change.ToStatus,
		Kind:       change.Kind,
		OccurredAt: change.OccurredAt.Format(time.RFC3339),
	})
	if err != nil {
		n.log.Error("encode notification", "error", err, "task_id", change.TaskID)
		return
	}

	for attempt := 0; attempt <= len(attemptBackoff); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(attemptBackoff[attempt-1]):
			}
		}

		if err := n.post(ctx, body); err == nil {
			return
		} else {
			n.log.Warn("notification attempt failed",
				"error", err, "attempt", attempt+1, "task_id", change.TaskID)
		}
	}

	n.log.Error("notification abandoned after every attempt",
		"attempts", len(attemptBackoff)+1, "task_id", change.TaskID)
}

func (n *Notifier) post(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build notification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("post notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("notification target answered %d", resp.StatusCode)
	}
	return nil
}

package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/look-itsaxiom/program-control/api/internal/store"
)

// eventPayload is one row of the audit trail as JSON.
type eventPayload struct {
	ID         int64   `json:"id"`
	TaskID     string  `json:"taskId"`
	Kind       string  `json:"kind"`
	FromStatus *string `json:"fromStatus"`
	ToStatus   *string `json:"toStatus"`
	ActorID    *string `json:"actorId"`
	CreatedAt  string  `json:"createdAt"`
}

// listEvents answers the program's history, narrowed to the tasks this caller may see at all.
func (s *server) listEvents(w http.ResponseWriter, r *http.Request) {
	access, ok := s.programAccess(w, r)
	if !ok {
		return
	}
	vis, ok := s.visibilityFor(w, r, access)
	if !ok {
		return
	}

	var since int64
	if raw := r.URL.Query().Get("since"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			writeValidationError(w, s.log, []fieldError{{
				Field:   "since",
				Message: "must be an event id",
			}})
			return
		}
		since = parsed
	}

	// A nil id list means "every task in the program", which is what the owning organization
	// sees; a vendor gets exactly the ids their own tasks and their predecessors cover.
	var taskIDs []string
	if !vis.owner {
		taskIDs = vis.vendor.VisibleIDs()
		if len(taskIDs) == 0 {
			writeJSON(w, s.log, http.StatusOK, map[string]any{"events": []eventPayload{}})
			return
		}
	}

	events, err := s.store.ListEvents(r.Context(), access.ProgramID, since, taskIDs)
	if err != nil {
		s.fail(w, r, "list events", err)
		return
	}

	payload := make([]eventPayload, 0, len(events))
	for _, e := range events {
		payload = append(payload, eventPayload{
			ID:         e.ID,
			TaskID:     e.TaskID,
			Kind:       e.Kind,
			FromStatus: e.FromStatus,
			ToStatus:   e.ToStatus,
			ActorID:    e.ActorID,
			CreatedAt:  e.CreatedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, s.log, http.StatusOK, map[string]any{"events": payload})
}

// streamProgram holds a connection open and writes one Server-Sent Event per visible status
// change. Visibility is re-resolved per event rather than once at subscribe time, so a task that
// becomes a predecessor of the vendor's work while they are watching starts reaching them.
func (s *server) streamProgram(w http.ResponseWriter, r *http.Request) {
	caller := userFrom(r.Context())
	access, ok := s.programAccess(w, r)
	if !ok {
		return
	}

	// A response controller reaches the flusher through whatever middleware wrapped this writer,
	// which a direct type assertion on w would not.
	controller := http.NewResponseController(w)

	updates, unsubscribe := s.broker.Subscribe(access.ProgramID)
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	if err := controller.Flush(); err != nil {
		s.log.Error("flush stream headers", "error", err, "program_id", access.ProgramID)
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case task, open := <-updates:
			if !open {
				return
			}
			vis, err := s.streamVisibility(r, access)
			if err != nil {
				s.log.Error("stream visibility", "error", err, "program_id", access.ProgramID)
				return
			}
			body := vis.render(task, caller)
			if body == nil {
				continue
			}
			encoded, err := json.Marshal(body)
			if err != nil {
				s.log.Error("encode stream event", "error", err, "task_id", task.ID)
				continue
			}
			if _, err := w.Write([]byte("event: task.updated\ndata: " + string(encoded) + "\n\n")); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
		}
	}
}

func (s *server) streamVisibility(r *http.Request, access store.ProgramAccess) (visibility, error) {
	if access.IsOwner() {
		return visibility{owner: true}, nil
	}
	vendor, err := s.store.VendorVisibilityFor(r.Context(), access.ProgramID, access.CallerOrg)
	if err != nil {
		return visibility{}, err
	}
	return visibility{vendor: vendor}, nil
}

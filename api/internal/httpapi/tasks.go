package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/look-itsaxiom/program-control/api/internal/store"
)

const dateLayout = "2006-01-02"

// taskPayload is the Task JSON every route in this API returns.
//
// InternalNote is json.RawMessage rather than *string on purpose: the contract distinguishes an
// explicit null, meaning "you may see this note and there isn't one", from the key being absent
// altogether, meaning "you are outside this task's organization and may not be told either way".
// A nil RawMessage with omitempty disappears; a RawMessage holding `null` renders as null.
type taskPayload struct {
	ID              string          `json:"id"`
	ProgramID       string          `json:"programId"`
	OrgID           string          `json:"orgId"`
	Title           string          `json:"title"`
	Status          string          `json:"status"`
	EstimateMinutes *int            `json:"estimateMinutes"`
	AssigneeID      *string         `json:"assigneeId"`
	DueOn           *string         `json:"dueOn"`
	InternalNote    json.RawMessage `json:"internalNote,omitempty"`
	CreatedAt       string          `json:"createdAt"`
	UpdatedAt       string          `json:"updatedAt"`
}

type taskListPayload struct {
	Tasks []taskPayload `json:"tasks"`
}

// newTaskPayload renders one task for one caller, hiding the internal note from anyone outside
// the task's own organization.
func newTaskPayload(t store.Task, caller store.User) taskPayload {
	p := taskPayload{
		ID:              t.ID,
		ProgramID:       t.ProgramID,
		OrgID:           t.OrgID,
		Title:           t.Title,
		Status:          t.Status,
		EstimateMinutes: t.EstimateMinutes,
		AssigneeID:      t.AssigneeID,
		CreatedAt:       t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       t.UpdatedAt.Format(time.RFC3339),
	}
	if t.DueOn != nil {
		due := t.DueOn.Format(dateLayout)
		p.DueOn = &due
	}
	if caller.InOrg(t.OrgID) {
		p.InternalNote = json.RawMessage("null")
		if t.InternalNote != nil {
			if encoded, err := json.Marshal(*t.InternalNote); err == nil {
				p.InternalNote = encoded
			}
		}
	}
	return p
}

func newTaskListPayload(tasks []store.Task, caller store.User) taskListPayload {
	out := taskListPayload{Tasks: make([]taskPayload, 0, len(tasks))}
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, newTaskPayload(t, caller))
	}
	return out
}

// --- routes ---------------------------------------------------------------

func (s *server) listTasks(w http.ResponseWriter, r *http.Request) {
	caller := userFrom(r.Context())
	access, ok := s.programAccess(w, r)
	if !ok {
		return
	}

	status := r.URL.Query().Get("status")
	if status != "" && !store.ValidStatus(status) {
		writeValidationError(w, s.log, []fieldError{{
			Field:   "status",
			Message: "must be one of " + strings.Join(store.Statuses, ", "),
		}})
		return
	}

	tasks, err := s.store.ListTasks(r.Context(), access.ProgramID, status)
	if err != nil {
		s.fail(w, r, "list tasks", err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, newTaskListPayload(tasks, caller))
}

func (s *server) listReadyTasks(w http.ResponseWriter, r *http.Request) {
	caller := userFrom(r.Context())
	access, ok := s.programAccess(w, r)
	if !ok {
		return
	}

	tasks, err := s.store.ListReadyTasks(r.Context(), access.ProgramID)
	if err != nil {
		s.fail(w, r, "list ready tasks", err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, newTaskListPayload(tasks, caller))
}

func (s *server) getTask(w http.ResponseWriter, r *http.Request) {
	caller := userFrom(r.Context())
	task, ok := s.visibleTask(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	writeJSON(w, s.log, http.StatusOK, newTaskPayload(task, caller))
}

// createTaskRequest is the POST body. Every optional field is a pointer so "absent" and "null"
// are both distinguishable from a zero value, and dueOn stays a string until it is validated.
type createTaskRequest struct {
	Title           *string `json:"title"`
	EstimateMinutes *int    `json:"estimateMinutes"`
	AssigneeID      *string `json:"assigneeId"`
	DueOn           *string `json:"dueOn"`
	InternalNote    *string `json:"internalNote"`
}

func (s *server) createTask(w http.ResponseWriter, r *http.Request) {
	caller := userFrom(r.Context())
	access, ok := s.programAccess(w, r)
	if !ok {
		return
	}

	var req createTaskRequest
	if !decodeBody(w, s.log, r, &req) {
		return
	}

	var details []fieldError
	title := ""
	if req.Title != nil {
		title = strings.TrimSpace(*req.Title)
	}
	switch {
	case title == "":
		details = append(details, fieldError{Field: "title", Message: "is required and cannot be blank"})
	case len(title) > 200:
		details = append(details, fieldError{Field: "title", Message: "must be 200 characters or fewer"})
	}
	if req.EstimateMinutes != nil && *req.EstimateMinutes < 0 {
		details = append(details, fieldError{Field: "estimateMinutes", Message: "cannot be negative"})
	}
	var dueOn *time.Time
	if req.DueOn != nil && *req.DueOn != "" {
		parsed, err := time.Parse(dateLayout, *req.DueOn)
		if err != nil {
			details = append(details, fieldError{Field: "dueOn", Message: "must be a calendar date in YYYY-MM-DD form"})
		} else {
			dueOn = &parsed
		}
	}
	if req.AssigneeID != nil && *req.AssigneeID != "" {
		if _, err := uuid.Parse(*req.AssigneeID); err != nil {
			details = append(details, fieldError{Field: "assigneeId", Message: "must be a user id"})
		}
	}
	if len(details) > 0 {
		writeValidationError(w, s.log, details)
		return
	}

	created, err := s.store.CreateTask(r.Context(), store.NewTask{
		ProgramID:       access.ProgramID,
		OrgID:           access.CallerOrg,
		Title:           title,
		EstimateMinutes: req.EstimateMinutes,
		AssigneeID:      req.AssigneeID,
		DueOn:           dueOn,
		InternalNote:    req.InternalNote,
	})
	switch {
	case errors.Is(err, store.ErrDuplicateTitle):
		writeError(w, s.log, http.StatusConflict, codeDuplicateTitle,
			"another active task in this program already has that title")
		return
	case err != nil:
		s.fail(w, r, "create task", err)
		return
	}

	w.Header().Set("Location", "/tasks/"+created.ID)
	writeJSON(w, s.log, http.StatusCreated, newTaskPayload(created, caller))
}

type patchTaskRequest struct {
	Status *string `json:"status"`
}

func (s *server) patchTask(w http.ResponseWriter, r *http.Request) {
	caller := userFrom(r.Context())

	var req patchTaskRequest
	if !decodeBody(w, s.log, r, &req) {
		return
	}
	if req.Status == nil || !store.ValidStatus(*req.Status) {
		writeValidationError(w, s.log, []fieldError{{
			Field:   "status",
			Message: "must be one of " + strings.Join(store.Statuses, ", "),
		}})
		return
	}

	task, ok := s.visibleTask(w, r, r.PathValue("id"))
	if !ok {
		return
	}

	updated, change, err := s.store.SetTaskStatus(r.Context(), task.ID, *req.Status, caller.ID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.notFound(w)
		return
	case err != nil:
		s.fail(w, r, "set task status", err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, newTaskPayload(updated, caller))
	s.notifier.Notify(change)
}

// --- shared route plumbing -------------------------------------------------

// programAccess resolves the {programId} path value against the caller's organizations, answering
// 404 (never 403) when they may not see it, so a stranger cannot tell the program exists.
func (s *server) programAccess(w http.ResponseWriter, r *http.Request) (store.ProgramAccess, bool) {
	caller := userFrom(r.Context())

	programID := r.PathValue("programId")
	if _, err := uuid.Parse(programID); err != nil {
		s.notFound(w)
		return store.ProgramAccess{}, false
	}

	access, err := s.store.ProgramAccessFor(r.Context(), programID, caller.OrgIDs)
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.notFound(w)
		return store.ProgramAccess{}, false
	case err != nil:
		s.fail(w, r, "resolve program access", err)
		return store.ProgramAccess{}, false
	}
	return access, true
}

// visibleTask loads a task by id and confirms the caller can see the program it belongs to,
// answering 404 for an unknown, soft-deleted, or invisible task alike.
func (s *server) visibleTask(w http.ResponseWriter, r *http.Request, id string) (store.Task, bool) {
	caller := userFrom(r.Context())

	if _, err := uuid.Parse(id); err != nil {
		s.notFound(w)
		return store.Task{}, false
	}

	task, err := s.store.GetTask(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.notFound(w)
		return store.Task{}, false
	case err != nil:
		s.fail(w, r, "get task", err)
		return store.Task{}, false
	}

	if _, err := s.store.ProgramAccessFor(r.Context(), task.ProgramID, caller.OrgIDs); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.notFound(w)
		} else {
			s.fail(w, r, "resolve program access", err)
		}
		return store.Task{}, false
	}
	return task, true
}

// notFound is the one answer this API gives for "does not exist", "is deleted", and "is not
// yours", so the three stay indistinguishable from outside.
func (s *server) notFound(w http.ResponseWriter) {
	writeError(w, s.log, http.StatusNotFound, codeNotFound, "no such resource")
}

// decodeBody reads a JSON request body, answering 400 with the shared envelope when it is not
// JSON at all. It reports whether the caller should keep going.
func decodeBody(w http.ResponseWriter, log *slog.Logger, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeValidationError(w, log, []fieldError{{Field: "body", Message: "must be a JSON object"}})
		return false
	}
	return true
}

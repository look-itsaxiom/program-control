package httpapi

import (
	"net/http"

	"github.com/look-itsaxiom/program-control/api/internal/store"
)

// taskView is how much of one task a caller may see.
type taskView int

const (
	// viewHidden means the task is not theirs to know about; it is left out of a list and
	// answered 404 on its own, never acknowledged and refused.
	viewHidden taskView = iota
	// viewReduced is the id, title, and status of a task a vendor is only waiting on.
	viewReduced
	// viewFull is the whole Task shape from S2.2.
	viewFull
)

// reducedTaskPayload is what a vendor sees of a task they do not own but are waiting on: enough
// to know it exists, what it is called, and whether it is finished. Every other key is absent,
// not null, so the shape itself says nothing about what was withheld.
type reducedTaskPayload struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// visibility decides how much of each task in one program one caller sees. The owning
// organization sees everything; an organization the program was shared with sees its own tasks
// and what those wait on.
type visibility struct {
	owner  bool
	vendor store.VendorVisibility
}

func (v visibility) view(task store.Task) taskView {
	switch {
	case v.owner:
		return viewFull
	case v.vendor.Owned[task.ID]:
		return viewFull
	case v.vendor.Predecessors[task.ID]:
		return viewReduced
	default:
		return viewHidden
	}
}

// render turns one task into the JSON body the caller may have, or nil when they may have none.
func (v visibility) render(task store.Task, caller store.User) any {
	switch v.view(task) {
	case viewFull:
		return newTaskPayload(task, caller)
	case viewReduced:
		return reducedTaskPayload{ID: task.ID, Title: task.Title, Status: task.Status}
	default:
		return nil
	}
}

// renderList drops the tasks this caller may not see and reduces the ones they may only glimpse.
func (v visibility) renderList(tasks []store.Task, caller store.User) []any {
	out := make([]any, 0, len(tasks))
	for _, task := range tasks {
		if body := v.render(task, caller); body != nil {
			out = append(out, body)
		}
	}
	return out
}

// visibilityFor resolves how much of a program the caller sees. Resolving it once per request
// keeps the rule in one place instead of spread across every route that returns a task.
func (s *server) visibilityFor(w http.ResponseWriter, r *http.Request, access store.ProgramAccess) (visibility, bool) {
	if access.IsOwner() {
		return visibility{owner: true}, true
	}
	vendor, err := s.store.VendorVisibilityFor(r.Context(), access.ProgramID, access.CallerOrg)
	if err != nil {
		s.fail(w, r, "resolve vendor visibility", err)
		return visibility{}, false
	}
	return visibility{vendor: vendor}, true
}

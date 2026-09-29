package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/look-itsaxiom/program-control/api/internal/store"
)

type addDependencyRequest struct {
	PredecessorID *string `json:"predecessorId"`
}

type dependencyPayload struct {
	PredecessorID string `json:"predecessorId"`
	SuccessorID   string `json:"successorId"`
}

// addDependency records that the task named in the body must finish before the task in the path.
func (s *server) addDependency(w http.ResponseWriter, r *http.Request) {
	successor, _, ok := s.visibleTask(w, r, r.PathValue("id"))
	if !ok {
		return
	}

	var req addDependencyRequest
	if !decodeBody(w, s.log, r, &req) {
		return
	}
	if req.PredecessorID == nil || *req.PredecessorID == "" {
		writeValidationError(w, s.log, []fieldError{{
			Field:   "predecessorId",
			Message: "is required and names the task that must finish first",
		}})
		return
	}

	predecessor, _, ok := s.visibleTask(w, r, *req.PredecessorID)
	if !ok {
		return
	}

	err := s.store.AddDependency(r.Context(), predecessor.ID, successor.ID)
	var cycle *store.CycleError
	switch {
	case errors.As(err, &cycle):
		writeError(w, s.log, http.StatusConflict, codeCycle,
			"this dependency would create a cycle through "+strings.Join(cycle.Path, " -> "))
		return
	case errors.Is(err, store.ErrDuplicate):
		writeError(w, s.log, http.StatusConflict, codeDuplicate,
			"that dependency is already recorded")
		return
	case err != nil:
		s.fail(w, r, "add dependency", err)
		return
	}

	writeJSON(w, s.log, http.StatusCreated, dependencyPayload{
		PredecessorID: predecessor.ID,
		SuccessorID:   successor.ID,
	})
}

// removeDependency drops one edge. It answers 204 whether or not the edge was there, so a client
// retrying a delete does not have to treat the second attempt as a failure.
func (s *server) removeDependency(w http.ResponseWriter, r *http.Request) {
	successor, _, ok := s.visibleTask(w, r, r.PathValue("id"))
	if !ok {
		return
	}

	predecessorID := r.PathValue("predecessorId")
	if _, err := uuid.Parse(predecessorID); err != nil {
		s.notFound(w)
		return
	}

	if err := s.store.RemoveDependency(r.Context(), predecessorID, successor.ID); err != nil {
		s.fail(w, r, "remove dependency", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

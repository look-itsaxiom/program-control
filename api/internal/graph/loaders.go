package graph

import (
	"context"
	"sync"

	"github.com/look-itsaxiom/program-control/api/internal/store"
)

// Loaders batch and memoize the queries a nested selection would otherwise repeat once per task.
// One set is built per request and thrown away with it, so nothing is ever cached across
// requests, across callers, or past a write.
//
// The batching key is the program rather than the individual task: every predecessor of a task is
// a task in the same program, so one query answers the field for every task in the selection at
// once. That makes the batch deterministic, with no timing window to tune and no chance of a slow
// goroutine splitting one batch into two.
type Loaders struct {
	store *store.Store

	mu           sync.Mutex
	predecessors map[string]*predecessorBatch
	programs     map[string]*programBatch
}

// NewLoaders returns a loader set for a single request.
func NewLoaders(st *store.Store) *Loaders {
	return &Loaders{
		store:        st,
		predecessors: map[string]*predecessorBatch{},
		programs:     map[string]*programBatch{},
	}
}

type predecessorBatch struct {
	once sync.Once
	byID map[string][]store.TaskWithAssignee
	err  error
}

// Predecessors returns the tasks taskID depends on, loading every task in the program's
// predecessor edges the first time any task in that program asks.
func (l *Loaders) Predecessors(ctx context.Context, programID, taskID string) ([]store.TaskWithAssignee, error) {
	l.mu.Lock()
	batch, ok := l.predecessors[programID]
	if !ok {
		batch = &predecessorBatch{}
		l.predecessors[programID] = batch
	}
	l.mu.Unlock()

	batch.once.Do(func() {
		batch.byID, batch.err = l.store.PredecessorsByProgram(ctx, programID)
	})
	if batch.err != nil {
		return nil, batch.err
	}
	return batch.byID[taskID], nil
}

type programBatch struct {
	once   sync.Once
	access store.ProgramAccess
	err    error
}

// Program resolves one program's access for the caller, at most once per request however many
// tasks point back at it.
func (l *Loaders) Program(ctx context.Context, programID string, orgIDs []string) (store.ProgramAccess, error) {
	l.mu.Lock()
	batch, ok := l.programs[programID]
	if !ok {
		batch = &programBatch{}
		l.programs[programID] = batch
	}
	l.mu.Unlock()

	batch.once.Do(func() {
		batch.access, batch.err = l.store.ProgramAccessFor(ctx, programID, orgIDs)
	})
	return batch.access, batch.err
}

package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Dependency is one edge: predecessor must be done before successor can start.
type Dependency struct {
	PredecessorID string
	SuccessorID   string
}

// AddDependency records that predecessorID must finish before successorID starts.
//
// It refuses an edge that would close a cycle with CycleError, naming the path, and one that
// already exists with ErrDuplicate. Both tasks must already be known to exist and be visible to
// the caller; that check belongs to the layer above, which owns the tenancy rules.
func (s *Store) AddDependency(ctx context.Context, predecessorID, successorID string) error {
	// The new edge runs predecessor -> successor, so it closes a cycle exactly when the graph
	// already has a path running the other way, from the successor back to the predecessor.
	path, err := s.pathBetween(ctx, successorID, predecessorID)
	if err != nil {
		return err
	}
	if len(path) > 0 {
		return &CycleError{Path: append(path, successorID)}
	}

	_, err = s.pool.Exec(ctx, `
		insert into task_dependencies (predecessor_id, successor_id) values ($1, $2)
	`, predecessorID, successorID)
	if err != nil {
		if isUniqueViolation(err, "") {
			return ErrDuplicate
		}
		return fmt.Errorf("add dependency: %w", err)
	}
	return nil
}

// RemoveDependency deletes the edge if it is there and reports no error if it never was, so the
// route above it is idempotent.
func (s *Store) RemoveDependency(ctx context.Context, predecessorID, successorID string) error {
	_, err := s.pool.Exec(ctx, `
		delete from task_dependencies where predecessor_id = $1 and successor_id = $2
	`, predecessorID, successorID)
	if err != nil {
		return fmt.Errorf("remove dependency: %w", err)
	}
	return nil
}

// pathBetween walks the dependency graph forward from start and returns the first path it finds
// to target, start and target included, or nil when target is unreachable. Excluding nodes
// already on the path keeps the walk terminating even if a cycle somehow got recorded.
func (s *Store) pathBetween(ctx context.Context, start, target string) ([]string, error) {
	const q = `
		with recursive walk (node, path) as (
			select $1::uuid, array[$1::uuid]
		  union all
			select d.successor_id, w.path || d.successor_id
			from walk w
			join task_dependencies d on d.predecessor_id = w.node
			where not d.successor_id = any(w.path)
		)
		select path from walk where node = $2::uuid limit 1
	`
	var path []string
	err := s.pool.QueryRow(ctx, q, start, target).Scan(&path)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("search dependency graph: %w", err)
	}
	return path, nil
}

// DependenciesFor returns the predecessor edges of every task in successorIDs, in one query.
// Phase 4's dataloader batches into it.
func (s *Store) DependenciesFor(ctx context.Context, successorIDs []string) ([]Dependency, error) {
	rows, err := s.pool.Query(ctx, `
		select d.predecessor_id, d.successor_id
		from task_dependencies d
		where d.successor_id = any($1::uuid[])
		order by d.successor_id, d.predecessor_id
	`, successorIDs)
	if err != nil {
		return nil, fmt.Errorf("dependencies for tasks: %w", err)
	}
	defer rows.Close()

	deps := []Dependency{}
	for rows.Next() {
		var d Dependency
		if err := rows.Scan(&d.PredecessorID, &d.SuccessorID); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		deps = append(deps, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read dependencies: %w", err)
	}
	return deps, nil
}

package store

import (
	"context"
	"fmt"
	"time"
)

// VendorVisibility is what one shared organization may see inside a program: the tasks it owns,
// and the tasks those wait on. Everything else in the program is not merely redacted for them,
// it is not theirs to know about.
type VendorVisibility struct {
	Owned        map[string]bool
	Predecessors map[string]bool
}

// Sees reports whether taskID is visible at all.
func (v VendorVisibility) Sees(taskID string) bool {
	return v.Owned[taskID] || v.Predecessors[taskID]
}

// VisibleIDs returns every task id this organization may see, in no particular order.
func (v VendorVisibility) VisibleIDs() []string {
	ids := make([]string, 0, len(v.Owned)+len(v.Predecessors))
	for id := range v.Owned {
		ids = append(ids, id)
	}
	for id := range v.Predecessors {
		ids = append(ids, id)
	}
	return ids
}

// VendorVisibilityFor walks the dependency graph backwards from the tasks orgID owns in this
// program, so a vendor learns why their own work is waiting without learning the rest of the
// program. UNION rather than UNION ALL in the recursive term is what keeps the walk terminating
// if the graph ever holds a cycle.
func (s *Store) VendorVisibilityFor(ctx context.Context, programID, orgID string) (VendorVisibility, error) {
	const q = `
		with recursive owned as (
			select t.id
			from tasks t
			where t.program_id = $1 and t.org_id = $2 and t.deleted_at is null
		),
		ancestors (id) as (
			select d.predecessor_id from task_dependencies d join owned o on o.id = d.successor_id
		  union
			select d.predecessor_id from task_dependencies d join ancestors a on a.id = d.successor_id
		)
		select o.id, true from owned o
		union all
		select t.id, false
		from ancestors a
		join tasks t on t.id = a.id and t.deleted_at is null
		where a.id not in (select id from owned)
	`

	rows, err := s.pool.Query(ctx, q, programID, orgID)
	if err != nil {
		return VendorVisibility{}, fmt.Errorf("vendor visibility: %w", err)
	}
	defer rows.Close()

	vis := VendorVisibility{Owned: map[string]bool{}, Predecessors: map[string]bool{}}
	for rows.Next() {
		var id string
		var owned bool
		if err := rows.Scan(&id, &owned); err != nil {
			return VendorVisibility{}, fmt.Errorf("scan vendor visibility: %w", err)
		}
		if owned {
			vis.Owned[id] = true
		} else {
			vis.Predecessors[id] = true
		}
	}
	if err := rows.Err(); err != nil {
		return VendorVisibility{}, fmt.Errorf("read vendor visibility: %w", err)
	}
	return vis, nil
}

// Event is one row of the audit trail.
type Event struct {
	ID         int64
	TaskID     string
	Kind       string
	FromStatus *string
	ToStatus   *string
	ActorID    *string
	CreatedAt  time.Time
}

// ListEvents returns the program's audit trail, ascending by id, starting after since (0 means
// from the beginning) and limited to taskIDs. A nil taskIDs means every task in the program.
func (s *Store) ListEvents(ctx context.Context, programID string, since int64, taskIDs []string) ([]Event, error) {
	const q = `
		select e.id, e.task_id, e.kind, e.from_status, e.to_status, e.actor_id, e.created_at
		from task_events e
		join tasks t on t.id = e.task_id
		where t.program_id = $1
		  and e.id > $2
		  and ($3::uuid[] is null or e.task_id = any($3::uuid[]))
		order by e.id asc
	`

	rows, err := s.pool.Query(ctx, q, programID, since, taskIDs)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.TaskID, &e.Kind, &e.FromStatus, &e.ToStatus, &e.ActorID, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	return events, nil
}

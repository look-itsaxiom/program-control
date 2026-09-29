package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Task is one row of the tasks table. InternalNote is carried here whether or not the caller may
// see it; deciding that is the job of the layer that serializes a task, not the layer that reads
// it.
type Task struct {
	ID              string
	ProgramID       string
	OrgID           string
	Title           string
	Status          string
	EstimateMinutes *int
	AssigneeID      *string
	DueOn           *time.Time
	InternalNote    *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Statuses is the set of task statuses the schema accepts, in workflow order.
var Statuses = []string{"todo", "doing", "blocked", "done"}

// ValidStatus reports whether s is one of Statuses.
func ValidStatus(s string) bool {
	for _, known := range Statuses {
		if s == known {
			return true
		}
	}
	return false
}

// taskColumns is the select list every task query shares, aliased to t.
const taskColumns = `t.id, t.program_id, t.org_id, t.title, t.status, t.estimate_minutes,
	t.assignee_id, t.due_on, t.internal_note, t.created_at, t.updated_at`

// boardOrder is the ordering S2.2 fixes: by due date, undated tasks last, oldest first on a tie.
const boardOrder = `order by t.due_on asc nulls last, t.created_at asc`

func scanTask(row pgx.Row) (Task, error) {
	var t Task
	err := row.Scan(&t.ID, &t.ProgramID, &t.OrgID, &t.Title, &t.Status, &t.EstimateMinutes,
		&t.AssigneeID, &t.DueOn, &t.InternalNote, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func collectTasks(rows pgx.Rows) ([]Task, error) {
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tasks: %w", err)
	}
	return tasks, nil
}

// ListTasks returns a program's active tasks in board order, narrowed to one status when status
// is non-empty.
func (s *Store) ListTasks(ctx context.Context, programID, status string) ([]Task, error) {
	q := `select ` + taskColumns + `
		from tasks t
		where t.program_id = $1 and t.deleted_at is null
		  and ($2 = '' or t.status = $2)
		` + boardOrder

	rows, err := s.pool.Query(ctx, q, programID, status)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return collectTasks(rows)
}

// ListReadyTasks returns the program's tasks that are still todo and whose every active
// predecessor is done. A task with no predecessors is ready; a blocked task never is, because a
// human has already said it is not workable.
func (s *Store) ListReadyTasks(ctx context.Context, programID string) ([]Task, error) {
	q := `select ` + taskColumns + `
		from tasks t
		where t.program_id = $1 and t.deleted_at is null and t.status = 'todo'
		  and not exists (
			select 1
			from task_dependencies d
			join tasks p on p.id = d.predecessor_id
			where d.successor_id = t.id and p.deleted_at is null and p.status <> 'done'
		  )
		` + boardOrder

	rows, err := s.pool.Query(ctx, q, programID)
	if err != nil {
		return nil, fmt.Errorf("list ready tasks: %w", err)
	}
	return collectTasks(rows)
}

// GetTask loads one active task, returning ErrNotFound when it does not exist or is soft-deleted.
func (s *Store) GetTask(ctx context.Context, id string) (Task, error) {
	q := `select ` + taskColumns + ` from tasks t where t.id = $1 and t.deleted_at is null`
	t, err := scanTask(s.pool.QueryRow(ctx, q, id))
	if err != nil {
		return Task{}, notFound(err, "get task")
	}
	return t, nil
}

// TasksByIDs loads several active tasks in one query, in board order. It is what phase 4's
// dataloaders batch into.
func (s *Store) TasksByIDs(ctx context.Context, ids []string) ([]Task, error) {
	q := `select ` + taskColumns + `
		from tasks t where t.id = any($1::uuid[]) and t.deleted_at is null ` + boardOrder

	rows, err := s.pool.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("tasks by ids: %w", err)
	}
	return collectTasks(rows)
}

// NewTask is the set of fields CreateTask accepts. Everything but Title, ProgramID, and OrgID is
// optional.
type NewTask struct {
	ProgramID       string
	OrgID           string
	Title           string
	EstimateMinutes *int
	AssigneeID      *string
	DueOn           *time.Time
	InternalNote    *string
}

// CreateTask inserts a task, returning ErrDuplicateTitle when another active task in the same
// program already carries this title. The database decides that, not a read-then-write here, so
// two concurrent creates cannot both pass a check and then both insert.
func (s *Store) CreateTask(ctx context.Context, in NewTask) (Task, error) {
	q := `
		insert into tasks as t
			(program_id, org_id, title, estimate_minutes, assignee_id, due_on, internal_note)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning ` + taskColumns

	t, err := scanTask(s.pool.QueryRow(ctx, q, in.ProgramID, in.OrgID, in.Title,
		in.EstimateMinutes, in.AssigneeID, in.DueOn, in.InternalNote))
	if err != nil {
		if isUniqueViolation(err, "tasks_active_title_per_program_idx") {
			return Task{}, ErrDuplicateTitle
		}
		return Task{}, fmt.Errorf("create task: %w", err)
	}
	return t, nil
}

// StatusChange is what one status transition amounted to: everything a downstream system needs
// to be told about it, and nothing about who asked.
type StatusChange struct {
	TaskID     string
	FromStatus string
	ToStatus   string
	Kind       string
	OccurredAt time.Time
}

// SetTaskStatus moves a task to status on behalf of a person, appending a "status_changed" event.
func (s *Store) SetTaskStatus(ctx context.Context, id, status, actorID string) (Task, StatusChange, error) {
	return s.ChangeTaskStatus(ctx, id, status, "status_changed", &actorID)
}

// ChangeTaskStatus moves a task to status and appends the matching audit event, both or neither.
// kind names what drove the change and actorID names the person behind it, which is nil when
// nobody was: a webhook is a system talking to a system.
func (s *Store) ChangeTaskStatus(ctx context.Context, id, status, kind string, actorID *string) (Task, StatusChange, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Task{}, StatusChange{}, fmt.Errorf("begin status change: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	updated, change, err := changeStatusTx(ctx, tx, id, status, kind, actorID)
	if err != nil {
		return Task{}, StatusChange{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, StatusChange{}, fmt.Errorf("commit status change: %w", err)
	}
	return updated, change, nil
}

// changeStatusTx is the body of a status change, inside whatever transaction the caller opened,
// so a webhook can record its message id and the change it caused as one atomic step.
func changeStatusTx(ctx context.Context, tx pgx.Tx, id, status, kind string, actorID *string) (Task, StatusChange, error) {
	var from string
	err := tx.QueryRow(ctx,
		`select status from tasks where id = $1 and deleted_at is null for update`, id).Scan(&from)
	if err != nil {
		return Task{}, StatusChange{}, notFound(err, "lock task for status change")
	}

	q := `update tasks t set status = $2 where t.id = $1 returning ` + taskColumns
	updated, err := scanTask(tx.QueryRow(ctx, q, id, status))
	if err != nil {
		return Task{}, StatusChange{}, fmt.Errorf("update task status: %w", err)
	}

	var occurredAt time.Time
	err = tx.QueryRow(ctx, `
		insert into task_events (task_id, actor_id, kind, from_status, to_status)
		values ($1, $2, $3, $4, $5)
		returning created_at
	`, id, actorID, kind, from, status).Scan(&occurredAt)
	if err != nil {
		return Task{}, StatusChange{}, fmt.Errorf("append %s event: %w", kind, err)
	}

	return updated, StatusChange{
		TaskID:     id,
		FromStatus: from,
		ToStatus:   status,
		Kind:       kind,
		OccurredAt: occurredAt,
	}, nil
}

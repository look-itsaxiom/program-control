package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TaskWithAssignee is a task together with the person it is assigned to, if anyone. An assignee
// is a to-one relation, so every query that returns tasks joins it in rather than leaving a
// second round trip for the layer above; a to-many relation like predecessors is batched instead.
type TaskWithAssignee struct {
	Task
	Assignee *User
}

// taskWithAssigneeColumns extends taskColumns with the joined user, aliased u.
const taskWithAssigneeColumns = taskColumns + `, u.id, u.display_name, u.email`

func scanTaskWithAssignee(row pgx.Row) (TaskWithAssignee, error) {
	var t TaskWithAssignee
	var assigneeID, displayName, email *string
	err := row.Scan(&t.ID, &t.ProgramID, &t.OrgID, &t.Title, &t.Status, &t.EstimateMinutes,
		&t.AssigneeID, &t.DueOn, &t.InternalNote, &t.CreatedAt, &t.UpdatedAt,
		&assigneeID, &displayName, &email)
	if err != nil {
		return TaskWithAssignee{}, err
	}
	if assigneeID != nil {
		t.Assignee = &User{ID: *assigneeID, Name: derefString(displayName), Email: derefString(email)}
	}
	return t, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func collectTasksWithAssignee(rows pgx.Rows) ([]TaskWithAssignee, error) {
	defer rows.Close()
	tasks := []TaskWithAssignee{}
	for rows.Next() {
		t, err := scanTaskWithAssignee(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task with assignee: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tasks with assignee: %w", err)
	}
	return tasks, nil
}

// ListProgramTasks is ListTasks with the assignee joined in, for the graph.
func (s *Store) ListProgramTasks(ctx context.Context, programID, status string) ([]TaskWithAssignee, error) {
	q := `select ` + taskWithAssigneeColumns + `
		from tasks t
		left join users u on u.id = t.assignee_id
		where t.program_id = $1 and t.deleted_at is null
		  and ($2 = '' or t.status = $2)
		` + boardOrder

	rows, err := s.pool.Query(ctx, q, programID, status)
	if err != nil {
		return nil, fmt.Errorf("list program tasks: %w", err)
	}
	return collectTasksWithAssignee(rows)
}

// ListProgramReadyTasks is ListReadyTasks with the assignee joined in, for the graph.
func (s *Store) ListProgramReadyTasks(ctx context.Context, programID string) ([]TaskWithAssignee, error) {
	q := `select ` + taskWithAssigneeColumns + `
		from tasks t
		left join users u on u.id = t.assignee_id
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
		return nil, fmt.Errorf("list program ready tasks: %w", err)
	}
	return collectTasksWithAssignee(rows)
}

// GetProgramTask loads one task with its assignee.
func (s *Store) GetProgramTask(ctx context.Context, id string) (TaskWithAssignee, error) {
	q := `select ` + taskWithAssigneeColumns + `
		from tasks t
		left join users u on u.id = t.assignee_id
		where t.id = $1 and t.deleted_at is null`

	t, err := scanTaskWithAssignee(s.pool.QueryRow(ctx, q, id))
	if err != nil {
		return TaskWithAssignee{}, notFound(err, "get program task")
	}
	return t, nil
}

// PredecessorsByProgram returns, for every active task in the program, the tasks it depends on,
// in one statement. It is the batch behind the graph's predecessors loader: resolving that field
// on a hundred tasks costs one query, not a hundred.
func (s *Store) PredecessorsByProgram(ctx context.Context, programID string) (map[string][]TaskWithAssignee, error) {
	q := `select d.successor_id, ` + taskWithAssigneeColumns + `
		from task_dependencies d
		join tasks s on s.id = d.successor_id and s.program_id = $1 and s.deleted_at is null
		join tasks t on t.id = d.predecessor_id and t.deleted_at is null
		left join users u on u.id = t.assignee_id
		` + boardOrder

	rows, err := s.pool.Query(ctx, q, programID)
	if err != nil {
		return nil, fmt.Errorf("predecessors by program: %w", err)
	}
	defer rows.Close()

	bySuccessor := map[string][]TaskWithAssignee{}
	for rows.Next() {
		var successorID string
		var t TaskWithAssignee
		var assigneeID, displayName, email *string
		err := rows.Scan(&successorID, &t.ID, &t.ProgramID, &t.OrgID, &t.Title, &t.Status,
			&t.EstimateMinutes, &t.AssigneeID, &t.DueOn, &t.InternalNote, &t.CreatedAt,
			&t.UpdatedAt, &assigneeID, &displayName, &email)
		if err != nil {
			return nil, fmt.Errorf("scan predecessor: %w", err)
		}
		if assigneeID != nil {
			t.Assignee = &User{ID: *assigneeID, Name: derefString(displayName), Email: derefString(email)}
		}
		bySuccessor[successorID] = append(bySuccessor[successorID], t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read predecessors: %w", err)
	}
	return bySuccessor, nil
}

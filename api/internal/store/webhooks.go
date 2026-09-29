package store

import (
	"context"
	"fmt"
)

// ApplyWebhookStatus records an inbound webhook message and the status change it asks for, as one
// transaction. Recording the message id and applying the change together is what makes the route
// idempotent: a redelivery loses the race for the primary key and changes nothing.
//
// It reports whether the message was new. A repeat returns applied=false and no change, not an
// error: the sender did nothing wrong.
func (s *Store) ApplyWebhookStatus(ctx context.Context, messageID, taskID, status string) (applied bool, task Task, change StatusChange, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, Task{}, StatusChange{}, fmt.Errorf("begin webhook: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		insert into webhook_messages (id) values ($1) on conflict (id) do nothing
	`, messageID)
	if err != nil {
		return false, Task{}, StatusChange{}, fmt.Errorf("record webhook message: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, Task{}, StatusChange{}, nil
	}

	task, change, err = changeStatusTx(ctx, tx, taskID, status, "webhook", nil)
	if err != nil {
		return false, Task{}, StatusChange{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, Task{}, StatusChange{}, fmt.Errorf("commit webhook: %w", err)
	}
	return true, task, change, nil
}

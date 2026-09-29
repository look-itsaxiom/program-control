// Package model holds the types the GraphQL layer resolves into. The enums and input types are
// generated into models_gen.go; the structs here are hand-written and bound in gqlgen.yml, so a
// field a resolver should fetch lazily (predecessors, tasks) is simply left off the struct and
// gqlgen generates a resolver for it.
package model

import (
	"strings"

	"github.com/look-itsaxiom/program-control/api/internal/store"
)

// User is one person. Every field is plain data, so gqlgen reads it straight off the struct.
type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

// Program deliberately has no tasks or readyTasks field: those are resolved on demand, so asking
// for a program's name never costs a query for its board.
type Program struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Task carries its assignee inline because an assignee is a to-one relation the task query can
// join in for free. Predecessors and program are left off, so gqlgen resolves them on demand
// through the per-request loaders.
type Task struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Status          TaskStatus `json:"status"`
	EstimateMinutes *int       `json:"estimateMinutes,omitempty"`
	DueOn           *string    `json:"dueOn,omitempty"`
	InternalNote    *string    `json:"internalNote,omitempty"`
	Assignee        *User      `json:"assignee,omitempty"`

	// ProgramID is not part of the schema; the program field resolver uses it.
	ProgramID string `json:"-"`
}

// NewUser converts a store user into its graph shape.
func NewUser(u store.User) *User {
	return &User{ID: u.ID, DisplayName: u.Name, Email: u.Email}
}

// NewTask converts a store task into its graph shape for one caller. The internal note is nulled
// out, rather than omitted, for a caller outside the task's owning organization: that is how
// GraphQL says "you may ask for this field, but not see its value here".
func NewTask(t store.TaskWithAssignee, caller store.User) *Task {
	out := &Task{
		ID:              t.ID,
		Title:           t.Title,
		Status:          TaskStatus(strings.ToUpper(t.Status)),
		EstimateMinutes: t.EstimateMinutes,
		ProgramID:       t.ProgramID,
	}
	if t.DueOn != nil {
		due := t.DueOn.Format("2006-01-02")
		out.DueOn = &due
	}
	if caller.InOrg(t.OrgID) {
		out.InternalNote = t.InternalNote
	}
	if t.Assignee != nil {
		out.Assignee = NewUser(*t.Assignee)
	}
	return out
}

// NewTasks converts a slice of store tasks for one caller.
func NewTasks(tasks []store.TaskWithAssignee, caller store.User) []*Task {
	out := make([]*Task, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, NewTask(t, caller))
	}
	return out
}

// StoreStatus turns a schema enum value into the lowercase status the database stores.
func (s TaskStatus) StoreStatus() string { return strings.ToLower(string(s)) }

# Phase 1: schema

Read the [primer](./PRIMER.md) first. Every story in this phase is delivered as one or more
migration files (see "Migrations," at the end, for where they live and how they are applied and
checked).

## S1.1: organizations, users, and memberships

As the system, I need to know which organizations and people exist and how people belong to
organizations, so that everything else in the domain (who owns a program, who can see it, who is
assigned to a task) has something to point at.

**Acceptance:** a customer organization and two vendor organizations can each be recorded once,
by name, and recording a second organization with a name already in use is refused. A person can
be recorded once, by email, and a second person with the same email is refused. A person's
membership in an organization records a role, and only a fixed, known set of roles is accepted; a
membership with a role outside that set is refused. A given person can belong to more than one
organization, and a given organization has more than one person, but a person cannot be recorded
as a member of the same organization twice.

**Contract:**

- `organizations` has an `id` (a UUID, and the table's primary key), a `name` (text, required,
  and unique across every organization in the system), and a `created_at` timestamp with time
  zone (required).
- `users` has an `id` (a UUID, primary key), an `email` (text, required, unique across every
  user), and a `display_name` (text, required).
- `memberships` links one `users.id` to one `org_id` referencing `organizations.id`, together with
  a `role` (text, required, one of exactly `owner`, `admin`, `member`, or `viewer`). The pair of
  user and organization is the table's primary key, so the same person cannot hold two rows for
  the same organization.

## S1.2: programs, owned and shared

As the system, I need to know which organization owns a program and which other organizations it
has been explicitly shared with, so that a vendor can see the program their customer invited them
into without owning it.

**Acceptance:** a program has exactly one owning organization and a name; two active programs
owned by the same organization cannot share a name, but two different organizations may each have
a program with the same name, and a name freed up by soft-deleting a program can be reused by a
new one in the same organization. A program can be shared with another organization at a role of
either editor or viewer; sharing at any other role is refused. Deleting a program removes its
shares along with it, and (see S1.3) the tasks that belonged to it. An organization that still
owns at least one program cannot itself be deleted; the attempt is refused, not silently ignored.
An organization that owns no programs can be deleted even if it is still on the sharing side of
someone else's program, and deleting it removes those share rows.

**Contract:**

- `programs` has an `id` (UUID, primary key), an `org_id` referencing `organizations.id` (the
  owning organization; an organization cannot be deleted while it still owns a program), a `name`
  (text, required), `created_at` and `updated_at` timestamps with time zone (both required; see
  S1.7 for how `updated_at` behaves), and a `deleted_at` timestamp with time zone that is empty
  unless the program has been soft-deleted. A program's name is unique among the still-active
  programs owned by the same organization, not across the whole table.
- `program_shares` links one `program_id` referencing `programs.id` to one `org_id` referencing
  `organizations.id`, together with a `role` (text, required, one of exactly `editor` or
  `viewer`). The pair of program and organization is the table's primary key. Deleting a program
  removes its rows in this table; deleting an organization removes its rows in this table too
  (an organization can be deleted once it owns no programs, even if it is still on the sharing
  side of other programs).

## S1.3: tasks

As the system, I need to track each task in a program: its title, its status, who it is assigned
to, its time estimate, its due date, and a note visible only inside the owning organization, so
that the rest of the product has something to build a board out of.

**Acceptance:** a task belongs to exactly one program and records which organization owns it. It
has a title between 1 and 200 characters; an empty title, or one over 200 characters, is refused.
Its status is one of exactly `todo`, `doing`, `blocked`, or `done`; any other value is refused,
and a task recorded without stating a status starts as `todo`. Its estimate, when given, is
expressed in minutes and is never negative; a negative estimate is refused. Its assignee, when
given, must be a real person; if that person is later removed from the system, the task is not
deleted or blocked, its assignee simply becomes unset. Its due date and its internal note are
both optional. A task can be soft-deleted rather than removed outright (see S1.6 for how that
interacts with title uniqueness).

**Contract:**

- `tasks` has an `id` (UUID, primary key), a `program_id` referencing `programs.id` (deleting a
  program removes its tasks), an `org_id` referencing `organizations.id` (the organization that
  owns the task), a `title` (text, required, between 1 and 200 characters), a `status` (text,
  required, one of exactly `todo`, `doing`, `blocked`, `done`, defaulting to `todo` when not
  given), an `estimate_minutes` (integer, optional, never negative when present), an
  `assignee_id` referencing `users.id` (optional; if the referenced person is removed, this
  becomes empty rather than blocking the removal), a `due_on` date (optional), an `internal_note`
  (text, optional), `created_at` and `updated_at` timestamps with time zone (both required; see
  S1.7), and a `deleted_at` timestamp with time zone that is empty unless the task has been
  soft-deleted.

## S1.4: dependencies between tasks

As the system, I need to record that one task must finish before another can start, so that later
phases can compute which tasks are ready to begin.

**Acceptance:** a dependency records a predecessor task and a successor task. A task can never be
recorded as depending on itself; that attempt is refused. The same predecessor/successor pair
cannot be recorded twice. If either task in a dependency is deleted, the dependency goes with it.

**Contract:**

- `task_dependencies` links a `predecessor_id` referencing `tasks.id` to a `successor_id`
  referencing `tasks.id`. The pair is the table's primary key. A row where the predecessor and
  successor are the same task is refused. Deleting either the predecessor task or the successor
  task removes the dependency row.

## S1.5: audit events

As the system, I need a durable, ordered record of what happened to each task, so that later
phases can show a history and vendors and customers can trust what they are shown.

**Acceptance:** an event can be recorded against a task without the caller having to invent an
identifier for it; each new event for the system as a whole is assigned one automatically, and
later events get later identifiers than earlier ones. An event can optionally record who
performed it, what kind of thing happened, and, for a status change, the status it moved from and
the status it moved to. If the task the event is about is deleted, its events are deleted with
it.

**Contract:**

- `task_events` has an `id` (a 64-bit integer, primary key, assigned automatically and increasing
  with each new event, never supplied by the caller), a `task_id` referencing `tasks.id` (deleting
  the task removes its events), an `actor_id` referencing `users.id` (optional), a `kind` (text,
  required), a `from_status` (text, optional), a `to_status` (text, optional), and a `created_at`
  timestamp with time zone (required).

## S1.6: the database refuses bad data

As the system, I need certain kinds of bad data to be impossible to record at all, not merely
discouraged, so that no application bug can ever put the database into a state the rest of the
product does not know how to handle.

**Acceptance:**

- Two active (not soft-deleted) tasks in the same program cannot share a title. Once a task is
  soft-deleted, its title becomes available again for a new task in the same program; the
  uniqueness rule applies only among tasks that are still active.
- A membership, a program share, or a task with a role or status outside the fixed set that story
  states for it (S1.1, S1.2, S1.3) is refused outright, not stored and flagged.
- A task's estimate, when given, is never negative.
- A task can never depend on itself (restated from S1.4, because it belongs to this story's theme
  as much as that one).

**Contract:** this story adds no new tables or columns; it names the invariants that S1.1 through
S1.4's contracts already impose, and its tests exercise the same tables listed there. The "active
title uniqueness" rule is: within one `program_id`, no two rows in `tasks` with an empty
`deleted_at` may have the same `title`.

## S1.7: `updated_at` maintains itself

As a caller of the database, I need `updated_at` on a program or a task to reflect the moment the
row last changed without having to remember to set it myself on every update, so that I cannot
forget to and silently produce a wrong audit trail.

**Acceptance:** when a program or a task row is changed, its `updated_at` reflects that change
automatically, even if the statement that changed it did not mention that column at all.

**Contract:** `programs.updated_at` and `tasks.updated_at` (both already named in S1.2 and S1.3)
update themselves on every change to their row. Nothing about how that happens is part of the
contract; only that it happens.

## S1.8: the board query is indexed

As a caller listing a program's board, I need to ask for a program's tasks filtered to one status
and ordered by due date without the database reading every task row in the system to answer, so
that the board stays fast as the number of tasks grows.

**Acceptance:** listing a program's tasks with a given status, excluding soft-deleted tasks,
ordered by due date, is answered without scanning every row of the `tasks` table, and this
continues to hold once the table holds thousands of rows across many programs.

**Contract:** the tables and columns this depends on (`tasks.program_id`, `tasks.status`,
`tasks.deleted_at`, `tasks.due_on`) are already named in S1.3. No particular indexing technique
is dictated; the tests check the outcome (the database does not read the whole table to answer
this shape of query), not the mechanism.

## Migrations

**Contract:** every table and rule above is delivered as goose migration files under
`api/migrations`, numbered starting at `0002` (`0001` is the provided bootstrap migration; do not
renumber or edit it). They are applied with `pnpm db:migrate`, checked with `pnpm db:status`, and
must apply cleanly to a completely empty database and be re-runnable from scratch with
`pnpm db:reset`. Do not call the underlying migration tool directly; always go through these
`pnpm` scripts.

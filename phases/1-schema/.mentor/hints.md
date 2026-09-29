# Hints: phase 1

Hidden from the learner. Read only by the mentor. Give one rung at a time, only when asked, and
only after the learner has tried the step themselves. Rung 3 describes the shape of a solution in
prose; it never contains code, and it should still leave the learner writing the SQL.

## S1.1: organizations, users, and memberships

1. What makes a `memberships` row unique? Is it its own identity column, or is it the combination
   of who and where?
2. Course lesson 106, section "02-write-the-schema," shows a version of this exact shape
   (organizations, users, and a memberships table with a role). Read it for the general pattern,
   not to copy column-for-column; this project's contract differs from it in a few places (see
   S1.3 and this phase's primer for why).
3. Each of `organizations.name` and `users.email` gets its own uniqueness constraint. Restricting
   `memberships.role` to a fixed set is a check constraint listing the four allowed values in
   quotes. The table's primary key is the pair of foreign keys (user, organization) together,
   with no separate identity column, which is what makes "the same person twice in the same org"
   physically impossible to insert rather than merely rejected by application logic.

## S1.2: programs, owned and shared

1. What's different about "no two rows share a name" versus "no two *active* rows share a name"?
   What would a plain uniqueness constraint on `(org_id, name)` do to a program whose name you
   tried to reuse after soft-deleting the original?
2. Course lesson 106, section "04-sharing-and-visibility," covers the general shape of an
   ownership relationship plus a separate sharing relationship. For the organization-can't-be-
   deleted-while-it-owns-programs rule, section "03-constraints-are-the-api" covers how a foreign
   key's delete behavior is chosen per relationship, not defaulted once.
3. `programs.org_id` is a foreign key to `organizations.id` whose delete behavior refuses the
   delete outright rather than cascading or nulling it out. `program_shares` is its own table with
   a composite primary key of `(program_id, org_id)` and foreign keys to both, each set to cascade
   on delete. The "unique name per owning org, while active" rule cannot be a plain `unique
   (org_id, name)` constraint, because that would still reject a reused name after a soft delete;
   it needs to be scoped to only the rows where `deleted_at` is empty, which in Postgres means a
   partial unique index rather than a table-level `unique (...)` constraint.

## S1.3: tasks

1. Which of `title`, `status`, `estimate_minutes` needs a bound on its *value*, versus which of
   `due_on`, `internal_note`, `assignee_id` just needs to be allowed to be empty?
2. Course lesson 106, section "03-constraints-are-the-api," walks through choosing a delete
   behavior per foreign key relationship, including the exact case of an assignee who leaves and a
   task that should not be deleted or blocked by that. This phase's primer covers why this
   project's `status` is text-plus-check rather than a native enum, if you are wondering why the
   course's version of this table looks slightly different from this project's contract.
3. `title` gets a check constraint on its character length being between 1 and 200. `status` gets
   a check constraint against the four allowed values, plus a column default of `'todo'` so an
   insert that omits it still gets a sensible value. `estimate_minutes` gets a check constraint
   that it is either null or zero-or-greater. `assignee_id` is a foreign key to `users.id` whose
   delete behavior clears the column instead of blocking the user's removal or deleting the task.

## S1.4: dependencies between tasks

1. What SQL comparison operator expresses "these two values must not be equal," and which table
   would you attach that check to?
2. Course lesson 106, section "03-constraints-are-the-api," shows the cascading-delete pattern for
   a table whose two foreign keys both point at the same table (a task pointing at another task).
3. `task_dependencies` gets a check constraint comparing its two task-referencing columns for
   inequality, and both of those columns are foreign keys to `tasks.id` with a cascading delete
   action, so removing either task in a pair removes the dependency row with it.

## S1.5: audit events

1. What Postgres feature assigns an increasing numeric value automatically on insert, without the
   caller ever supplying one?
2. Course lesson 106, section "02-write-the-schema," uses `uuid` primary keys with a default
   throughout; `task_events` is the one table in this phase where a different, auto-incrementing
   kind of primary key makes more sense. Think about why an audit log benefits from an ordering
   that a UUID does not give you for free.
3. `task_events.id` is a generated identity column (Postgres's `generated always as identity`, or
   an equivalent auto-incrementing integer primary key) rather than a value the caller supplies.
   Its other columns are an ordinary foreign key to `tasks.id` with a cascading delete, an
   optional foreign key to `users.id`, and plain text columns for the kind of event and the
   before/after status.

## S1.6: the database refuses bad data

1. This story doesn't add new tables; which of your existing constraints from S1.1 through S1.4
   would you actually try to violate to prove they work? Have you tried inserting the bad row by
   hand in `psql` (or through `pnpm db:status`'s connection) to see what error Postgres gives you?
2. Course lesson 109, section "04-safe-migration-review," is written from the reviewer's seat:
   read it for the habit of asking "what bad insert would this constraint actually catch" before
   you trust that it does.
3. There is no new shape here beyond what S1.1 through S1.4 already describe; this story is about
   verifying those constraints reject the specific bad inputs the acceptance criteria list, not
   about writing anything new.

## S1.7: `updated_at` maintains itself

1. An `update` statement that only changes a task's `title` doesn't mention `updated_at` at all.
   What would have to run automatically, as a side effect of that statement, for `updated_at` to
   change anyway?
2. Course lesson 106, section "02-write-the-schema," names the mechanism directly: a trigger
   function, attached to both tables that need this behavior.
3. Write a small `plpgsql` function that sets `new.updated_at` to the current time and returns
   `new`, then attach it as a `before update` trigger on both `programs` and `tasks`. A default
   value on the column (`default now()`) is not enough on its own, because a default only applies
   on insert, not on every later update.

## S1.8: the board query is indexed

1. Without any index beyond each table's primary key, how would Postgres have to answer "give me
   this program's tasks with this status, ordered by due date"? What would `EXPLAIN` show for that
   plan?
2. Course lesson 106, section "01-modeling-the-domain," is about designing from the queries the
   application actually runs; treat this specific query (filter by program and status, exclude
   soft-deleted rows, order by due date) as the access pattern to design an index around.
3. Add an index on `tasks` covering `program_id`, `status`, and `due_on` together, in an order
   that lets Postgres use it for both the equality filters and the ordering in one pass; a partial
   index restricted to `deleted_at is null` (the same technique as S1.2's active-title uniqueness)
   keeps it from wasting space on rows the query will never match. Confirm it worked by running
   `explain` on the query yourself against a table with a few thousand rows, the same way the
   acceptance test does, rather than assuming the index is being used.

# Primer: schema

## Modeling from access patterns

A schema is not a diagram you draw from the domain description alone; it is a diagram you draw
from the questions the application is going to ask of it. Before you write a single table, list
the questions: which tasks does a program have, which of them belong to a given vendor, which
ones are ready to start, who is assigned to what. Each of those questions implies a shape (which
columns exist, which relationships need an index, which values must never be null) that is easy
to miss if you model the domain's nouns first and think about the queries later. Course lesson
106, section "01-modeling-the-domain," walks through this habit in general terms; it is worth
reading before you look at this project's own tables.

## Constraints as the API

A foreign key, a `not null`, a uniqueness rule, a check on an allowed set of values: each of
these is a promise the database itself enforces, not a hope that your application code remembers
to check. That promise is a kind of API. Callers of the database, whether that is your own REST
handlers in phase 2 or a teammate's script six months from now, can rely on the constraint being
true without re-verifying it, the same way a caller of a typed function can rely on its signature
without reading its body. This matters more in a database than almost anywhere else in a system,
because a database's data usually outlives any particular version of the application code that
wrote it. Course lesson 106, section "03-constraints-are-the-api," covers this idea directly.

## Explicit sharing in a multi-tenant schema

A multi-tenant system is one where rows belong to different organizations and must not leak
across that boundary by accident. The simplest version of multi-tenancy is "every row has an
owning organization, and every query filters by it." This project's domain adds a second
dimension: a resource can be explicitly shared with an organization that does not own it. That
means "which organizations can see this program" is not implied by ownership alone; it needs its
own relationship, separate from the row's owner, with its own rules about what a shared party can
do versus what the owner can do. Course lesson 106, section "04-sharing-and-visibility," covers
this pattern in general terms; think about what it would mean for a query to accidentally forget
the sharing relationship and only check ownership.

## Text and a check constraint versus a native enum, for a fixed set of values

A column that can only ever be one of a short, known set of values (a status, a role) can be
modeled two ways: a native enum type, or a plain text column with a check constraint limiting it
to the allowed values. An enum reads nicely and the database will reject a bad value either way,
but an enum's set of values is part of the database's type system, and in Postgres specifically,
growing that set later (adding a new status) is a schema change with sharper edges than adding a
value to a check constraint's list: it cannot be done inside the same transaction as other DDL,
and older Postgres versions could not remove a value at all. A text column with a check constraint
is less elegant to look at, but changing the allowed set later is a one-line, ordinary migration,
and every layer above the database (Go, JSON, GraphQL) already treats it as a plain string with no
extra mapping step. The choice comes down to how likely the allowed set is to change: a value set
that is closer to a business decision than a structural fact about the data tends to age better as
a check constraint than as a type.

## Migrations as code, and why they must be re-runnable

A migration is a file, checked into version control, that describes one change to the database's
shape. Treating schema changes as code (reviewed, ordered, applied the same way on every machine)
is what makes "everyone's local database looks like everyone else's" a property you can rely on,
the same way a lockfile does for dependencies. Two properties matter enough to test directly: a
migration must apply cleanly to a completely empty database (a new teammate, a fresh CI run, a
disaster-recovery restore all start from nothing), and the whole migration history must be
re-runnable from scratch on demand, not just appendable. Course lesson 109, section
"01-migrations-as-code," introduces the general idea; section "04-safe-migration-review" is worth
a look for what reviewers of a migration tend to check.

## The interview angle

A take-home or live-coding schema exercise is usually judged less on "does it work" and more on
"did you think about the failure modes before they happened": what happens when two people submit
the same title at once, what happens when someone tries to make a task depend on itself, what
happens when a query that used to return ten rows now has to search a hundred thousand. Building
those guarantees into the schema, rather than into every place the application happens to touch
the data, is the habit this phase is training.

## Questions to answer for yourself

1. Look at the tables this phase's stories describe. Which relationships would break silently (no
   error, just wrong data) if you left out a constraint, versus which ones would break loudly
   (an error your tests would catch immediately)?
2. Soft deletes (a `deleted_at` column instead of an actual `DELETE`) complicate uniqueness: "no
   two active tasks in a program share a title" is a different rule than "no two rows in this
   table share a title." What has to be different about how that uniqueness rule is scoped?
3. Why might a migration that runs perfectly on a database that already has data still be wrong to
   ship, if it has never been tested against a completely empty one?

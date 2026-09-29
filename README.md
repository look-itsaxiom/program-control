# Program Control

## What this is

Program Control is a build-it-yourself interview project. You are building a small system for
running an engine test campaign: one **program** owned by a customer org and shared with two
vendor orgs. Users belong to orgs through memberships with roles. Tasks belong to the program,
have a status (`todo`, `doing`, `blocked`, `done`), an optional assignee, an estimate in minutes,
and a due date. Dependencies are edges between tasks, where a predecessor must be done before its
successor is "ready". Vendors see the tasks shared with them and the status of their
predecessors, never the customer's internal notes. Status changes append to an audit log, and
vendor systems push updates through a webhook. You build the Postgres schema, the Go REST API,
the React UI, the GraphQL layer, and, if you have time, the integrations and permissions work.
Each phase below is a checkpoint with its own stories and acceptance tests, and a mentor (Claude
Code, guided by `CLAUDE.md`) is available the whole way through to answer questions and give
hints, never to write the code for you.

| Phase | Outcome | Forces | Course refs | Chase's time |
|---|---|---|---|---|
| 0 Environment and scaffold | From a blank machine: install Git, Go, Node + pnpm, Docker Desktop, the GitHub CLI, an editor, and Claude Code; clone; setup runs; tests are red; he can read the skeleton and the mentor works | Toolchain, Go tool, pnpm, Docker, CI | 102, 92, 35 | 1 h |
| 1 Schema | Migrations create orgs, users, memberships, programs, shares, tasks, dependencies, audit; constraints hold | PostgreSQL, migrations | 106, 109 | 2 h |
| 2 REST API | Tasks list/create, dependency add with cycle rejection, ready list, status change with audit, JSON error envelope, request ids | Go net/http, middleware, httptest, pgx | 103, 104 | 3 h |
| 3 React UI | Task table with filter and sort, dependency editor with cycle feedback, optimistic status toggle with rollback, Tailwind layout | React 19, Tailwind | 113, 03, 54, 18 | 3 h |
| 4 GraphQL | gqlgen schema and resolvers over the same store, dataloaders (query count bounded), Apollo Client on the UI with a normalized cache | gqlgen, Apollo | 110, 111, 108 | 3 h |
| 5 Integrations | Signed vendor webhook with replay and idempotency guards, retrying outbound notifier, background worker | Go integrations | 105 | stretch |
| 6 Permissions and realtime | Cross-org visibility rules, audit log endpoint, SSE feed of task changes | System design | 112, 106 | stretch |
| 7 Ship | CI with Postgres service green, seed script, demo script, README decisions and not-done | GitHub Actions, release safety | 92, 93, 114 | stretch |

Phases 0 through 4 are the core path (aim for under 12 hours total). Phases 5 through 7 are
stretch goals, and each phase is demoable when it is done.

## Start here

You need nothing installed yet. Clone this repo, open a terminal in it, and run the doctor
script for your OS:

- Windows: `powershell -ExecutionPolicy Bypass -File scripts/doctor.ps1`
- macOS or Linux: `bash scripts/doctor.sh`

It checks Git, Go, Node, pnpm, Docker, the GitHub CLI, and Claude Code, and tells you what is
missing or too old, with a link to install it. Re-run it after installing anything, then run:

```
pnpm setup
```

This installs dependencies, starts Postgres in Docker, downloads Go modules, and applies
migrations. It ends by telling you what to do next.

## Daily loop

- `pnpm dev` runs the API and the web app together.
- `pnpm check <phase>` runs that phase's acceptance tests and prints a scoreboard. `pnpm check`
  with no argument runs every phase and prints a summary table. Add `--verbose` to see the raw
  test output as it runs. For example, `pnpm check 1` after finishing the schema stories prints:

  ```
  Phase 1: 4/4 passing
    ✓ TestPhase1_OrgsTableExists
    ✓ TestPhase1_MembershipRoleConstraint
    ✓ TestPhase1_TaskStatusConstraint
    ✓ TestPhase1_DependencyCycleRejected
  ```

  A failing test shows `✗` with the first line of the failure message underneath it, and the
  command exits non-zero.
- Run `claude` in this folder any time you want to ask your mentor a question, get a hint, or
  have your code reviewed. The mentor is Claude Code itself, reading `CLAUDE.md` and the `mentor`
  skill in `.claude/`, so there is nothing to start or configure.

## The mentor

Ask it anything: why a query returns the wrong rows, what a partial index is, whether your plan
for the dependency editor is overcomplicated, or what a failing test in `pnpm check 2` is
actually claiming. It will read your code and run the phase tests when that helps.

- **Ask a question** and you get a question back, or a concept explained in full, or a pointer
  into the phase primer or a course lesson. Ask for a concept ("explain dataloaders") and you
  get the whole concept, taught properly.
- **Say "hint"** to get the first rung of the ladder for the story you are on, and "next hint"
  for the one after. There are three rungs per story, each more concrete than the last, and the
  mentor tells you which rung you are on. It never volunteers one.
- **Paste code and ask for a review** and it names what is wrong, what that costs in production
  or in an interview, and where to look, without fixing it for you.

What it will not do: write, paste, or edit implementation code under `api/internal`,
`api/migrations`, `api/cmd`, `api/phase_tests`, or `web/src`; state the answer to a story; hand
you a hint you did not ask for; or look at the `solutions` branch. `.claude/settings.json` denies
the file tools on those paths as a second line of defence behind that instruction, and a mentor
asked to edit one of them has been observed trying and being refused by the permission system
(`docs/mentor-validation.md`, "Guard exercised"). Treat it as a backstop rather than a proof: it
covers the file tools by path, not every route a determined prompt could take. That is the whole
point of the exercise anyway: you are building this so you can answer questions about it later.

The first time you run `claude` here it asks whether you trust this folder. Say yes, or it will
keep asking permission every time it wants to run `pnpm check` for you.

## Rules of the game

- Do not open any `phases/*/.mentor/` folder. That is the mentor's hint ladder, and reading it
  yourself defeats the point.
- Do not check out the `solutions` branch until the phase you are on is green. It exists so every
  story is provably buildable, not so you can copy it.

## Layout

```
api/            Go module: cmd/api, cmd/migrate, migrations, internal/*, phase_tests/
web/            Vite + React + TypeScript app, with its own phase tests
phases/         PRIMER.md and STORIES.md per phase, plus the mentor's hidden hints
scripts/        setup, dev, check, and db orchestration (Node, no dependencies)
docs/superpowers/  the spec and plan behind this project
```

## Troubleshooting

- **Port 5433 already in use.** Something else on your machine is using it. Stop that service or
  change the port mapping in `docker-compose.yml` and in `DATABASE_URL`.
- **Docker commands hang or fail.** Make sure Docker Desktop is running. `docker info` should
  succeed before `pnpm setup` or `pnpm db:up` will work.
- **`go: command not found` right after installing Go.** Close and reopen your terminal so it
  picks up the updated `PATH`, then try again.
- **Go version errors.** This project needs Go 1.26 or newer; an older Go will try to download the
  right toolchain automatically the first time you run a go command, which needs network access.

## Decisions

These are the choices the stories left open, and why this branch went the way it did.

- **The database refuses bad data, the API does not merely check it.** Every rule phase 1 names
  is a constraint or a partial unique index, and the API finds out about a duplicate title by
  failing an insert rather than by reading first and writing second. A check-then-write would be
  wrong under concurrency, and it would put the same rule in two places.
- **Not-found is the only answer for "not yours".** A stranger asking about a program, a vendor
  asking about a task outside their slice, and anybody asking about a row that never existed all
  get the same 404. A 403 would confirm the thing is there, which is the leak the story is about.
- **The error envelope is produced by the handler, and the router's own errors are rewritten to
  match.** Go's ServeMux generates a plain-text 404 and 405 by itself. A catch-all `/` route would
  have swallowed the 405 entirely, because it matches every method, so a thin response writer
  intercepts those two statuses instead and re-emits them as the shared JSON shape.
- **To-one relations are joined, to-many relations are batched.** A task's assignee comes back
  with the task in the same query; its predecessors come from one per-request loader keyed by
  program. Batching by program rather than by task id means the loader has no timing window to
  tune, so the query count is the same every run instead of usually small.
- **Readiness is computed on the client under GraphQL, and read from the server under REST.** The
  REST board has no way to know the dependency graph, so it asks for the ready list. The GraphQL
  board already holds each task's predecessors and their statuses in a normalized cache, so it
  works readiness out locally, which is what lets a mutation on one task flip a downstream task's
  badge with no second request.
- **Delivery and the live stream both hang off one publish point.** A status change, whether it
  came from REST, the webhook, or a GraphQL mutation, is handed to the same function after the
  response is written. Nothing on the request path waits for either.
- **The identity header has a query-parameter twin on the stream route only.** The browser's
  `EventSource` cannot set a request header, and the header here stands in for a login rather than
  being a credential, so carrying it in the query string on that one route costs nothing real.

## Not done

- **Authentication is still a header.** There is no session, token, or password anywhere. That was
  the stories' own decision, and everything downstream of it (the caller's organizations, the
  tenant rules) would survive replacing it, but it is the first thing that would have to change
  before any of this faced the internet.
- **The UI cannot create a task or remove a dependency.** Both routes exist and are tested on the
  API side. The board reads, changes a status, and adds a dependency, because those are what the
  phase 3 and 4 stories asked to see; the rest would be more of the same form handling.
- **The notifier's queue lives in memory.** A change enqueued when the process dies is lost, and a
  full queue drops rather than blocks. Making delivery survive a restart means an outbox table and
  a worker polling it, which is a different story than the one phase 5 tells.
- **The live stream is single-process.** The broker fans out inside one server. Two instances
  behind a load balancer would each only see their own writes; that needs Postgres `LISTEN/NOTIFY`
  or a message bus.
- **Vendor narrowing applies to the REST routes only.** Phase 6's contract names those routes, and
  the GraphQL layer still answers with phase 2's tenant rules. Extending the narrowing to the graph
  would mean threading the same visibility set through the task resolvers.
- **One phase 4 acceptance test cannot pass alongside phase 3's.** It asks the dependency
  `<option>` elements to have the bare task title as their accessible name, while phase 3 asks
  `getByText` and `getByLabelText` to find exactly one element per title across the whole board.
  Any accessible name equal to the title makes one of the two queries ambiguous. This branch keeps
  phase 3 green; the details are in the task 11 report.

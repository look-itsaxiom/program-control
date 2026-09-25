# Program Control: a build-it-yourself interview project

Date: 2026-09-25. Status: approved in conversation.

## Goal

Give Chase one project that exercises the whole stack of the Integrate Full Stack Software
Engineer role (React, Go, GraphQL, REST, PostgreSQL, GitHub Actions, Tailwind) through a series
of build phases, learned by doing rather than by reading. He has under 15 hours before the
interview is scheduled and expects more once it is. The React Refresher course
(`C:\Users\ChaseSkibeness\Projects\ReactRefresher`, 115 lessons) stays as the reference he looks
things up in; this project is what he does.

Three requirements from Chase shape everything:

1. **User stories, not instructions.** Each phase says what to accomplish and how it will be
   judged, never how to do it. Where tests must talk to his code, the story states that contract
   as a product requirement (a URL, a JSON shape, an exported function) and leaves the rest open.
2. **Teaching woven in.** Each phase opens with a one-page primer on the concept, why it matters
   for this product and this interview, and questions to answer for himself before coding, with
   pointers into the course by lesson number.
3. **A mentor, not a solver.** He can ask questions and get guided to an answer. The mentor never
   writes implementation code or states the answer; it asks back, points, and climbs a hint ladder
   only when asked.

## Non-goals

- No hosted deployment of the app (a `docker compose up` demo is the target).
- No authentication provider integration; a signed-in principal is simulated by a header in the
  core phases and made real only as a stretch story.
- No mobile UI; one responsive screen set.
- The mentor does not grade; the acceptance tests do.

## The domain

One **program** (an engine test campaign) owned by a **customer org** and shared with two
**vendor orgs**. **Users** belong to orgs through **memberships** with roles. **Tasks** belong to
the program, have a status (`todo`, `doing`, `blocked`, `done`), an optional assignee, an estimate
in minutes, and a due date. **Dependencies** are edges between tasks (predecessor must be done
before the successor is "ready"). Vendors see the tasks shared with them and the status of their
predecessors, never the customer's internal notes. Status changes append to an **audit log**.
Vendor systems push updates through a **webhook**.

This is the same shape as the course's lessons 106 (schema), 107 (queries), 103 (REST), 110
(GraphQL in Go), 105 (integrations), and 112 (system design), so their solutions are legitimate
references when he chooses to look.

## Repository

`look-itsaxiom/program-control`, public, one repo:

```
program-control/
  README.md                 how to set up, run, check a phase, ask the mentor
  CLAUDE.md                 the mentor contract (loaded by any Claude Code session here)
  .claude/skills/mentor/    the mentor skill: behaviour, hint-ladder protocol, review mode
  package.json              root scripts: setup, dev, check <phase>, db:*
  pnpm-workspace.yaml       workspace: web
  docker-compose.yml        postgres:17 on port 5433 with a healthcheck
  .github/workflows/ci.yml  go vet/test with a Postgres service, pnpm typecheck/test, SHA-pinned
  scripts/check.mjs         runs one phase's acceptance tests (Go + Vitest) and prints a scoreboard
  api/                      Go module github.com/look-itsaxiom/program-control/api
    go.mod
    cmd/api/main.go         boots the HTTP server (Chase fills in most of it)
    cmd/migrate/main.go     goose up/down/status over api/migrations (provided, complete)
    migrations/             0001_init.sql ... (Chase writes them; 0000_bootstrap.sql provided)
    internal/testdb/        test helper: connect, migrate up, reset between tests (provided)
    internal/store/         Chase's data layer (stub compiles)
    internal/httpapi/       Chase's REST layer (stub compiles; exports NewHandler)
    internal/graph/         gqlgen config + schema.graphqls (Chase writes schema and resolvers)
    internal/worker/        stretch: background processing
    phase_tests/            acceptance tests per phase: phase1_test.go, phase2_test.go, ...
  web/                      Vite + React 19 + TS + Tailwind 4 + Apollo Client 4 + Vitest
    src/                    Chase's UI (App.tsx placeholder provided)
    src/phase-tests/        acceptance tests per phase: phase3.test.tsx, phase4.test.tsx
  phases/
    0-scaffold/  1-schema/  2-rest-api/  3-react-ui/  4-graphql/  5-integrations/
    6-permissions-and-realtime/  7-ship/
      PRIMER.md             what you are about to learn and why it matters here (one page)
      STORIES.md            user stories with acceptance criteria and any required contracts
      .mentor/hints.md      hint ladder per story, read only by the mentor
  docs/superpowers/         this spec and the plan
```

The reference solution lives on a branch named `solutions` that is never merged and that the
README tells Chase not to open until he has finished a phase. It exists so every acceptance test
is proven achievable and every story is complete enough to be built from.

## Phases

Core (target under 12 hours of Chase's time): 0 to 4. Phase 0 assumes NO development environment at all: its first stories are about producing one, verified by `scripts/doctor.ps1` (Windows) / `scripts/doctor.sh` (macOS, Linux), which need only the shell the OS ships with and report each tool as present with an acceptable version, missing, or too old, with the official download page for anything missing. Stretch: 5 to 7. Each phase is demoable
when done.

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

### Stories

Stories follow "As a <role>, I need <outcome>, so that <why>", each with acceptance criteria
written as observable behaviour, and, where needed, a **Contract** block naming what the tests
will call. Contracts are the only place a name or shape is dictated. Examples of the level:

- Phase 1: "As the API, I need the database to refuse a dependency from a task to itself and a
  duplicate task title within a program that is still active, so that bad data cannot enter."
  Contract: migration files under `api/migrations`, applied by `pnpm db:migrate`; table and
  column names are listed because every later phase and test queries them.
- Phase 2: "As a vendor engineer, I need to see which tasks are ready to start, so that I can plan
  the shift." Acceptance: `GET /programs/{id}/tasks/ready` returns only tasks whose every
  predecessor is done, including tasks with no predecessors; a task with a `blocked` status is
  never ready. Contract: `httpapi.NewHandler(ctx, cfg) (http.Handler, error)`, the error envelope
  `{"error":{"code":"...","message":"..."}}`, and the request/response shapes.
- Phase 3: "As a program manager, I need to flip a task's status and see it change immediately,
  and see it flip back with a message if the save fails, so that the board feels live and honest."
  Contract: the UI talks to the phase 2 API; tests replace `fetch` with a fake that follows the
  same contract.
- Phase 4: "As the UI, I need to load a program with its tasks and their assignees in one request
  without the server running one query per task, so that the board loads fast at scale."
  Acceptance: a query-count probe sees at most 4 SQL statements for 3 projects × 4 tasks.

### Acceptance tests

- Go tests in `api/phase_tests/` named `TestPhaseN_...`, run against the real Postgres from
  `docker-compose.yml` using `internal/testdb` (migrate up, `TRUNCATE ... RESTART IDENTITY
  CASCADE` between tests). They call exported contracts only (`httpapi.NewHandler`, the
  database). Phase 4 wraps the pool with a pgx tracer to count statements.
- Vitest tests in `web/src/phase-tests/` tagged by phase in their `describe` names; they render
  `App` or exported components from `web/src`, with `fetch`/GraphQL faked through a small
  in-test server that implements the contract.
- `pnpm check <phase>` runs both and prints one line per test with pass/fail and a phase score.
  `pnpm check` with no argument runs everything.

### Skeleton (what Chase receives)

- `scripts/doctor.ps1` / `doctor.sh`: checks git, go (1.22+), node (22+), pnpm (10+), docker (daemon reachable, compose v2), gh (logged in), claude (Claude Code CLI on PATH) and prints a table; exit code non-zero when anything required is missing. Phase 0's stories point at it, and `pnpm setup` runs it first.
- `pnpm setup`: installs web deps, `docker compose up -d --wait`, `go mod download`, runs
  `db:migrate` (which applies only `0000_bootstrap.sql` on a fresh clone), prints next steps.
- `pnpm dev`: runs the API (`go run ./cmd/api`) and the web app (Vite) together.
- Compiling stubs: `store` and `httpapi` packages exist with a `NewHandler` returning a 501
  handler; `cmd/api` serves `/healthz`; `graph` folder holds `gqlgen.yml` and an empty
  `schema.graphqls`; `App.tsx` renders a placeholder.
- All acceptance tests present and red except phase 0's (which check the skeleton itself).
- CI green on the skeleton (tests for phases 1+ are skipped in CI until `PHASE=N` is set in
  `ci.yml`, which Chase raises as he goes; the stories for phase 7 ask for the full pipeline).

### Mentor

`CLAUDE.md` is short and binding for any session in the repo: you are a mentor; the learner is
building this to learn; never write implementation code, never paste a solution, never edit
files under `api/internal`, `api/migrations`, `api/cmd`, or `web/src` (you may read them); answer
a question with a question, a concept explanation, or a pointer to a phase primer or course
lesson; give hints only when asked, one rung at a time from `phases/N/.mentor/hints.md`; in
review mode, name what is wrong and why it matters without fixing it; explain any concept fully
when asked for the concept rather than the solution; when the learner is truly stuck after the
last hint, describe the shape of the solution in prose (no code). The `mentor` skill carries the
protocol in detail (modes: question, hint, review, concept, plan-check) and a self-check list the
mentor runs before every reply ("does this reply contain code the learner should write?").

Mentor validation before handover: a scripted transcript of 8 prompts (a direct "just write the
migration for me", a vague "why is my ready query wrong", "hint", "next hint", "review my
handler", "explain dataloaders", "what should the schema look like", "am I overthinking this")
is run through a fresh Claude Code session in the repo and checked for the contract.

## Testing of the framework itself

- `doctor` correctly reports a missing tool: verified by running it with PATH stripped of one tool.
- Fresh clone on this machine: `pnpm setup` succeeds; `pnpm check 0` passes; `pnpm check 1`
  through `4` are all red with readable failures; `pnpm dev` serves `/healthz` and the placeholder.
- On `solutions`: `pnpm check` passes every phase, including stretch, and CI is green with
  `PHASE=7`.
- Mentor transcript check as above.

## Risks

- Docker on Windows: compose port 5433 avoids a local Postgres clash; `--wait` needs compose v2.17+
  (v5.1 present).
- gqlgen generation needs network for `go mod download`; the skeleton pre-downloads modules in
  `pnpm setup`.
- Test flakiness from shared database state: `testdb` truncates between tests and tests run
  serially (`-p 1`).
- Hint leakage: hints live in a dotfolder the README asks him not to open; the mentor is the
  intended reader.

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

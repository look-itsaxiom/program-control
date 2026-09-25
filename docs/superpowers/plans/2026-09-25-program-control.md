# Program Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the skeleton, acceptance tests, phase stories, mentor, and reference solution for `program-control`, a build-it-yourself full-stack project (Go, PostgreSQL, React, GraphQL, GitHub Actions) that Chase completes phase by phase with a Socratic mentor.

**Architecture:** One repo: a Go module under `api/` (pgx, goose, gqlgen), a Vite React app under `web/` (Tailwind 4, Apollo Client 4, Vitest), Postgres 17 in Docker Compose on port 5433, a root `package.json` whose scripts orchestrate everything, and a `phases/` tree of primers, stories, and mentor-only hints. Acceptance tests live in `api/phase_tests/` (Go, real database) and `web/src/phase-tests/` (Vitest); `scripts/check.mjs N` runs one phase and prints a scoreboard. The mentor is a `CLAUDE.md` contract plus a `.claude/skills/mentor` skill. The reference solution lives on the `solutions` branch only.

**Tech Stack:** Go 1.25 (`go 1.24` in go.mod for compatibility), `github.com/jackc/pgx/v5`, `github.com/pressly/goose/v3`, `github.com/99designs/gqlgen`, Node 22, pnpm 10, Vite 7, React 19, TypeScript 5.9, Tailwind 4, `@apollo/client` 4, Vitest 3, Testing Library, Docker Compose v2, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-09-25-program-control-design.md`

## Global Constraints

- Work happens on branch `main` of `C:\Users\ChaseSkibeness\Projects\program-control` for the skeleton and content; the reference solution goes only on branch `solutions` (Task 11) and is never merged.
- Nothing in `phases/*/STORIES.md` or `PRIMER.md` states HOW to implement; only WHAT, acceptance criteria, and explicit **Contract** blocks. A reviewer must be able to point at any implementation detail in a story and call it a defect.
- Acceptance tests call only contracts named in the stories: `httpapi.NewHandler`, the database, `web/src/App` and named exports listed in stories, `/graphql`. They never import `internal/store` directly.
- Every Go acceptance test is named `TestPhaseN_<Story>` and lives in `api/phase_tests/`; every Vitest acceptance test is inside `describe('phase N ...')` in `web/src/phase-tests/`.
- Windows first: scripts run under PowerShell 7 and Git Bash on Windows and under bash on macOS/Linux; no shell-specific syntax inside `package.json` scripts (use `node scripts/*.mjs`).
- Postgres runs on host port 5433; `DATABASE_URL=postgres://postgres:postgres@localhost:5433/program_control?sslmode=disable`.
- GitHub Actions are pinned by commit SHA with a version comment: `actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1`, `actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0`, `actions/setup-node@820762786026740c76f36085b0efc47a31fe5020 # v7.0.0`, `pnpm/action-setup@ea17c68df8912ef543352723c149a84f56e3d413 # v6.1.0`.
- Commit after every task with a Conventional Commit and the trailer `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`; never amend or force-push.
- No em-dashes in any prose written for Chase.

---

### Task 1: Repository scaffold, doctor scripts, compose, root scripts, CI

**Files:**
- Create: `.gitignore`, `.gitattributes`, `README.md`, `package.json`, `pnpm-workspace.yaml`, `docker-compose.yml`, `.env.example`, `scripts/doctor.ps1`, `scripts/doctor.sh`, `scripts/setup.mjs`, `scripts/dev.mjs`, `scripts/db.mjs`, `.github/workflows/ci.yml`, `.editorconfig`

**Interfaces:**
- Produces: root scripts `setup`, `dev`, `check` (delegates to `scripts/check.mjs`, created in Task 4), `db:up`, `db:down`, `db:migrate`, `db:status`, `db:reset`, `doctor`; the env var `DATABASE_URL` default; the CI matrix of jobs.

- [ ] **Step 1: Write the doctor scripts (they are phase 0's acceptance surface)**

`scripts/doctor.ps1` (PowerShell 5.1 compatible, since a blank Windows machine has only that):

```powershell
# Reports whether the toolchain needed for program-control is installed. Needs nothing but Windows PowerShell.
$ErrorActionPreference = 'Continue'
$rows = @()
function Check($name, $required, $cmd, $minVersion, $url, [scriptblock]$parse) {
  $exe = Get-Command $cmd -ErrorAction SilentlyContinue
  if (-not $exe) { $script:rows += [pscustomobject]@{ Tool=$name; Status='MISSING'; Version=''; Need=$minVersion; Get=$url; Required=$required }; return }
  try { $v = & $parse } catch { $v = '' }
  $ok = $true
  if ($minVersion -and $v) { $ok = ([version]($v -replace '[^0-9.].*$','') -ge [version]$minVersion) }
  $script:rows += [pscustomobject]@{ Tool=$name; Status=$(if ($ok) {'ok'} else {'TOO OLD'}); Version=$v; Need=$minVersion; Get=$url; Required=$required }
}
Check 'git'    $true 'git'    '2.30' 'https://git-scm.com/downloads'            { (git --version) -replace 'git version ','' }
Check 'go'     $true 'go'     '1.22' 'https://go.dev/dl/'                        { (go version) -replace '^go version go','' -replace ' .*$','' }
Check 'node'   $true 'node'   '22.0' 'https://nodejs.org/'                       { (node --version) -replace '^v','' }
Check 'pnpm'   $true 'pnpm'   '10.0' 'https://pnpm.io/installation'              { pnpm --version }
Check 'docker' $true 'docker' '24.0' 'https://www.docker.com/products/docker-desktop/' { (docker version --format '{{.Client.Version}}') }
Check 'gh'     $false 'gh'    '2.40' 'https://cli.github.com/'                   { (gh --version | Select-Object -First 1) -replace '^gh version ','' -replace ' .*$','' }
Check 'claude' $false 'claude' ''    'https://docs.claude.com/en/docs/claude-code/overview' { (claude --version 2>$null | Select-Object -First 1) }
# Docker daemon and compose v2
$daemon = 'MISSING'
try { docker info --format '{{.ServerVersion}}' 2>$null | Out-Null; if ($LASTEXITCODE -eq 0) { $daemon = 'ok' } } catch {}
$rows += [pscustomobject]@{ Tool='docker daemon'; Status=$daemon; Version=''; Need='running'; Get='Start Docker Desktop'; Required=$true }
$compose = 'MISSING'
try { $cv = docker compose version --short 2>$null; if ($LASTEXITCODE -eq 0 -and $cv) { $compose = 'ok' } } catch {}
$rows += [pscustomobject]@{ Tool='docker compose'; Status=$compose; Version=$cv; Need='v2'; Get='Included with Docker Desktop'; Required=$true }
$rows | Format-Table Tool, Status, Version, Need, Get -AutoSize
$missing = $rows | Where-Object { $_.Required -and $_.Status -ne 'ok' }
if ($missing) { Write-Host "`nRequired tools missing or too old: $($missing.Tool -join ', ')" -ForegroundColor Red; exit 1 }
Write-Host "`nEverything required is installed." -ForegroundColor Green
```

`scripts/doctor.sh`: the same checks in POSIX sh (`command -v`, version parsing with `sed`, `sort -V` for comparison), same table, same exit codes, same URLs.

- [ ] **Step 2: Root package.json and workspace**

```json
{
  "name": "program-control",
  "private": true,
  "packageManager": "pnpm@10.32.1",
  "engines": { "node": ">=22" },
  "scripts": {
    "doctor": "node scripts/doctor-launch.mjs",
    "setup": "node scripts/setup.mjs",
    "dev": "node scripts/dev.mjs",
    "check": "node scripts/check.mjs",
    "db:up": "node scripts/db.mjs up",
    "db:down": "node scripts/db.mjs down",
    "db:migrate": "node scripts/db.mjs migrate",
    "db:status": "node scripts/db.mjs status",
    "db:reset": "node scripts/db.mjs reset",
    "api:test": "node scripts/db.mjs ensure && cd api && go test ./...",
    "web:test": "pnpm --filter web test"
  }
}
```

`scripts/doctor-launch.mjs` runs `doctor.ps1` via `powershell -ExecutionPolicy Bypass -File` on Windows and `sh doctor.sh` elsewhere, propagating the exit code. `pnpm-workspace.yaml`: `packages: ['web']`.

- [ ] **Step 3: Compose, env, db script**

`docker-compose.yml`:

```yaml
services:
  db:
    image: postgres:17
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: program_control
    ports: ["5433:5432"]
    volumes: ["pgdata:/var/lib/postgresql/data"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d program_control"]
      interval: 2s
      timeout: 3s
      retries: 30
volumes:
  pgdata:
```

`.env.example` holds `DATABASE_URL=postgres://postgres:postgres@localhost:5433/program_control?sslmode=disable` and `API_ADDR=:8080`. `scripts/db.mjs <up|down|migrate|status|reset|ensure>`: `up` = `docker compose up -d --wait`; `down` = `docker compose down`; `migrate`/`status` = `go run ./cmd/migrate up|status` from `api/` with `DATABASE_URL` from `.env` (fallback to the default); `reset` = `go run ./cmd/migrate reset` then `up`; `ensure` = `up` only if `pg_isready` via `docker compose exec db pg_isready` fails. Load `.env` with a 10-line parser (no dependency).

- [ ] **Step 4: setup and dev scripts**

`scripts/setup.mjs`: run doctor (fail fast), `pnpm install`, `docker compose up -d --wait`, `cd api && go mod download`, `node scripts/db.mjs migrate`, then print "Next: open phases/0-scaffold/STORIES.md, run pnpm check 0, and run `claude` in this folder to meet your mentor." `scripts/dev.mjs`: spawn `go run ./cmd/api` (cwd `api`, env from `.env`) and `pnpm --filter web dev` concurrently, prefixing output with `[api]`/`[web]`, killing both on Ctrl+C.

- [ ] **Step 5: CI**

`.github/workflows/ci.yml`: on push and pull_request; `permissions: contents: read`; `concurrency` group with cancel-in-progress; job `api` on `ubuntu-24.04` with a `postgres:17` service (env as above, healthcheck options), steps checkout (SHA-pinned, `persist-credentials: false`), setup-go (SHA-pinned, `go-version-file: api/go.mod`, cache), `go vet ./...` and `go test ./... -p 1` in `api/` with `DATABASE_URL=postgres://postgres:postgres@localhost:5432/program_control?sslmode=disable` and `PHASE: ${{ vars.PHASE || '0' }}`; job `web` with pnpm/action-setup and setup-node (SHA-pinned, `cache: pnpm`), `pnpm install --frozen-lockfile`, `pnpm --filter web typecheck`, `pnpm --filter web test`, `PHASE` env likewise; `timeout-minutes: 15` on both. (Task 4 makes tests above `PHASE` skip.)

- [ ] **Step 6: README (setup-first, mentor-second) and housekeeping**

`README.md` sections: What this is (one paragraph, the domain, the phases table copied from the spec), Start here (run `scripts/doctor.ps1` or `.sh`; then `pnpm setup`), Daily loop (`pnpm dev`, `pnpm check N`, ask the mentor with `claude`), Rules of the game (don't open `phases/*/.mentor/`, don't check out `solutions` until a phase is green), Layout, Troubleshooting (port 5433, Docker not running, `go: command not found` after install means reopen the terminal). `.gitignore`: `node_modules`, `web/dist`, `.env`, `api/bin`, `*.log`, `.superpowers`, `.claude/scheduled_tasks.lock`. `.gitattributes`: `* text=auto eol=lf`, `*.ps1 text eol=crlf`. `.editorconfig` basic.

- [ ] **Step 7: Verify and commit**

Run `powershell -ExecutionPolicy Bypass -File scripts/doctor.ps1` (expect all ok on this machine, exit 0) and `bash scripts/doctor.sh` (Git Bash; expect the same). Run `node scripts/db.mjs up` and confirm `docker compose ps` shows healthy; `node scripts/db.mjs down`. Commit: `chore: scaffold repo with doctor, compose, root scripts, and CI`.

---

### Task 2: Go API skeleton (module, migrate tool, testdb, stubs, phase 0 test)

**Files:**
- Create: `api/go.mod`, `api/go.sum`, `api/cmd/api/main.go`, `api/cmd/migrate/main.go`, `api/migrations/0000_bootstrap.sql`, `api/internal/testdb/testdb.go`, `api/internal/store/store.go`, `api/internal/httpapi/handler.go`, `api/internal/graph/gqlgen.yml`, `api/internal/graph/schema.graphqls`, `api/internal/graph/README.md`, `api/phase_tests/phase0_test.go`, `api/phase_tests/helpers_test.go`

**Interfaces:**
- Produces (contracts the stories will cite): `httpapi.NewHandler(ctx context.Context, cfg httpapi.Config) (http.Handler, error)` with `Config{ DatabaseURL string; Logger *slog.Logger }`; `testdb.Open(t *testing.T) *pgxpool.Pool` (skips the test with a clear message when `DATABASE_URL` is unset or unreachable, migrates up once per process, truncates all non-goose tables before each test via `t.Cleanup`); `testdb.Migrations()` embedded FS; `cmd/migrate` subcommands `up|down|status|reset` (reset = down to 0 then up).

- [ ] **Step 1: go.mod and dependencies**

`module github.com/look-itsaxiom/program-control/api`, `go 1.24`. `go get github.com/jackc/pgx/v5@latest github.com/pressly/goose/v3@latest github.com/99designs/gqlgen@latest github.com/vektah/gqlparser/v2@latest`; record the resolved versions in the report. `go mod tidy`.

- [ ] **Step 2: migrate tool with embedded migrations**

`api/migrations/0000_bootstrap.sql` (goose format, `-- +goose Up` / `-- +goose Down`) creates only `create extension if not exists pgcrypto;` (harmless on 17 where `gen_random_uuid` is built in; keeps the file meaningful) and a comment explaining that Chase's migrations start at `0001`. `api/internal/testdb/migrations.go` exposes `//go:embed ../../migrations/*.sql`? Go embed cannot reach `..`; so put the embed in `api/migrations/embed.go` (`package migrations; //go:embed *.sql; var FS embed.FS`). `cmd/migrate/main.go`: goose with `goose.SetBaseFS(migrations.FS)`, dialect postgres, commands `up|down|status|reset|create <name>` (create writes a timestamped or sequential `NNNN_<name>.sql` template into `api/migrations`; use sequential numbering by scanning existing files).

- [ ] **Step 3: testdb**

```go
package testdb
// Open connects using DATABASE_URL, runs migrations up once per process, and registers a cleanup
// that truncates every table in schema public except goose_db_version. Tests are serialized by
// running `go test -p 1`; the helper also takes a package-level mutex.
func Open(t *testing.T) *pgxpool.Pool
// Truncate empties all user tables; exported for tests that need a reset mid-test.
func Truncate(t *testing.T, pool *pgxpool.Pool)
```

Skip behaviour: `t.Skip("DATABASE_URL not set; run `pnpm db:up`")` when unset; when set but unreachable, `t.Fatalf` with the connection error and the same hint (CI must fail loudly, local runs must explain).

- [ ] **Step 4: stubs**

`internal/store/store.go`: `package store` with a doc comment "Your data layer. Nothing here is required by the tests; shape it as you see fit." and one exported type `type DB = *pgxpool.Pool` to make the package non-empty. `internal/httpapi/handler.go`:

```go
type Config struct { DatabaseURL string; Logger *slog.Logger }
// NewHandler returns the HTTP API. This stub answers every route with 501 until you build it.
func NewHandler(ctx context.Context, cfg Config) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "application/json"); _, _ = w.Write([]byte(`{"status":"ok"}`)) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.Error(w, `{"error":{"code":"not_implemented","message":"build me"}}`, http.StatusNotImplemented) })
	return mux, nil
}
```

`cmd/api/main.go`: read `API_ADDR` (default `:8080`) and `DATABASE_URL`, build the handler, `http.Server` with `ReadHeaderTimeout`, serve, `signal.NotifyContext` shutdown with 10 s timeout. `internal/graph/gqlgen.yml` pointing at `schema.graphqls`, generated files under `internal/graph/generated`, models under `internal/graph/model`; `schema.graphqls` empty except a comment; `graph/README.md` says "run `go run github.com/99designs/gqlgen generate` from api/ after writing the schema" (this is a tool command, not a design instruction).

- [ ] **Step 5: phase 0 test and shared helpers**

`api/phase_tests/helpers_test.go`: `newHandler(t) http.Handler` (calls `httpapi.NewHandler` with `DATABASE_URL`), `do(t, h, method, path, body any, headers map[string]string) (*httptest.ResponseRecorder)`, `decode[T](t, rec) T`, `errorCode(t, rec) string`, `phaseGate(t, n int)` which skips when env `PHASE` is set and lower than `n` (used by CI) and never skips locally. `phase0_test.go`: `TestPhase0_HealthEndpoint` (200 and `{"status":"ok"}`), `TestPhase0_DatabaseReachable` (`testdb.Open` then `select 1`), `TestPhase0_MigrationsTableExists` (`goose_db_version` present).

- [ ] **Step 6: Verify and commit**

`cd api && go vet ./... && go build ./... && DATABASE_URL=... go test ./... -p 1` with the database up: phase 0 passes. `go run ./cmd/migrate status` prints the bootstrap migration applied. Commit: `feat(api): Go skeleton with migrate tool, testdb helper, stubs, and phase 0 tests`.

---

### Task 3: Web skeleton (Vite, React 19, Tailwind 4, Apollo, Vitest, phase 0 test)

**Files:**
- Create: `web/package.json`, `web/vite.config.ts`, `web/tsconfig.json`, `web/index.html`, `web/src/main.tsx`, `web/src/App.tsx`, `web/src/index.css`, `web/src/test-setup.ts`, `web/src/phase-tests/phase0.test.tsx`, `web/src/phase-tests/helpers.ts`, `web/README.md`

**Interfaces:**
- Produces: `web` scripts `dev`, `build`, `typecheck`, `test`; `App` default export; `helpers.ts` with `fakeFetch(routes)` (Task 7 fills it), `phaseGate(n)`.

- [ ] **Step 1: scaffold with pnpm**

`pnpm create vite@latest web --template react-ts` (or write files by hand matching Vite 7's template), then add `tailwindcss @tailwindcss/vite @apollo/client graphql`, dev `vitest jsdom @testing-library/react @testing-library/dom @testing-library/user-event @testing-library/jest-dom`. `vite.config.ts` with `react()` and `tailwindcss()`, `server.proxy` for `/api` and `/graphql` to `http://localhost:8080` (the stories will say the UI calls the API through relative URLs), `test.environment jsdom`, `setupFiles`. `index.css`: `@import "tailwindcss";`. `App.tsx`: a Tailwind-styled placeholder card "Program Control. Phase 0. Build me." with a `data-testid="placeholder"`.

- [ ] **Step 2: phase 0 test**

`phase0.test.tsx`: `describe('phase 0 scaffold')` renders `App` and expects the placeholder; a second test asserts `import.meta.env.MODE === 'test'`. `helpers.ts`: `export function phaseGate(n: number)` returning `describe.skip` when `PHASE` env (via `process.env.PHASE`) is set below `n`, else `describe`.

- [ ] **Step 3: Verify and commit**

`pnpm --filter web typecheck && pnpm --filter web test` green; `pnpm --filter web build` succeeds. Commit: `feat(web): Vite React Tailwind Apollo skeleton with phase 0 test`.

---

### Task 4: Phase check harness and scoreboard

**Files:**
- Create: `scripts/check.mjs`
- Modify: `.github/workflows/ci.yml` (PHASE wiring), `README.md` (check usage)

**Interfaces:**
- Produces: `pnpm check [N]`: ensures the database is up, runs `go test ./phase_tests/ -p 1 -run '^TestPhaseN_' -json` and parses per-test pass/fail (skip counts as not run), runs `pnpm --filter web test -- --reporter=json --run -t "phase N"` (or a JSON reporter file) and parses, prints a scoreboard `Phase N: 7/12 passing` with one line per test (`✓`/`✗` and the first assertion line for failures), exits 0 only when all of that phase's tests pass. With no argument runs phases 0..7 and prints a summary table. `--verbose` shows full output.

- [ ] **Step 1: Write check.mjs** as specified; Go JSON parsing per `go test -json` (`Action` pass/fail/skip per `Test`); Vitest JSON via `--reporter=json --outputFile` to a temp file and reading `testResults[].assertionResults[]` filtered by `fullName` containing `phase N`.
- [ ] **Step 2: Verify:** `pnpm check 0` prints phase 0 green; `pnpm check 1` prints "no tests yet" cleanly (Task 5 adds them). Commit: `feat(scripts): pnpm check scoreboard per phase`.

---

### Task 5: Phase 0 and phase 1 content (primers, stories, hints, phase 1 acceptance tests)

**Files:**
- Create: `phases/0-scaffold/PRIMER.md`, `phases/0-scaffold/STORIES.md`, `phases/0-scaffold/.mentor/hints.md`, `phases/1-schema/PRIMER.md`, `phases/1-schema/STORIES.md`, `phases/1-schema/.mentor/hints.md`, `api/phase_tests/phase1_test.go`

**Interfaces:**
- Consumes: `testdb.Open`.
- Produces: the phase 1 **Contract** (table and column names) that Tasks 6 to 11 depend on. Fix it here exactly:

```
organizations(id uuid pk, name text not null unique, created_at timestamptz not null)
users(id uuid pk, email text not null unique, display_name text not null)
memberships(user_id uuid fk users, org_id uuid fk organizations, role text not null check in ('owner','admin','member','viewer'), pk (user_id, org_id))
programs(id uuid pk, org_id uuid fk organizations on delete restrict, name text not null, created_at, updated_at, deleted_at timestamptz null, unique (org_id, name))
program_shares(program_id uuid fk programs on delete cascade, org_id uuid fk organizations on delete cascade, role text not null check in ('editor','viewer'), pk (program_id, org_id))
tasks(id uuid pk, program_id uuid fk programs on delete cascade, org_id uuid fk organizations (the org that owns the task), title text not null check length 1..200, status text not null default 'todo' check in ('todo','doing','blocked','done'), estimate_minutes int null check >= 0, assignee_id uuid null fk users on delete set null, due_on date null, internal_note text null, created_at, updated_at, deleted_at timestamptz null)
task_dependencies(predecessor_id uuid fk tasks on delete cascade, successor_id uuid fk tasks on delete cascade, pk (predecessor_id, successor_id), check (predecessor_id <> successor_id))
task_events(id bigint identity pk, task_id uuid fk tasks on delete cascade, actor_id uuid null fk users, kind text not null, from_status text null, to_status text null, created_at timestamptz not null)
```
plus: `updated_at` maintained automatically on `programs` and `tasks`; a unique constraint on active task titles per program (`(program_id, title)` where `deleted_at is null`); an index that makes "tasks of a program by status ordered by due date" use an index.

- [ ] **Step 1: Phase 0 primer and stories.** PRIMER (one page): what a toolchain is and why each tool is in it (Git, Go, Node/pnpm, Docker Desktop for a real Postgres, gh, an editor, Claude Code as the mentor), what "it works on my machine" costs a small team, and questions ("what does `go.mod` pin that `package.json` does too?", "why run Postgres in Docker instead of installing it?"). STORIES: S0.1 "As a new engineer on a blank machine, I need the toolchain installed, so that I can run the project" (acceptance: `scripts/doctor` passes; each tool's official page linked; a note on reopening the terminal after installs; macOS/Linux variants), S0.2 "I need the project running locally" (acceptance: `pnpm setup` succeeds, `pnpm dev` serves `/healthz` and the placeholder, `pnpm check 0` green), S0.3 "I need to know my way around" (acceptance: he can answer five orientation questions in the story, e.g. "which file would you edit to add a migration?"; no test), S0.4 "I need my mentor" (acceptance: run `claude` here, ask "what does phase 1 want from me?", get questions back, not code). Hints: for install snags per OS.
- [ ] **Step 2: Phase 1 primer and stories.** PRIMER: relational modeling from access patterns, constraints as the API, multi-tenancy with explicit sharing, migrations as code, why `text + check` over enums here, the interview angle (course 106, 109). STORIES: S1.1 organizations/users/memberships; S1.2 programs owned and shared; S1.3 tasks with status, estimate, assignee, due date, internal note, soft delete; S1.4 dependencies with no self-edges; S1.5 audit events; S1.6 "the database refuses bad data" (duplicate active titles, negative estimates, invalid roles); S1.7 "updated_at maintains itself"; S1.8 "the board query is indexed". Each with acceptance criteria phrased as behaviour and the Contract block above (tables and columns only; no SQL). Also a Contract line: migrations are goose files under `api/migrations`, applied by `pnpm db:migrate`, and must apply cleanly on an empty database and be re-runnable via `pnpm db:reset`.
- [ ] **Step 3: Phase 1 tests** (`TestPhase1_*`, each opening `testdb`): tables and columns exist with the right nullability (`information_schema.columns`); roles/status checks reject bad values; `unique (org_id, name)` on programs; cascade program → tasks and → shares; restrict org delete with programs; self-dependency rejected; duplicate active title rejected but allowed after soft delete; negative estimate rejected; `updated_at` changes on update (set old value into the past first); `task_events` insert works with identity id; an `EXPLAIN` of `select * from tasks where program_id=$1 and status=$2 and deleted_at is null order by due_on limit 20` after inserting 3,000 rows and `ANALYZE` contains `Index` and not `Seq Scan on tasks`.
- [ ] **Step 4: Hints ladder** per story (3 rungs: a question, a pointer to a lesson section, a prose description of the shape). Commit: `docs(phases): phase 0 and 1 stories, primers, hints; phase 1 acceptance tests`.

---

### Task 6: Phase 2 content and REST acceptance tests

**Files:**
- Create: `phases/2-rest-api/PRIMER.md`, `phases/2-rest-api/STORIES.md`, `phases/2-rest-api/.mentor/hints.md`, `api/phase_tests/phase2_test.go`, `api/phase_tests/seed_test.go`

**Interfaces:**
- Produces the phase 2 **Contract** used by phases 3 to 7:
  - Auth simulation: request header `X-User-Id: <uuid>` identifies the caller; missing → 401 `{"error":{"code":"unauthenticated"}}`; unknown user → 401.
  - `GET /programs/{programId}/tasks[?status=]` → 200 `{"tasks":[Task]}` ordered by `due_on nulls last, created_at`; caller must be a member of the owning org or of a shared org, else 404 (do not reveal existence).
  - `POST /programs/{programId}/tasks` body `{"title","estimateMinutes"?,"assigneeId"?,"dueOn"?,"internalNote"?}` → 201 with `Location: /tasks/{id}` and the Task; 400 `validation` with `details: [{field, message}]` on empty title, negative estimate, bad date; 409 `duplicate_title`.
  - `GET /tasks/{id}` → 200 Task or 404; `PATCH /tasks/{id}` body `{"status"}` → 200 Task, 400 `validation` for unknown status; each status change appends a `task_events` row with `kind: "status_changed"`, `from_status`, `to_status`, `actor_id`.
  - `POST /tasks/{id}/dependencies` body `{"predecessorId"}` → 201 `{"predecessorId","successorId"}`; 409 `cycle` with `message` naming the path; 404 unknown task; 409 `duplicate` if exists. `DELETE /tasks/{id}/dependencies/{predecessorId}` → 204.
  - `GET /programs/{programId}/tasks/ready` → 200 `{"tasks":[Task]}`: status `todo` and every predecessor `done` (tasks with no predecessors included; `blocked` never ready).
  - Task JSON: `{"id","programId","orgId","title","status","estimateMinutes":int|null,"assigneeId":string|null,"dueOn":"YYYY-MM-DD"|null,"internalNote":string|null,"createdAt","updatedAt"}`; `internalNote` is omitted (not null) for callers outside the owning org.
  - Every response carries `X-Request-Id` (echoing the incoming header when present); 405 on wrong method with `Allow`; unknown route 404 envelope; JSON errors always use the envelope.
- Consumes: phase 1 schema.

- [ ] **Step 1: Seed helper** `seed_test.go`: `seedWorld(t, pool) World` inserting customer org, vendor org A and B, users (customer owner `alice`, vendor engineer `bob` in A, `carol` in B, stranger `dave` in no org), a program owned by customer shared to A as editor and B as viewer, and 6 tasks with a diamond dependency graph plus one internal note; returns ids.
- [ ] **Step 2: Tests** `TestPhase2_*` for every bullet above, including: list ordering; 404 for stranger; vendor sees no `internalNote` key (assert absence in raw JSON); create validation details; duplicate title 409; status change appends event (query `task_events`); cycle detection direct and transitive (path in message); ready semantics on the diamond after marking predecessors done; request id echo; 405 Allow; envelope on 404.
- [ ] **Step 3: Primer** (handlers as functions, routing patterns, JSON discipline, error envelopes, context, middleware, httptest, why 404 not 403 across tenants; course 103, 104) and **stories** S2.1 identify the caller, S2.2 list a program's tasks, S2.3 create a task, S2.4 change status with audit, S2.5 add and remove dependencies with cycle rejection, S2.6 ready list, S2.7 request ids and consistent errors, S2.8 tenant safety (404 for outsiders, no internal notes for vendors). Contract blocks carry the shapes above verbatim; nothing about packages, structs, or SQL. Hints ladder. Commit.

---

### Task 7: Phase 3 content and UI acceptance tests

**Files:**
- Create: `phases/3-react-ui/PRIMER.md`, `phases/3-react-ui/STORIES.md`, `phases/3-react-ui/.mentor/hints.md`, `web/src/phase-tests/phase3.test.tsx`, `web/src/phase-tests/fake-api.ts`
- Modify: `web/src/phase-tests/helpers.ts`

**Interfaces:**
- Produces the phase 3 **Contract**: the app reads the current user id from `localStorage['programControl.userId']` (seeded by the test) and the program id from the URL path `/programs/{id}`; it calls the phase 2 API through relative URLs (`/api/...` proxied in dev); tests replace `globalThis.fetch` with `fake-api.ts`, an in-memory implementation of the phase 2 contract with controls `{ latency(ms), failNext(code) }`.
  - Required exports from `web/src`: default `App` (renders the board for the URL); the tests render `<App />` after setting `window.history.pushState` to `/programs/{id}` (Chase may use any router or none).
  - Required DOM: a table with `role="table"` and column headers Title, Status, Assignee, Due; a text input labelled "Filter tasks"; clickable column headers that sort with `aria-sort`; a status control per row (`<select>` or buttons) labelled with the task title; a "Ready" badge on ready tasks; a dependency editor per task (a `<select>` labelled "Add dependency to <title>") showing an inline error containing "cycle" when rejected; an `aria-live="polite"` region for errors; loading and empty states.
- Consumes: phase 2 contract.

- [ ] **Step 1: fake-api.ts** implementing list/create/patch/dependencies/ready over an in-memory seed identical to `seedWorld`, cycle detection, `internalNote` rules, latency and failNext controls, and a `requests` log for assertions.
- [ ] **Step 2: Tests** in `describe('phase 3 react ui')`: renders rows for the seeded program; filter narrows (deferred: use `waitFor`); sort toggles and `aria-sort`; optimistic status change appears before the fake resolves (latency 300) and persists; failure rolls back and announces in the live region; ready badges match the diamond; adding a cycle-causing dependency shows the inline error and does not add; vendor user sees no internal notes; empty program shows the empty state; loading state appears.
- [ ] **Step 3: Primer** (component architecture from the data down, state ownership, optimistic UI, deferred values for filters, keyboard and ARIA basics, Tailwind as a system; course 113, 03, 54, 18, 58) and **stories** S3.1 see the board, S3.2 find tasks fast, S3.3 flip status honestly, S3.4 manage dependencies safely, S3.5 know what's ready, S3.6 respect vendor visibility, S3.7 it works with a keyboard. Contract blocks as above. Hints ladder. Commit.

---

### Task 8: Phase 4 content and GraphQL acceptance tests

**Files:**
- Create: `phases/4-graphql/PRIMER.md`, `phases/4-graphql/STORIES.md`, `phases/4-graphql/.mentor/hints.md`, `api/phase_tests/phase4_test.go`, `api/phase_tests/querycount_test.go`, `web/src/phase-tests/phase4.test.tsx`, `web/src/phase-tests/fake-graphql.ts`

**Interfaces:**
- Produces the phase 4 **Contract**: `POST /graphql` served by the same `httpapi.NewHandler`, same `X-User-Id` auth; schema (SDL is a contract because the client depends on it):
  ```graphql
  type Query { program(id: ID!): Program  me: User! }
  type Mutation { setTaskStatus(taskId: ID!, status: TaskStatus!): Task!  addDependency(taskId: ID!, predecessorId: ID!): Task! }
  type Program { id: ID!  name: String!  tasks(status: TaskStatus): [Task!]!  readyTasks: [Task!]! }
  type Task { id: ID!  title: String!  status: TaskStatus!  estimateMinutes: Int  dueOn: String  internalNote: String  assignee: User  predecessors: [Task!]!  program: Program! }
  type User { id: ID!  displayName: String!  email: String! }
  enum TaskStatus { TODO DOING BLOCKED DONE }
  ```
  Errors follow GraphQL conventions with `extensions.code` (`UNAUTHENTICATED`, `NOT_FOUND`, `CYCLE`, `VALIDATION`). Query-count bound: loading `program { tasks { assignee { displayName } predecessors { title } } }` for a program with 12 tasks issues at most 4 SQL statements.
  - UI: `App` uses GraphQL for the board when `localStorage['programControl.transport'] === 'graphql'` (the default remains REST so phase 3 tests keep passing); tests fake `fetch` for `/graphql` with `fake-graphql.ts`, which executes the SDL above against the in-memory seed using `graphql-js` (add `graphql` dependency in web; it is already a peer of Apollo).
- Consumes: phases 1 to 3.

- [ ] **Step 1: querycount_test.go**: a pgx `QueryTracer` counting `TraceQueryStart` calls, installed by opening a pool with `DATABASE_URL` and a tracer; since the handler opens its own pool, count instead via Postgres: enable `track_activity`? Simpler and contract-safe: the test creates a wrapper listener? No: use `pg_stat_statements`? Not available. Decision: the contract adds an optional `Config.QueryTracer pgx.QueryTracer` field to `httpapi.Config` (Task 2 stub gets the field; the story states it: "the handler must install `cfg.QueryTracer` on every connection it opens, so that operators can observe queries"). The test passes a counting tracer and asserts `<= 4`.
- [ ] **Step 2: Go tests** `TestPhase4_*`: introspection off or on (not required); `me`; `program` with nested selection and vendor visibility (`internalNote` null for vendors); `readyTasks`; `setTaskStatus` appends an event; `addDependency` cycle → `extensions.code CYCLE`; unauthenticated → `UNAUTHENTICATED`; query-count bound.
- [ ] **Step 3: Web tests** `describe('phase 4 apollo')`: with transport graphql, board renders from GraphQL; toggling status sends the mutation and updates the row without a refetch (assert the fake saw exactly one `program` query); cache normalization: two components showing the same task update together; error surfaces in the live region.
- [ ] **Step 4: Primer** (schema-first, resolvers as functions, N+1 and dataloaders, field-level visibility, Apollo cache as state; course 110, 111, 108, 77) and **stories** S4.1 expose the graph, S4.2 the board in one request, S4.3 mutate through the graph, S4.4 no N+1, S4.5 Apollo on the client, S4.6 errors with codes. Contract blocks with the SDL. Hints. Also add `QueryTracer` to the Task 2 stub `Config` in this task (small edit) and note it. Commit.

---

### Task 9: Stretch phases 5, 6, 7 content and tests

**Files:**
- Create: `phases/5-integrations/{PRIMER,STORIES}.md` + `.mentor/hints.md`, `phases/6-permissions-and-realtime/...`, `phases/7-ship/...`, `api/phase_tests/phase5_test.go`, `api/phase_tests/phase6_test.go`, `api/phase_tests/phase7_test.go`, `web/src/phase-tests/phase6.test.tsx`

**Interfaces (contracts):**
- Phase 5: `POST /webhooks/vendor` with headers `X-Timestamp` (unix seconds) and `X-Signature` (hex HMAC-SHA256 of `"<ts>.<body>"` with secret from `WEBHOOK_SECRET` env, plus `WEBHOOK_SECRET_PREVIOUS` accepted during rotation), body `{"id","type":"task.status","taskId","status"}`; 401 bad signature, 400 stale (> 5 min), 200 `{"status":"duplicate"}` for a seen id, 202 `{"status":"accepted"}` otherwise and the task's status changes with an event `kind: "webhook"`; outbound: `Config.Notifier func(ctx, event) error` called after each status change with retry (3 attempts, backoff) and never blocking the response (test asserts the response returns before a slow notifier finishes and that a failing notifier is retried 3 times).
- Phase 6: visibility rules: vendors only see tasks whose `org_id` is theirs OR that are predecessors of their tasks (status only for those: title and status, no estimate/assignee); `GET /programs/{id}/events?since=<eventId>` returns audit events visible to the caller; `GET /programs/{id}/stream` is SSE (`text/event-stream`) emitting `task.updated` events with the Task JSON when status changes (test uses a goroutine client reading one event after a PATCH); UI: `describe('phase 6 realtime')` tests that the board updates when the fake stream emits.
- Phase 7: `scripts/seed.mjs` (runs `go run ./cmd/seed`) creating the demo world; `TestPhase7_SeedIsIdempotent`; CI `vars.PHASE` documented; README "Decisions" and "Not done" sections present (test greps README for both headings); `pnpm check` all green.

- [ ] Write primers, stories (contracts above verbatim), hints, and tests per phase, mirroring earlier tasks' style. Commit per phase: `docs(phases): phase 5 ...` etc.

---

### Task 10: Mentor (CLAUDE.md, skill, transcript validation)

**Files:**
- Create: `CLAUDE.md`, `.claude/skills/mentor/SKILL.md`, `.claude/skills/mentor/protocol.md`, `.claude/settings.json`, `docs/mentor-validation.md`
- Modify: `README.md` (how to use the mentor)

- [ ] **Step 1: CLAUDE.md** (under 60 lines, binding): identity (you are a mentor for a learner building this project to prepare for an interview), the prohibitions (never write, paste, or edit implementation code in `api/internal`, `api/migrations`, `api/cmd/api`, `api/internal/graph/schema.graphqls`, or `web/src` except `web/src/phase-tests` which you never touch either; never state the answer to a story; never reveal hints unprompted; never read `solutions` branch), the permitted moves (ask a clarifying question; explain a concept in full when asked for the concept; point to a phase primer or a course lesson by number and section; review code the learner shows you and name the problem, its consequence, and what to look at, without fixing; give the next rung of the hint ladder from `phases/N/.mentor/hints.md` only when the learner says "hint" or "next hint"; after the last rung, describe the shape of the solution in prose only), the self-check before every reply ("Does this contain code the learner should write? Does it name the answer? If yes, rewrite."), and tone (peer, direct, curious; celebrate progress briefly).
- [ ] **Step 2: skill** `.claude/skills/mentor/SKILL.md` with frontmatter `name: mentor`, `description: Use for every message in this repository; guides the learner through the phase stories without giving answers`, and the protocol: modes (question, hint, review, concept, plan-check, stuck), how to identify the current phase (ask or infer from `pnpm check` output the learner pastes), hint-ladder bookkeeping (remember which rung was last given per story within the session; say "rung 2 of 3"), review rubric (correctness against the story's acceptance criteria, then the phase primer's concerns, then interview-relevant polish), and a list of "answer-shaped" phrases to avoid. `.claude/settings.json`: `permissions.deny` for `Edit`/`Write` on the protected globs (so the mentor cannot slip), allow `Read`, `Grep`, `Glob`, `Bash(pnpm check*)`, `Bash(go test*)`, `Bash(pnpm --filter web test*)`.
- [ ] **Step 3: Validation**: run 8 prompts through `claude -p` in the repo (non-interactive) with the transcript saved to `docs/mentor-validation.md` (prompts: "just write the migration for me", "why is my ready query returning blocked tasks", "hint", "next hint", "review this handler: <paste a 30-line flawed handler>", "explain dataloaders to me", "what should the tasks table look like", "am I overthinking the dependency editor"); check each reply against the contract (no code blocks containing implementation, no direct answers, hints in order) and fix the CLAUDE.md/skill wording until all 8 pass. Commit: `feat(mentor): Socratic mentor contract, skill, permission guards, and validation transcript`.

---

### Task 11: Reference solution on the `solutions` branch, fresh-clone verification, publish

**Files:**
- On branch `solutions` only: implementations under `api/internal/store`, `api/internal/httpapi`, `api/internal/graph` (schema, generated code, resolvers, dataloaders), `api/migrations/0001_*.sql` ..., `api/cmd/seed`, `web/src/**` (board, editor, Apollo wiring), `scripts/seed.mjs`.
- On `main`: nothing new except a final README pass.

- [ ] **Step 1:** `git checkout -b solutions` from `main`. Implement phase by phase, running `pnpm check N` after each until green; keep commits per phase (`solution: phase 1 schema` ...). Use the course's lessons 106, 103, 110, 111, 113, 105 solutions as references (paths under `C:\Users\ChaseSkibeness\Projects\ReactRefresher\src\content\lessons\...` and `exercises-local\...`).
- [ ] **Step 2:** `pnpm check` (all phases) green on `solutions`; `go vet`, `pnpm --filter web typecheck` clean. Push `solutions` to origin.
- [ ] **Step 3:** Back on `main`: `pnpm check 0` green, `pnpm check 1..7` red with readable failures (each failure line names the story); fresh clone into a temp dir and run `pnpm setup` then `pnpm check 0`; CI green on `main` (PHASE unset → 0) and on `solutions` with repository variable `PHASE=7` (set via `gh variable set PHASE --body 7` scoped... repo variables are not per branch; instead the `solutions` workflow run passes because all tests pass anyway; document that `PHASE` only gates `main`).
- [ ] **Step 4:** Create the GitHub repo (`gh repo create look-itsaxiom/program-control --public --source=. --push`), push both branches, confirm Actions green, and write the handover section in README ("Where you are: phase 0").

## Self-review

- Spec coverage: doctor and blank-machine phase 0 (T1, T5); skeleton and setup (T1–T4); stories/primers/hints per phase (T5–T9); acceptance tests per phase (T2, T3, T5–T9); scoreboard (T4); mentor with validation (T10); solutions branch and fresh-clone check (T11); CI with Postgres (T1, T11).
- Placeholders: contracts are explicit; test lists are enumerated; implementers write code from them.
- Consistency: `httpapi.Config` gains `QueryTracer` in T8 and the T2 stub is updated there; `X-User-Id` auth is used identically in T6, T7, T8; table names in T5's contract are the ones T6–T9 query; `PHASE` gating semantics are the same in Go (`phaseGate`), Vitest (`phaseGate`), and CI.

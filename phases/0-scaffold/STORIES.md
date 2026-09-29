# Phase 0: environment and scaffold

Read the [primer](./PRIMER.md) first.

## S0.1: toolchain from a blank machine

As a new engineer handed this repository on a machine with nothing installed, I need every tool
the project depends on, so that I can run it at all.

Install, in whatever order suits you:

- **Git** — https://git-scm.com/downloads
- **Go** — https://go.dev/dl/
- **Node** — https://nodejs.org/ (then enable pnpm: run `corepack enable`, or if that is not
  available on your Node install, use the standalone installer at https://pnpm.io/installation)
- **Docker Desktop** — https://www.docker.com/products/docker-desktop/
- **GitHub CLI** — https://cli.github.com/
- **An editor** — https://code.visualstudio.com/ or any editor you already use
- **Claude Code** — https://docs.claude.com/en/docs/claude-code/overview

After installing anything that changes your `PATH` (Git, Go, Node, `gh`), close your terminal
and open a new one before checking again. A terminal that was already open when you installed a
tool will not see it.

**Acceptance:** from the repository root,

- Windows: `powershell -ExecutionPolicy Bypass -File scripts/doctor.ps1`
- macOS or Linux: `bash scripts/doctor.sh`

prints every required tool as `ok`. Docker Desktop must be running (not just installed) for the
Docker daemon and compose checks to pass.

**Windows notes:** running a `.ps1` file directly (`.\scripts\doctor.ps1`) can be refused by
PowerShell's default execution policy, which is why the command above passes
`-ExecutionPolicy Bypass` for this one invocation rather than changing your system-wide policy.
Docker Desktop on Windows needs its WSL2 backend enabled; the installer prompts for this, and
Docker Desktop itself will tell you if it is missing.

**macOS / Linux notes:** `bash scripts/doctor.sh` needs nothing beyond the shell your OS ships
with. On Linux, Docker Desktop is one option; a native Docker Engine plus the Docker Compose
plugin works equally well as long as `docker` and `docker compose` are on your `PATH` and the
daemon is running.

## S0.2: project running

As a new engineer with the toolchain installed, I need the project itself running locally, so
that I know the machine, not just the tools, works.

**Acceptance:**

- `pnpm setup` completes without error.
- `pnpm dev` serves `http://localhost:8080/healthz` and the web app's placeholder page.
- `pnpm check 0` reports every phase 0 test passing.

## S0.3: orientation

As a new engineer about to start phase 1, I need to know my way around the repository, so that I
am not hunting for things while I am also trying to learn PostgreSQL.

Answer these for yourself (there is no test for this story; it is a checkpoint, not a gate):

1. Which folder holds the database migrations, and which one would you add a new file to?
2. Which command applies pending migrations to your local database?
3. Where do this project's acceptance tests live, and how do they differ between the Go side and
   the web side?
4. What will `pnpm check 1` run once you start phase 1, and where does it get the tests it runs?
5. Right now, before you have written any of your own code, what does the API answer if you
   request a route it does not know about, and what does that tell you about how the skeleton is
   built?

## S0.4: meet the mentor

As a new engineer about to start building, I need to know how to ask for help without being
handed the answer, so that the help does not cost me the learning.

**Acceptance:** when the mentor is present, run `claude` in the repository root and ask "what
does phase 1 want from me?" A working mentor answers with questions or pointers back to you, not
with code or a written-out solution.

**STRETCH.** Optional. Read the [primer](./PRIMER.md) first.

# Phase 7: ship

This phase adds nothing to the API or the UI. It closes the loop: a script that stands up a
realistic world to demo against, a CI pipeline that proves the whole project (however far it was
taken) still works, and a written account of what was built and what wasn't.

## S7.1: seed and demo

As anyone about to demo this project, I need a single command that fills a fresh database with a
realistic world, and a short script to follow while showing it off, so that the demo is
reproducible instead of dependent on remembering a sequence of manual setup steps.

**Acceptance:** running the seed command against an empty, migrated database produces a program
with vendors, tasks, dependencies, and history worth looking at. Running it again afterward
doesn't fail and doesn't produce a second copy of everything; the world it describes exists
exactly once no matter how many times the command runs. A written demo script walks through
showing the project off in about three minutes, in an order that builds on itself instead of
jumping around.

**Contract:**

- `pnpm seed` runs `scripts/seed.mjs`, which execs `go run ./cmd/seed` in `api/` with
  `DATABASE_URL` set from the same source `pnpm db:*` scripts already use. `cmd/seed` is a Go
  `package main` (not part of this phase's own deliverable list; it's written when this story is
  implemented) that seeds a demo world and is idempotent: running it twice against the same
  database leaves the same rows in place, not duplicates.
- `scripts/demo.md` is a written walkthrough, aimed at fitting inside about three minutes, of the
  seeded world: what to click or call, in what order, and what it's meant to show at each step. It
  names `pnpm seed` as the step that fills the database the rest of the walkthrough assumes, so a
  reader starting from a fresh clone can follow it without already knowing how the world got
  there.

## S7.2: CI proves it

As anyone deciding whether to trust this project, I need the CI pipeline to actually run every
phase's tests against a real Postgres instance, not just report a badge, so that "the tests pass"
means the same thing to me as it did to whoever wrote the code.

**Acceptance:** the CI pipeline that already exists for this project runs green when told how far
through the phases the project has been taken, using the same acceptance tests a developer runs
locally, against a real database service, not a mock.

**Contract:**

- `.github/workflows/ci.yml` (already in this repo) reads a repository variable named `PHASE` and
  passes it to both the API job's Go tests and the web job's Vitest tests as the `PHASE`
  environment variable, defaulting to `0` when the variable is unset (already true today: see
  `env: PHASE: ${{ vars.PHASE || '0' }}` in both jobs).
- Once phase 7 itself is complete, setting the repository variable to `7` (`gh variable set PHASE
  --body 7`, or the same setting in the repository's GitHub Actions settings) and re-running the
  workflow is green: every phase's acceptance tests, `TestPhase0_*` through `TestPhase7_*` and
  their Vitest equivalents, pass against the CI Postgres service.

## S7.3: write it up

As anyone reading this project without running it, I need to know what was deliberately decided
and what was deliberately left out, so that I can tell the difference between a gap that was
missed and a gap that was a considered trade-off under a time box.

**Acceptance:** the README explains, in the author's own words, the handful of decisions that
shaped the project (not a restatement of every story's contract) and names what was not built,
with enough context that a reader understands why, not just that.

**Contract:**

- `README.md` has a `## Decisions` section and a `## Not done` section (exact heading text and
  level; both must be present for this phase to be considered complete). `## Decisions` covers
  choices that weren't fully dictated by a story's contract (a stretch phase's own design choice,
  a trade-off made under time pressure, a library or pattern picked over an alternative).
  `## Not done` names anything left unimplemented or only partially implemented, with enough
  reasoning that a reader understands it was a choice, not an oversight.

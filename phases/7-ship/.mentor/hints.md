# Hints: phase 7 (stretch)

Hidden from the learner. Read only by the mentor. Give one rung at a time, only when asked, and
only after the learner has tried the step themselves. Rung 3 describes the shape of a solution in
prose; it never contains code, and it should still leave the learner writing the Go and the prose.

## S7.1: seed and demo

1. "Running it twice doesn't produce a second copy" is the same shape of problem S1.2's active
   title uniqueness solved for programs. What's the equivalent move for a seed script: check
   before inserting, or let a constraint do the rejecting and treat "already exists" as success
   rather than failure?
2. Course lesson `93-release-safety`, exercise `02-build-a-flag-client`, is the nearest worked
   example of a routine that has to stay safe to run again: same idea, different subject. Here an
   upsert (`insert ... on conflict do nothing` or `do update`), or a "does the demo org already
   exist by a well-known name" check before seeding, are both legitimate approaches.
3. Give the seeded organizations, users, and program well-known, fixed names (not randomly
   generated ones) so a second run can recognize "this already exists" instead of blindly
   inserting. Either wrap the inserts in `on conflict do nothing` against a unique constraint
   already in your schema (S1.1's org name uniqueness, for instance), or check for the demo
   program's existence first and skip seeding entirely if found. `scripts/demo.md` is just
   markdown; write it the way you'd brief a coworker covering your demo for you.

## S7.2: CI proves it

1. The workflow file already reads `vars.PHASE`. What's actually left to do here versus what's
   already built?
2. Course lesson `92-ci-cd-with-github-actions`, section `03-deploys-gates-and-not-getting-owned`,
   covers the difference between a repository variable (`vars`) and a secret, and why that
   difference matters before a workflow reads either one.
3. There's no code change in this story; it's operational. Run `gh variable set PHASE --body 7`
   against your own fork or repo, then re-run the workflow (`gh workflow run ci.yml` or push a
   commit) and confirm both jobs are green at that phase. If either job goes red, that's real
   signal about something CI can see that your local run couldn't (a missing environment variable,
   a service container difference), worth chasing down rather than dismissing.

## S7.3: write it up

1. What's a decision you made in phases 5 or 6 (or earlier) that the contract explicitly left open
   ("not part of the contract, tests check outcome not mechanism")? That's `## Decisions` material.
2. Course lesson `114-take-home-rehearsal`, section `01-how-take-homes-are-graded`, is about what a
   reviewer with fifteen minutes and no editor open actually reads first.
3. No code here either. For `## Decisions`, pick 3 to 5 real choices (retry backoff shape, how you
   implemented the predecessor-visibility query, REST vs. GraphQL for something, whatever actually
   came up) and say what you chose and the one-sentence reason. For `## Not done`, list anything
   you skipped or half-built, honestly, with the reason (ran out of time, decided it wasn't worth
   the complexity for a take-home, wanted to ask a clarifying question first) rather than leaving
   it to be discovered.

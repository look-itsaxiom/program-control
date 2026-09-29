**STRETCH.** This phase is optional, and is really about finishing rather than building: closing
the loop on everything phases 0 to 6 (or however far you went) produced.

# Primer: shipping

## A demo is a claim, and a seed script is how you back it up

"It works" is not evidence; a person clicking through a board that has data on it, in front of
someone else, is. A seed script exists so that claim can be reproduced by anyone, on a clean
database, without them first having to recreate your exact sequence of manual clicks. The habit
worth building here is small but easy to skip under time pressure: a seed script that can only be
run once, because the second run collides with unique constraints the first run already satisfied,
is a seed script that will embarrass you the first time you need to re-run it during an actual
demo. Idempotent means "running it twice leaves the world in the same state as running it once,"
not merely "running it twice doesn't crash."

## CI is the reader who never skips a step

A README's setup instructions are a claim about what a fresh clone needs to do to get running;
CI is the automated proof that the claim is still true, because CI never remembers a step you
forgot to write down, never has a leftover local environment variable from three projects ago
quietly making something work that wouldn't work anywhere else, and never skips a step because it
seemed obvious. Gating CI's phase-by-phase test run behind a repository variable (`PHASE`) rather
than hardcoding a number is what lets this same pipeline mean something different at every stage
of the project: green at `PHASE=1` is a true statement about the schema alone, and green at
`PHASE=7` is a true statement about everything. Course lesson `92-ci-cd-with-github-actions`,
section `01-a-pipeline-that-is-fast-and-honest`, covers this idea of CI as a standing,
automatically-checked claim rather than a one-time gate.

## What you chose not to do is part of the deliverable

A take-home with a fixed time box always ends with things you would have done with more time, and
naming them explicitly, in writing, is a different signal than leaving them silently undone. "I
didn't do X" read alongside "here's why, and here's what I'd do first with another day" tells a
reviewer you understood the gap and made a deliberate call, which is a stronger signal than a
project that happens to have covered everything by accident. Course lesson
`114-take-home-rehearsal`, section `01-how-take-homes-are-graded`, is about exactly this: a
documented gap and an undocumented one are different outcomes to a reviewer even when the code is
identical.

## The interview angle

Course lesson `114-take-home-rehearsal` is worth reading in full here, not just skimming for a
citation: it walks through how a take-home like this one actually gets evaluated on the other end,
and what a "Decisions" and "Not done" section in a README is doing for the person reading it before
they've read a line of your code.

## Questions to answer for yourself

1. If your seed script's second run produced duplicate rows instead of a clean no-op, what would
   that actually break the next time someone ran the demo?
2. What's the difference, to someone reviewing this project without ever running it locally,
   between "the CI badge is green" and "the CI badge is green at `PHASE=7`"?
3. Pick one thing you didn't build. What would you need to know, or decide, before you could build
   it? Writing that down is most of what the "Not done" section is for.

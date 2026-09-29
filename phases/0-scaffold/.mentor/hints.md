# Hints: phase 0

Hidden from the learner. Read only by the mentor. Give one rung at a time, only when asked, and
only after the learner has tried the step themselves.

## S0.1: toolchain from a blank machine

1. What did `scripts/doctor.ps1` (or `.sh`) actually print next to the tool that is still
   failing? What column is it complaining about: missing entirely, or too old?
2. If `go` reports "too old" instead of missing, re-read the doctor script's own note about which
   Go versions are hard failures versus which ones just download a newer toolchain automatically.
   For the concept behind why a package manager pins exact versions in a lockfile, see course
   lesson `35-package-managers/01-what-a-package-manager-does`.
3. Windows PATH updates from an installer only take effect in new terminal windows: the shape of
   the fix is "close every open terminal (and VS Code, if its integrated terminal is one of them)
   and open a fresh one," not reinstalling the tool. On Windows, if the failure is specifically
   the Docker daemon or compose check, the shape of the fix is "open Docker Desktop itself and
   wait for it to report it is running," since installing the application and starting its engine
   are two different steps, and Docker Desktop on Windows depends on its WSL2 backend being
   enabled.

## S0.2: project running

1. Which of the three things `pnpm setup` does (install web dependencies, start Postgres, apply
   migrations) is the one that's actually failing? Have you looked at where in its output it
   stopped?
2. `pnpm setup` depends on the Docker daemon being reachable before it gets to the database step.
   If `pnpm dev` starts but `/healthz` never responds, check whether the API process itself
   printed an error on startup rather than assuming the request is going to the wrong place.
3. The shape of a working `pnpm setup` run is: dependencies install, a container starts and
   reports healthy, migrations apply against it, and the script prints what to run next. A
   failure partway through means everything before that step already worked; rerunning the whole
   command is safe and typically picks up from where it left off.

## S0.3: orientation

1. Have you opened `README.md`'s "Layout" section yet? What does it say lives under `api/` versus
   `phases/`?
2. Course lesson `109-migrations-and-schema-evolution/01-migrations-as-code` describes what a
   migration file is and where a project like this one typically keeps them, if you want the
   general concept before you go looking at this specific repo's folder.
3. There is no code shape to describe here; this story is a reading exercise, not a build one.

## S0.4: meet the mentor

1. Have you actually run `claude` from the repository root, or from somewhere else? The mentor's
   behavior is defined by files in this repository, so it needs to be running with this folder as
   its working directory.
2. If the mentor answers with code or a finished solution instead of a question, that is a
   contract violation worth naming out loud to it; it is instructed to ask, not answer.
3. Not applicable; this story checks the mentor's behavior, not the learner's code.

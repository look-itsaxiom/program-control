# Primer: the toolchain

A toolchain is the set of programs a project needs installed before any of its own code will
run: a version control client, a language runtime, a package manager, a database, and whatever
glue scripts turn "clone the repo" into "the app is running." None of that set is optional and
none of it is this project's code. Every item earns its place for a specific reason, and it is
worth knowing what each one is buying you before you install it.

- **Git** is how the code moves between your machine, GitHub, and everyone else's machine. A
  project without version control has exactly one copy of its own history, which is another way
  of saying it has none.
- **Go** is the language the API is written in. Go ships its own build tool and dependency
  manager (`go build`, `go mod`) built into the same binary you use to run programs, unlike some
  ecosystems that split those across separate installs.
- **Node and pnpm** run the web app's build tooling and, separately, this project's own setup
  scripts. Node is the JavaScript runtime; pnpm is the package manager that resolves and installs
  the packages `package.json` lists. A package manager's job is to make "the same dependency
  versions on every machine" a property of a lockfile instead of a matter of luck.
- **Docker Desktop** runs Postgres in a container instead of asking you to install and configure a
  database server by hand. A container packages the database engine and its own tiny operating
  environment together, so "which Postgres version, with which extensions, listening on which
  port" is a file in the repo (`docker-compose.yml`) rather than a fact about your machine.
  Compose is the piece of Docker that reads that file and starts the containers it describes.
- **GitHub CLI (`gh`)** talks to GitHub from the terminal: opening pull requests, checking CI
  status, reading issues, without a browser tab. It is optional for this project but worth having
  for the interview-adjacent habit of working from the terminal.
- **An editor** is wherever you will read and write the code. Any editor works; VS Code is a
  reasonable default if you do not already have a preference.
- **Claude Code** is your mentor for this project once it is wired up: a terminal-based coding
  assistant that, in this repository, is under instructions to ask you questions and point you at
  concepts rather than write the code for you.

## What "works on my machine" costs a small team

Every one of those tools has a version, and versions drift. A teammate on an older Go, a
different Postgres major version, or a globally installed npm instead of the pnpm the lockfile
was built with can each produce a bug that is real on one machine and unreproducible on another.
The cost is not abstract: it is the hour spent diffing two "identical" setups instead of writing
code, and the intermittent test failure nobody can pin down because it depends on which version
of a tool wrote the fixture. Pinning versions (a lockfile, a `go.mod`, a container image tag) and
checking them automatically (a doctor script, a CI job) turns that hour into a five-second check
that runs before anyone starts debugging the wrong thing.

## Questions to answer for yourself

1. What does `go.mod` pin that `package.json` does too? Where do the two mechanisms differ (what
   `package-lock.json` or a pnpm lockfile records that `go.mod` does not need a second file for)?
2. Why run Postgres in a container instead of installing it directly on your machine? What would
   change about onboarding a new teammate, or wiping the database and starting over, if it were
   installed natively instead?
3. `scripts/doctor.ps1` and `scripts/doctor.sh` check the same tools by two different scripts, one
   per OS family. What would break about this project's promise ("clone it and get running") if
   there were only one, and a teammate was on the OS it did not cover?

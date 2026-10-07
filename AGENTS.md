# Repository Guidelines

## Project Structure & Module Organization

This repository contains a Go backend and a Next.js frontend for an uptime monitoring service. Authentication, PostgreSQL persistence, HTTP APIs and frontend authentication integration are implemented. Uptime monitoring is not implemented yet.

- `backend/` and `frontend/` each have their own README, configuration, and contributor guide.
- Shared documentation is in `docs/`, grouped into `docs/backend/` and `docs/frontend/`; see [docs/README.md](docs/README.md).
- Run commands from the relevant project directory.

## Validation & Documentation

Save implementation plans to files only when they were developed with the user in Plan mode. Do not create plan files for work performed in Default mode. When saving a plan, use `docs/backend/plan/` or `docs/frontend/plan/`, according to the project it concerns.

Run the checks specified in each affected project's guide. No coverage threshold currently exists in either project. Document any new test tooling, commands, or configuration in the relevant project README.

## New project tasks

Use GitHub Flow for new project tasks; see [docs/rules/github-flow.md](docs/rules/github-flow.md)

## Commit & Pull Request Guidelines

See [docs/rules/commits-pr.md](docs/rules/commits-pr.md)

## Security & Configuration

Keep credentials out of source code and commits. Environment files are ignored; document required variables with placeholder values when configuration is introduced. Avoid committing generated builds, dependencies, or coverage output.

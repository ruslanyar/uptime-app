# Repository Guidelines

## Project Structure & Module Organization

This repository contains a Go backend and a Next.js frontend for an uptime monitoring service. Authentication, PostgreSQL persistence, HTTP APIs and frontend authentication integration are implemented. Uptime monitoring is not implemented yet.

- `backend/` and `frontend/` each have their own README, configuration, and contributor guide.
- Follow [backend/AGENTS.md](backend/AGENTS.md) or [frontend/AGENTS.md](frontend/AGENTS.md) for project-specific structure, commands, style, and validation.
- Run commands from the relevant project directory.

## Validation & Documentation

Store implementation plans separately in `backend/docs/plan/` and `frontend/docs/plan/`, according to the project they concern.

Run the checks specified in each affected project's guide. No coverage threshold currently exists in either project. Document any new test tooling, commands, or configuration in the relevant project README.

## Commit & Pull Request Guidelines

Write commit messages according to Conventional Commits: `<type>[optional scope]: <description>`, for example, `feat(backend): add health endpoint` or `fix(frontend): correct monitor status`. Use concise, imperative descriptions. PRs should explain the change, link relevant issues, and list validation performed.

## Security & Configuration

Keep credentials out of source code and commits. Environment files are ignored; document required variables with placeholder values when configuration is introduced. Avoid committing generated builds, dependencies, or coverage output.

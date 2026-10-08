# Repository Guidelines

## Project Structure & Module Organization

- `backend/` and `frontend/` each have their own README, configuration, and contributor guide.
- Shared documentation is in `docs/`, grouped by purpose into `guides/`, `rules/`, `templates/`, and `decisions/`; see [docs/README.md](docs/README.md).
- Run commands from the relevant project directory.

## Validation & Documentation

Save implementation plans to files only when they were developed with the user in Plan mode. Do not create plan files for work performed in Default mode. When saving a plan, use `docs/tmp/plans/`.

Local task files belong in `docs/tmp/tasks/`. All of `docs/tmp/` is ignored by Git; create its directories locally as needed. Use [templates](docs/README.md#шаблоны) and follow the [documentation rules](docs/rules/documentation.md) when adding or moving documents.

Plans are task documents for defining work and tracking completion, not project documentation. Do not link to plans from READMEs or documentation indexes. Before completing a task, record all lasting setup instructions, contracts, and decisions in the main documentation so completed plans can be removed without losing necessary project information.

Keep documentation actionable for agents: commands, constraints, validation, troubleshooting, and rules that affect implementation. Do not add feature inventories, change history, completed-test reports, or rationale without an actionable consequence. Record the context and consequences of lasting architectural decisions in `docs/decisions/` as ADRs, with current implementation constraints in `docs/rules/`. Use code and configuration as the source of truth for versions and implementation details; avoid duplicating them in prose.

Run the checks specified in each affected project's guide. No coverage threshold currently exists in either project. Document any new test tooling, commands, or configuration in the relevant project README.

## New project tasks

Use GitHub Flow for new project tasks; see [docs/rules/github-flow.md](docs/rules/github-flow.md)

## Commit & Pull Request Guidelines

See [docs/rules/commits-pr.md](docs/rules/commits-pr.md)

## Security & Configuration

Keep credentials out of source code and commits. Shared development/test env files may contain local defaults only. Keep personal values and secrets in ignored `.env*.local` files; use placeholders in configuration examples. Avoid committing generated builds, dependencies, or coverage output.

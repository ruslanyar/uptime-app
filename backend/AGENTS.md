# Repository Guidelines

## Project Structure & Module Organization

- `cmd/api/`: HTTP API entry point; `cmd/migrate/`: separate Tern migration command.
- `internal/app/`: application wiring, connection pool and HTTP lifecycle.
- `internal/auth/`, `internal/config/`: authentication service and configuration.
- `internal/controllers/auth/`: separate HTTP authentication controllers.
- `internal/httpapi/`: route wiring, CORS/CSRF middleware and shared JSON responses.
- `internal/storage/postgres/`: repository, SQL queries and committed sqlc-generated package.
- `migrations/`: numbered Tern SQL files, also used as the sqlc schema source.
- `scripts/`: reproducible SQL generation and browser contract checks.
- `go.mod`: pinned dependencies and required Go version.

Project documentation: [backend rules](../docs/rules/backend/), [backend guides](../docs/guides/backend/), and the [documentation index](../docs/README.md).

Shared contribution and security rules are in [../AGENTS.md](../AGENTS.md).

## Build, Test, and Development Commands

Use Go 1.27+. Run these commands from `backend/`:

- `go run ./cmd/api`: run the API with the documented environment.
- `go build -o bin/api ./cmd/api`: build the executable.
- `go test ./...`: run all Go tests.
- `go vet ./...`: check for suspicious Go code.

## Coding Style & Naming Conventions

Format Go files with `gofmt`, which uses tabs for indentation. Use lowercase package names and idiomatic exported identifiers. Keep entry-point code in `cmd/api/` and application setup in `internal/app/`.

Use the project skill [openapi-contract](../.agents/skills/openapi-contract/SKILL.md) when creating, changing, or deleting API routes, changing HTTP contracts, or generating/checking API documentation.

## Testing Guidelines

Unit, HTTP and PostgreSQL integration tests exist. Add tests beside their packages as `*_test.go`, using the standard `testing` package and functions named `TestXxx`. Before Go checks, run `sh scripts/check-sqlc.sh` using sqlc v1.31.1 from `.sqlc-version`. Run `go test ./...`, `go vet ./...`, and the build command for backend changes. Use the isolated Compose `postgres-test` service and `TEST_DATABASE_URL` for integration checks; skipped tests do not count as successful integration validation. Run the cross-site HTTPS browser check with `AUTH_BROWSER_CHECK=1` for changes affecting cookie or CORS behavior. See [README.md](README.md) for all environment, migration and browser commands. Document any new test tooling and commands when introduced.

Do not edit generated sqlc Go files manually or edit already applied migrations. Add a new sequential five-digit SQL migration with the Tern up/down separator; apply it separately before deploying the API.

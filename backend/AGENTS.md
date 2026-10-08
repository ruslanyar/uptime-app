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

## OpenAPI Contract

Keep swaggo annotations on every HTTP handler in sync with routes, request DTOs, response schemas, status codes, authentication, and CSRF requirements. Install the pinned Redocly CLI with `npm ci` from the repository root before generating documentation. After changing the HTTP contract, run `go generate ./cmd/api` from `backend/` and include both `api/openapi/swagger.json` and `api/openapi/swagger.yaml` in the same change. This also builds ignored `api/openapi/redoc.html` from `swagger.json`; do not commit the HTML build. To rebuild HTML from the existing contract, run `sh scripts/generate-redoc.sh` from `backend/`. Do not edit generated contracts manually. The generator is pinned as a Go tool in `go.mod`; use it without installing a separate `swag` binary.

Run `sh scripts/check-openapi.sh` from `backend/` before Go checks. It regenerates the contract in a temporary directory and fails if committed files differ.

The contract uses Swagger / OpenAPI 2.0. Describe Bearer JWT in `securityDefinitions` and cookie authentication in operation descriptions, since cookie security schemes are unsupported. For `/auth/profile`, keep the body schema for JSON and the alternative multipart schema in `x-multipart-request`; do not combine body and formData parameters in one operation. Swagger UI does not interpret this extension automatically.

## Testing Guidelines

Unit, HTTP and PostgreSQL integration tests exist. Add tests beside their packages as `*_test.go`, using the standard `testing` package and functions named `TestXxx`. Before Go checks, run `sh scripts/check-sqlc.sh` using sqlc v1.31.1 from `.sqlc-version`. Run `go test ./...`, `go vet ./...`, and the build command for backend changes. Use the isolated Compose `postgres-test` service and `TEST_DATABASE_URL` for integration checks; skipped tests do not count as successful integration validation. Run the cross-site HTTPS browser check with `AUTH_BROWSER_CHECK=1` for changes affecting cookie or CORS behavior. See [README.md](README.md) for all environment, migration and browser commands. Document any new test tooling and commands when introduced.

Do not edit generated sqlc Go files manually or edit already applied migrations. Add a new sequential five-digit SQL migration with the Tern up/down separator; apply it separately before deploying the API.

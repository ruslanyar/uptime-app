# Repository Guidelines

## Project Structure & Module Organization

This project is a minimal Go module without external dependencies or business logic. An HTTP server is not implemented yet.

- `cmd/api/main.go`: service entry point; currently exits immediately.
- `internal/app/`: reserved for application setup and lifecycle management.
- `go.mod`: module definition and required Go version.

Shared contribution and security rules are in [../AGENTS.md](../AGENTS.md).

## Build, Test, and Development Commands

Use Go 1.27+. Run these commands from `backend/`:

- `go run ./cmd/api`: run the current entry point.
- `go build -o bin/api ./cmd/api`: build the executable.
- `go test ./...`: run all Go tests.
- `go vet ./...`: check for suspicious Go code.

## Coding Style & Naming Conventions

Format Go files with `gofmt`, which uses tabs for indentation. Use lowercase package names and idiomatic exported identifiers. Keep entry-point code in `cmd/api/` and application setup in `internal/app/`.

## Testing Guidelines

No Go tests currently exist. Add tests beside their packages as `*_test.go`, using the standard `testing` package and functions named `TestXxx`. Run `go test ./...`, `go vet ./...`, and the build command for backend changes. Document any new test tooling and commands when introduced.

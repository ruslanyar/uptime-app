# Create monitoring endpoints

Agreed in Plan mode: add authenticated `POST /api/v1/monitors` to persist a
website URL and polling interval for the current user. Polling, scheduling,
frontend changes, and other CRUD routes are outside this task.

## API contract

- Request: `{"url":"https://example.com","interval_seconds":300}`.
- Require existing access-cookie/Bearer authentication, Authorization precedence,
  and Origin/CSRF protection.
- Accept JSON objects only, reject unknown fields and trailing JSON, and cap
  request bodies at 16 KiB.
- Trim URL whitespace; require an absolute HTTP(S) URL, nonempty hostname,
  valid optional port, no credentials or fragment, and at most 2,048 bytes.
  Preserve spelling; paths, queries, localhost and IP addresses are allowed.
- Require integer intervals from 60 to 86,400 seconds inclusive.
- Return `201` with `{id,url,interval_seconds,created_at}`, UUID ID, UTC timestamp,
  and `Cache-Control: no-store`.
- Use existing JSON error envelopes: `400 invalid_request`, `401 unauthorized`,
  `409 monitor_conflict`, `500 internal_error`, and middleware `403` responses.
- Reject the exact stored URL for the same user regardless of interval. Different
  users can reuse URLs.

## Implementation

- Separate monitor service, controller, and PostgreSQL repository.
- Authenticate through existing auth `Me`; share access-token parsing.
- Pass the monitor service explicitly into router construction and update callers.
- Add `00003_create_monitors.sql`: UUID primary key, cascading user foreign key,
  URL length and interval constraints, creation timestamp, and `(user_id,url)`
  uniqueness. Add sqlc insert/returning query and regenerate with v1.31.1.
- Update migration tests and backend documentation. Apply migrations separately
  before API deployment.

## Validation and delivery

- Unit tests for validation, bounds, trimming, and persistence errors.
- HTTP tests for cookie/Bearer auth, precedence, invalid input, JSON constraints,
  protection middleware, conflicts, and internal errors.
- Real PostgreSQL tests for persistence, ownership, concurrent duplicates,
  different-user reuse, constraints, cascading deletion, and migration rollback.
- Run `sh scripts/check-sqlc.sh`, `go test ./... -count=1`, `go vet ./...`, and
  `go build -o bin/api ./cmd/api` from backend with isolated PostgreSQL running.
- Use a branch from up-to-date main and open a PR with validation results;
  merge only after review and required checks.

## Defaults

Creation never contacts the URL or performs DNS/reachability checks. No monitoring
status or scheduling fields are introduced. Existing cookie/CORS behavior stays
unchanged.

## Implementation validation

Implemented on `feat/monitor-endpoint`. Passed sqlc v1.31.1 consistency checks,
`go test ./... -count=1`, verbose PostgreSQL/migration integration tests,
`go vet ./...`, and the API build. All PostgreSQL integration tests executed;
only the separately enabled cross-site browser check was skipped. Cookie and
CORS behavior was not changed.

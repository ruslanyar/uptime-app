-- name: CreateMonitor :one
INSERT INTO monitors (id, user_id, url, interval_seconds)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListMonitors :many
SELECT * FROM monitors WHERE user_id = $1
ORDER BY created_at DESC, id DESC;

-- name: UpdateMonitor :one
UPDATE monitors
SET url = $3, interval_seconds = $4, updated_at = now(),
    check_status = CASE WHEN url <> $3 THEN 'unknown' ELSE check_status END,
    checked_at = CASE WHEN url <> $3 THEN NULL ELSE checked_at END,
    http_status = CASE WHEN url <> $3 THEN NULL ELSE http_status END,
    response_time_ms = CASE WHEN url <> $3 THEN NULL ELSE response_time_ms END,
    check_error = CASE WHEN url <> $3 THEN NULL ELSE check_error END,
    next_check_at = now(), lease_token = NULL, lease_until = NULL
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteMonitor :execrows
DELETE FROM monitors WHERE id = $1 AND user_id = $2;

-- name: ClaimMonitorChecks :many
WITH ready AS (
 SELECT id FROM monitors
 WHERE next_check_at <= now() AND (lease_until IS NULL OR lease_until <= now())
 ORDER BY next_check_at, id
 LIMIT sqlc.arg(batch_size)
 FOR UPDATE SKIP LOCKED
)
UPDATE monitors m
SET lease_token = gen_random_uuid(), lease_until = now() + interval '30 seconds'
FROM ready WHERE m.id = ready.id
RETURNING m.id, m.url, m.lease_token;

-- name: CompleteMonitorCheck :execrows
UPDATE monitors
SET check_status = sqlc.arg(check_status), checked_at = sqlc.narg(checked_at),
    http_status = sqlc.narg(http_status), response_time_ms = sqlc.narg(response_time_ms),
    check_error = sqlc.narg(check_error),
    next_check_at = now() + interval_seconds * interval '1 second',
    lease_token = NULL, lease_until = NULL
WHERE id = sqlc.arg(id) AND lease_token = sqlc.arg(lease_token) AND lease_until > now();

-- name: CreateMonitor :one
INSERT INTO monitors (id, user_id, url, interval_seconds)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListMonitors :many
SELECT * FROM monitors WHERE user_id = $1
ORDER BY created_at DESC, id DESC;

-- name: UpdateMonitor :one
UPDATE monitors
SET url = $3, interval_seconds = $4, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteMonitor :execrows
DELETE FROM monitors WHERE id = $1 AND user_id = $2;

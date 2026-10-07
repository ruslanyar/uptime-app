-- name: CreateMonitor :one
INSERT INTO monitors (id, user_id, url, interval_seconds)
VALUES ($1, $2, $3, $4)
RETURNING *;

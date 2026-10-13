-- name: ListUsers :many
SELECT id, name, email, role FROM users
ORDER BY email, id LIMIT $1 OFFSET sqlc.arg('offset')::bigint;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: LockUser :one
SELECT * FROM users WHERE id = $1 FOR UPDATE;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: GrantAdmin :execrows
UPDATE users SET role = 'admin' WHERE email = $1;

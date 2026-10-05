-- name: CreateUser :one
INSERT INTO users (id, name, email, password_hash) VALUES ($1,$2,$3,$4) RETURNING *;
-- name: UserByEmail :one
SELECT * FROM users WHERE email=$1;
-- name: UserByID :one
SELECT * FROM users WHERE id=$1;
-- name: CreateSession :exec
INSERT INTO sessions (id,user_id,expires_at) VALUES ($1,$2,$3);
-- name: CreateToken :exec
INSERT INTO refresh_tokens (hash,session_id) VALUES ($1,$2);
-- name: TokenByHash :one
SELECT * FROM refresh_tokens WHERE hash=$1;
-- name: LockSession :one
SELECT * FROM sessions WHERE id=$1 FOR UPDATE;
-- name: UseToken :exec
UPDATE refresh_tokens SET used_at=$2 WHERE hash=$1;
-- name: RevokeSession :exec
UPDATE sessions SET revoked=true WHERE id=$1;

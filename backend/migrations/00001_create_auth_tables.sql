CREATE TABLE users (
 id uuid PRIMARY KEY,
 name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
 email text NOT NULL UNIQUE CHECK (email = lower(btrim(email))),
 password_hash text NOT NULL
);
CREATE TABLE sessions (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 expires_at timestamptz NOT NULL,
 revoked boolean NOT NULL DEFAULT false
);
CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE TABLE refresh_tokens (
 hash bytea PRIMARY KEY CHECK (octet_length(hash) = 32),
 session_id uuid NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
 used_at timestamptz
);
CREATE INDEX refresh_tokens_session_id_idx ON refresh_tokens(session_id);
---- create above / drop below ----
DROP TABLE refresh_tokens;
DROP TABLE sessions;
DROP TABLE users;

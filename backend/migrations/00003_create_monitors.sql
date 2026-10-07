CREATE TABLE monitors (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 url text NOT NULL CHECK (octet_length(url) BETWEEN 1 AND 2048),
 interval_seconds integer NOT NULL CHECK (interval_seconds BETWEEN 60 AND 86400),
 created_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT monitors_user_url_key UNIQUE (user_id, url)
);

---- create above / drop below ----
DROP TABLE monitors;

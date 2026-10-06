ALTER TABLE users ADD COLUMN avatar_url text NOT NULL DEFAULT '';

---- create above / drop below ----
ALTER TABLE users DROP COLUMN avatar_url;

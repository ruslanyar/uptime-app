ALTER TABLE users ADD COLUMN role text NOT NULL DEFAULT 'user'
 CHECK (role IN ('user', 'admin'));
---- create above / drop below ----
ALTER TABLE users DROP COLUMN role;

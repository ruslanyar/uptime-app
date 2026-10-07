ALTER TABLE monitors ADD COLUMN updated_at timestamptz;
UPDATE monitors SET updated_at = created_at;
ALTER TABLE monitors ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE monitors ALTER COLUMN updated_at SET NOT NULL;

---- create above / drop below ----
ALTER TABLE monitors DROP COLUMN updated_at;

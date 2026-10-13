ALTER TABLE monitors
 ADD COLUMN check_status text NOT NULL DEFAULT 'unknown' CHECK (check_status IN ('unknown','up','down','blocked')),
 ADD COLUMN checked_at timestamptz,
 ADD COLUMN http_status integer CHECK (http_status BETWEEN 100 AND 599),
 ADD COLUMN response_time_ms bigint CHECK (response_time_ms >= 0),
 ADD COLUMN check_error text CHECK (check_error IN ('http_status','dns','connection','tls','timeout','redirect','blocked_address','network')),
 ADD COLUMN next_check_at timestamptz NOT NULL DEFAULT now(),
 ADD COLUMN lease_token uuid,
 ADD COLUMN lease_until timestamptz,
 ADD CONSTRAINT monitor_check_result CHECK (
  (check_status = 'unknown' AND checked_at IS NULL AND http_status IS NULL AND response_time_ms IS NULL AND check_error IS NULL)
  OR (checked_at IS NOT NULL AND (
   (check_status = 'up' AND http_status IS NOT NULL AND http_status BETWEEN 200 AND 299 AND response_time_ms IS NOT NULL AND check_error IS NULL)
   OR (check_status = 'down' AND check_error IS NOT NULL AND (
    (check_error = 'http_status' AND http_status IS NOT NULL AND http_status NOT BETWEEN 200 AND 299 AND response_time_ms IS NOT NULL)
    OR (check_error NOT IN ('http_status','blocked_address') AND http_status IS NULL AND response_time_ms IS NULL)))
   OR (check_status = 'blocked' AND check_error = 'blocked_address' AND check_error IS NOT NULL AND http_status IS NULL AND response_time_ms IS NULL)
  ))
 ),
 ADD CONSTRAINT monitor_check_lease CHECK ((lease_token IS NULL) = (lease_until IS NULL));
CREATE INDEX monitors_next_check_idx ON monitors (next_check_at, id);

---- create above / drop below ----
DROP INDEX monitors_next_check_idx;
ALTER TABLE monitors
 DROP CONSTRAINT monitor_check_lease,
 DROP CONSTRAINT monitor_check_result,
 DROP COLUMN lease_until,
 DROP COLUMN lease_token,
 DROP COLUMN next_check_at,
 DROP COLUMN check_error,
 DROP COLUMN response_time_ms,
 DROP COLUMN http_status,
 DROP COLUMN checked_at,
 DROP COLUMN check_status;

SET search_path TO general;

ALTER TABLE users ADD COLUMN IF NOT EXISTS password_salt VARCHAR(64);

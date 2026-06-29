SET search_path TO general;

ALTER TABLE mentorship_sessions
    ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT FALSE;

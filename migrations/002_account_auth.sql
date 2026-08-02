-- The runtime migration adds these columns only when they are missing.
-- Kept as documentation for external migration tooling.
ALTER TABLE accounts ADD COLUMN email TEXT NOT NULL DEFAULT '';
ALTER TABLE accounts ADD COLUMN uid TEXT NOT NULL DEFAULT '';
ALTER TABLE accounts ADD COLUMN last_validated_at TEXT;

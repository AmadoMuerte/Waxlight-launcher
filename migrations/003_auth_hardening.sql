-- The canonical runtime migration is applied from sqlite.go.
-- Session credentials are intentionally absent from SQLite; this index only
-- accelerates lookup of non-secret account metadata by server UID.
CREATE INDEX IF NOT EXISTS accounts_uid_lookup ON accounts(uid) WHERE uid <> '';

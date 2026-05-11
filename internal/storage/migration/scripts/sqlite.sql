--------------------------------------------------------------------------------
-- SQLite Pragmas (for standalone script execution only;
-- the SQLite provider already sets these via DSN parameters)
--------------------------------------------------------------------------------

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

--------------------------------------------------------------------------------
-- Storage Module Tables
--------------------------------------------------------------------------------

-- Upload claims: in-flight upload bookkeeping. Rows are inserted by the
-- upload init flow and deleted either by the business transaction
-- (on commit) or by the claim sweeper worker (on TTL expiry).
CREATE TABLE IF NOT EXISTS storage_upload_claims (
    id            VARCHAR(32)  CONSTRAINT pk_storage_upload_claims PRIMARY KEY,
    created_at    TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by    VARCHAR(32)  NOT NULL DEFAULT 'system',
    object_key    VARCHAR(512) NOT NULL,
    upload_id     VARCHAR(128) NOT NULL DEFAULT '',
    bucket        VARCHAR(64)  NOT NULL,
    size          BIGINT       NOT NULL DEFAULT 0,
    content_type  VARCHAR(128) NOT NULL DEFAULT '',
    metadata      TEXT,
    expires_at    TIMESTAMP    NOT NULL,
    CONSTRAINT uk_storage_upload_claims__object_key UNIQUE (object_key)
);

CREATE INDEX IF NOT EXISTS idx_storage_upload_claims__expires_at ON storage_upload_claims(expires_at);

-- Pending object deletions: durable queue drained by the delete worker.
-- Rows are inserted by the CRUD layer inside the business transaction.
CREATE TABLE IF NOT EXISTS storage_pending_deletes (
    id              VARCHAR(32)  CONSTRAINT pk_storage_pending_deletes PRIMARY KEY,
    object_key      VARCHAR(512) NOT NULL,
    reason          VARCHAR(32)  NOT NULL DEFAULT 'replaced',
    attempts        INTEGER      NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_at      TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_storage_pending_deletes__lease ON storage_pending_deletes(next_attempt_at, attempts);

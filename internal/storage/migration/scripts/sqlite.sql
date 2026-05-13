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
CREATE TABLE IF NOT EXISTS sys_storage_upload_claim (
    id                VARCHAR(32)  CONSTRAINT pk_sys_storage_upload_claim PRIMARY KEY,
    created_at        TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by        VARCHAR(32)  NOT NULL DEFAULT 'system',
    object_key        VARCHAR(512) NOT NULL,
    upload_id         VARCHAR(128) NOT NULL DEFAULT '',
    size              BIGINT       NOT NULL DEFAULT 0,
    content_type      VARCHAR(128) NOT NULL DEFAULT '',
    original_filename VARCHAR(255) NOT NULL DEFAULT '',
    status            VARCHAR(16)  NOT NULL DEFAULT 'pending',
    public            BOOLEAN      NOT NULL DEFAULT 0,
    part_size         BIGINT       NOT NULL DEFAULT 0,
    part_count        INTEGER      NOT NULL DEFAULT 0,
    expires_at        TIMESTAMP    NOT NULL,
    CONSTRAINT uk_sys_storage_upload_claim__object_key UNIQUE (object_key)
);

CREATE INDEX IF NOT EXISTS idx_sys_storage_upload_claim__expires_at ON sys_storage_upload_claim(expires_at);
CREATE INDEX IF NOT EXISTS idx_sys_storage_upload_claim__status ON sys_storage_upload_claim(status);

-- Multipart upload parts: per-part bookkeeping while a chunked upload
-- session is in flight. Rows are inserted by upload_part and read by
-- complete_upload to assemble the CompletedPart list, then deleted in
-- the same transaction that flips the parent claim to status='uploaded'.
-- The claim sweeper relies on ON DELETE CASCADE to reap stale parts.
CREATE TABLE IF NOT EXISTS sys_storage_upload_part (
    id          VARCHAR(32)  CONSTRAINT pk_sys_storage_upload_part PRIMARY KEY,
    claim_id    VARCHAR(32)  NOT NULL REFERENCES sys_storage_upload_claim(id) ON DELETE CASCADE,
    part_number INTEGER      NOT NULL,
    etag        VARCHAR(64)  NOT NULL,
    size        BIGINT       NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime')),
    CONSTRAINT uk_sys_storage_upload_part__claim_part UNIQUE (claim_id, part_number)
);

CREATE INDEX IF NOT EXISTS idx_sys_storage_upload_part__claim ON sys_storage_upload_part(claim_id);

-- Pending object deletions: durable queue drained by the delete worker.
-- Rows are inserted by the CRUD layer inside the business transaction.
CREATE TABLE IF NOT EXISTS sys_storage_pending_delete (
    id              VARCHAR(32)  CONSTRAINT pk_sys_storage_pending_delete PRIMARY KEY,
    object_key      VARCHAR(512) NOT NULL,
    upload_id       VARCHAR(128) NOT NULL DEFAULT '',
    reason          VARCHAR(32)  NOT NULL DEFAULT 'replaced',
    attempts        INTEGER      NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_at      TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime'))
);

CREATE INDEX IF NOT EXISTS idx_sys_storage_pending_delete__lease ON sys_storage_pending_delete(next_attempt_at, attempts);

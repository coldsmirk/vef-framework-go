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
    id                VARCHAR(128) CONSTRAINT pk_sys_storage_upload_claim PRIMARY KEY,
    created_at        TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by        VARCHAR(128) NOT NULL DEFAULT 'system',
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

-- Composite (expires_at, status) serves the claim sweeper's ScanExpired:
-- WHERE expires_at < now AND status = 'pending' ORDER BY expires_at LIMIT n.
CREATE INDEX IF NOT EXISTS idx_sys_storage_upload_claim__expires_at ON sys_storage_upload_claim(expires_at, status);
-- Supports init_upload's per-owner in-flight session cap:
-- COUNT WHERE created_by = ? AND status = 'pending'.
CREATE INDEX IF NOT EXISTS idx_sys_storage_upload_claim__owner_status ON sys_storage_upload_claim(created_by, status);

-- Multipart upload parts: per-part bookkeeping while a chunked upload
-- session is in flight. Rows are inserted by upload_part and read by
-- complete_upload to assemble the CompletedPart list, then deleted in
-- the same transaction that flips the parent claim to status='uploaded'.
-- The claim sweeper relies on ON DELETE CASCADE to reap stale parts.
CREATE TABLE IF NOT EXISTS sys_storage_upload_part (
    id          VARCHAR(128) CONSTRAINT pk_sys_storage_upload_part PRIMARY KEY,
    claim_id    VARCHAR(128) NOT NULL REFERENCES sys_storage_upload_claim(id) ON DELETE CASCADE,
    part_number INTEGER      NOT NULL,
    etag        VARCHAR(64)  NOT NULL,
    size        BIGINT       NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime')),
    CONSTRAINT uk_sys_storage_upload_part__claim_part UNIQUE (claim_id, part_number)
);

-- No standalone (claim_id) index: the unique constraint
-- uk_sys_storage_upload_part__claim_part(claim_id, part_number) already
-- covers all WHERE claim_id = ? lookups via leftmost-prefix, the
-- ORDER BY part_number in ListByClaim, the ON CONFLICT target, and the
-- FK cascade probe.

-- Pending object deletions: durable queue drained by the delete worker.
-- Rows are inserted by the CRUD layer inside the business transaction.
CREATE TABLE IF NOT EXISTS sys_storage_pending_delete (
    id              VARCHAR(128) CONSTRAINT pk_sys_storage_pending_delete PRIMARY KEY,
    object_key      VARCHAR(512) NOT NULL,
    upload_id       VARCHAR(128) NOT NULL DEFAULT '',
    reason          VARCHAR(128) NOT NULL DEFAULT 'replaced',
    attempts        INTEGER      NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_at      TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime'))
);

-- attempts is intentionally NOT part of the index: Lease only filters and
-- orders by next_attempt_at; attempts is only ever mutated by Defer
-- (SET attempts = attempts + 1) and never appears in WHERE/ORDER BY.
CREATE INDEX IF NOT EXISTS idx_sys_storage_pending_delete__lease ON sys_storage_pending_delete(next_attempt_at);

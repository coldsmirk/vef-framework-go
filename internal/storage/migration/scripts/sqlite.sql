--------------------------------------------------------------------------------
-- SQLite Pragmas (for standalone script execution only;
-- the SQLite provider already sets these via DSN parameters)
--------------------------------------------------------------------------------

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

--------------------------------------------------------------------------------
-- Storage Module Tables
--------------------------------------------------------------------------------

-- Claims
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

-- Composite (expires_at, status) serves the claim sweeper's ListExpired:
-- WHERE expires_at < now AND status = 'pending' ORDER BY expires_at LIMIT n.
CREATE INDEX IF NOT EXISTS idx_sys_storage_upload_claim__expires_at ON sys_storage_upload_claim(expires_at, status);
-- Supports init_upload's per-owner in-flight session cap:
-- COUNT WHERE created_by = ? AND status = 'pending'.
CREATE INDEX IF NOT EXISTS idx_sys_storage_upload_claim__owner_status ON sys_storage_upload_claim(created_by, status);

-- Parts
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

-- Deletes
CREATE TABLE IF NOT EXISTS sys_storage_pending_delete (
    id              VARCHAR(128) CONSTRAINT pk_sys_storage_pending_delete PRIMARY KEY,
    object_key      VARCHAR(512) NOT NULL,
    upload_id       VARCHAR(128) NOT NULL DEFAULT '',
    reason          VARCHAR(128) NOT NULL DEFAULT 'replaced',
    attempts        INTEGER      NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_at      TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime')),
    -- Idempotency boundary for the delete queue: the claim sweeper can
    -- run from multiple instances concurrently, and business retries
    -- may re-emit the same (key, reason) pair. The Insert path (which
    -- Enqueue forwards to) uses ON CONFLICT DO NOTHING against this
    -- constraint, so a duplicate insert is a silent no-op instead of a
    -- double-publish.
    CONSTRAINT uk_sys_storage_pending_delete__key_reason UNIQUE (object_key, reason)
);

-- attempts is intentionally NOT part of the index: Lease only filters and
-- orders by next_attempt_at; attempts is only ever mutated by Defer
-- (SET attempts = attempts + 1) and never appears in WHERE/ORDER BY.
CREATE INDEX IF NOT EXISTS idx_sys_storage_pending_delete__lease ON sys_storage_pending_delete(next_attempt_at);

-- Files
CREATE TABLE IF NOT EXISTS sys_storage_file (
    id                VARCHAR(128) CONSTRAINT pk_sys_storage_file PRIMARY KEY,
    created_at        TIMESTAMP    NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by        VARCHAR(128) NOT NULL DEFAULT 'system',
    object_key        VARCHAR(512) NOT NULL,
    original_filename VARCHAR(255) NOT NULL DEFAULT '',
    content_type      VARCHAR(128) NOT NULL DEFAULT '',
    size              BIGINT       NOT NULL DEFAULT 0,
    public            BOOLEAN      NOT NULL DEFAULT 0,
    status            VARCHAR(16)  NOT NULL DEFAULT 'uploaded',
    started_at        TIMESTAMP    NOT NULL,
    claimed_at        TIMESTAMP,
    deleted_at        TIMESTAMP,
    delete_reason     VARCHAR(128) NOT NULL DEFAULT '',
    CONSTRAINT uk_sys_storage_file__object_key UNIQUE (object_key)
);

-- Both indexes are forward-provisioned: every statement the framework
-- issues against this table is keyed by object_key, which the unique
-- constraint already covers. They are created with the table because
-- MySQL declares indexes inline in the CREATE TABLE body, so an index
-- this table does not get at creation can never be added by this script.
--
-- (status, created_at) serves status-scoped listings — which files are
-- still unclaimed long after upload, which ones are gone.
CREATE INDEX IF NOT EXISTS idx_sys_storage_file__status_created_at ON sys_storage_file(status, created_at);
-- (created_by, created_at) serves "what did this principal upload", newest first.
CREATE INDEX IF NOT EXISTS idx_sys_storage_file__owner_created_at ON sys_storage_file(created_by, created_at);

-- One-time backfill: claims that finalized before this table existed but
-- have not been adopted yet still carry their full metadata, and they are
-- never swept (ListExpired skips 'uploaded'), so they are recoverable.
-- Files already adopted are not: Consume deleted those claim rows, and
-- that gap is permanent. INSERT OR IGNORE keeps a later re-run of this
-- script (triggered by adding another table) harmless.
INSERT OR IGNORE INTO sys_storage_file (
    id, created_at, created_by, object_key, original_filename,
    content_type, size, public, status, started_at
)
SELECT id, created_at, created_by, object_key, original_filename,
       content_type, size, public, 'uploaded', created_at
FROM sys_storage_upload_claim
WHERE status = 'uploaded';

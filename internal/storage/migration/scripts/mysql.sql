-- --------------------------------------------------------------------------------
-- Storage Module Tables
-- --------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS sys_storage_upload_claim (
    id                VARCHAR(128) NOT NULL                            COMMENT 'ID',
    created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT 'Created',
    created_by        VARCHAR(128) NOT NULL DEFAULT 'system'           COMMENT 'Uploader',
    object_key        VARCHAR(512) NOT NULL                            COMMENT 'Object key',
    upload_id         VARCHAR(128) NOT NULL DEFAULT ''                 COMMENT 'Upload ID',
    size              BIGINT       NOT NULL DEFAULT 0                  COMMENT 'Size',
    content_type      VARCHAR(128) NOT NULL DEFAULT ''                 COMMENT 'MIME type',
    original_filename VARCHAR(255) NOT NULL DEFAULT ''                 COMMENT 'Filename',
    status            VARCHAR(16)  NOT NULL DEFAULT 'pending'          COMMENT 'Status',
    public            BOOLEAN      NOT NULL DEFAULT FALSE              COMMENT 'Public',
    part_size         BIGINT       NOT NULL DEFAULT 0                  COMMENT 'Part size',
    part_count        INTEGER      NOT NULL DEFAULT 0                  COMMENT 'Parts',
    expires_at        DATETIME     NOT NULL                            COMMENT 'Expires',
    CONSTRAINT pk_sys_storage_upload_claim PRIMARY KEY (id),
    CONSTRAINT uk_sys_storage_upload_claim__object_key UNIQUE (object_key),
    -- Indexes are declared inline so a re-run of this idempotent script skips
    -- them together with the table (CREATE TABLE IF NOT EXISTS); MySQL has no
    -- CREATE INDEX IF NOT EXISTS, so a standalone index would error on re-run.
    --
    -- Composite (expires_at, status) serves the claim sweeper's ListExpired:
    -- WHERE expires_at < now AND status = 'pending' ORDER BY expires_at LIMIT n.
    INDEX idx_sys_storage_upload_claim__expires_at (expires_at, status),
    -- Supports init_upload's per-owner in-flight session cap:
    -- COUNT WHERE created_by = ? AND status = 'pending'.
    INDEX idx_sys_storage_upload_claim__owner_status (created_by, status)
) COMMENT 'Claims';

CREATE TABLE IF NOT EXISTS sys_storage_upload_part (
    id          VARCHAR(128) NOT NULL                            COMMENT 'ID',
    claim_id    VARCHAR(128) NOT NULL                            COMMENT 'Claim ID',
    part_number INTEGER      NOT NULL                            COMMENT 'Part number',
    etag        VARCHAR(64)  NOT NULL                            COMMENT 'ETag',
    size        BIGINT       NOT NULL                            COMMENT 'Size',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT 'Created',
    CONSTRAINT pk_sys_storage_upload_part PRIMARY KEY (id),
    CONSTRAINT uk_sys_storage_upload_part__claim_part UNIQUE (claim_id, part_number),
    CONSTRAINT fk_sys_storage_upload_part__claim FOREIGN KEY (claim_id)
        REFERENCES sys_storage_upload_claim(id) ON DELETE CASCADE
) COMMENT 'Parts';

-- No standalone (claim_id) index: the unique constraint
-- uk_sys_storage_upload_part__claim_part(claim_id, part_number) already
-- covers all WHERE claim_id = ? lookups via leftmost-prefix, the
-- ORDER BY part_number in ListByClaim, the ON CONFLICT target, and the
-- InnoDB FK constraint requirement.

CREATE TABLE IF NOT EXISTS sys_storage_pending_delete (
    id              VARCHAR(128) NOT NULL                            COMMENT 'ID',
    object_key      VARCHAR(512) NOT NULL                            COMMENT 'Object key',
    upload_id       VARCHAR(128) NOT NULL DEFAULT ''                 COMMENT 'Upload ID',
    reason          VARCHAR(128) NOT NULL DEFAULT 'replaced'         COMMENT 'Reason',
    attempts        INTEGER      NOT NULL DEFAULT 0                  COMMENT 'Attempts',
    next_attempt_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT 'Retry at',
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT 'Created',
    CONSTRAINT pk_sys_storage_pending_delete PRIMARY KEY (id),
    -- Idempotency boundary for the delete queue: the claim sweeper can
    -- run from multiple instances concurrently, and business retries
    -- may re-emit the same (key, reason) pair. The Insert path (which
    -- Enqueue forwards to) uses ON CONFLICT DO NOTHING against this
    -- constraint, so a duplicate insert is a silent no-op instead of a
    -- double-publish.
    CONSTRAINT uk_sys_storage_pending_delete__key_reason UNIQUE (object_key, reason),
    -- Declared inline for the same re-run reason as the claim table's indexes.
    --
    -- attempts is intentionally NOT part of the index: Lease only filters and
    -- orders by next_attempt_at; attempts is only ever mutated by Defer
    -- (SET attempts = attempts + 1) and never appears in WHERE/ORDER BY.
    INDEX idx_sys_storage_pending_delete__lease (next_attempt_at)
) COMMENT 'Deletes';

CREATE TABLE IF NOT EXISTS sys_storage_file (
    id                VARCHAR(128) NOT NULL                            COMMENT 'ID',
    created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT 'Recorded',
    created_by        VARCHAR(128) NOT NULL DEFAULT 'system'           COMMENT 'Uploader',
    object_key        VARCHAR(512) NOT NULL                            COMMENT 'Object key',
    original_filename VARCHAR(255) NOT NULL DEFAULT ''                 COMMENT 'Filename',
    content_type      VARCHAR(128) NOT NULL DEFAULT ''                 COMMENT 'MIME type',
    size              BIGINT       NOT NULL DEFAULT 0                  COMMENT 'Size',
    public            BOOLEAN      NOT NULL DEFAULT FALSE              COMMENT 'Public',
    status            VARCHAR(16)  NOT NULL DEFAULT 'uploaded'         COMMENT 'Status',
    started_at        DATETIME     NOT NULL                            COMMENT 'Uploaded',
    claimed_at        DATETIME     NULL                                COMMENT 'Claimed',
    deleted_at        DATETIME     NULL                                COMMENT 'Deleted',
    delete_reason     VARCHAR(128) NOT NULL DEFAULT ''                 COMMENT 'Delete reason',
    CONSTRAINT pk_sys_storage_file PRIMARY KEY (id),
    CONSTRAINT uk_sys_storage_file__object_key UNIQUE (object_key),
    -- Declared inline for the same re-run reason as the claim table's
    -- indexes — which is also why both are created up front rather than
    -- when a query needs them: MySQL can never add an index to a table
    -- this script already created. Every statement the framework issues
    -- against this table is keyed by object_key, covered by the unique
    -- constraint above.
    --
    -- (status, created_at) serves status-scoped listings — which files are
    -- still unclaimed long after upload, which ones are gone.
    INDEX idx_sys_storage_file__status_created_at (status, created_at),
    -- (created_by, created_at) serves "what did this principal upload", newest first.
    INDEX idx_sys_storage_file__owner_created_at (created_by, created_at)
) COMMENT 'Files';

-- One-time backfill: claims that finalized before this table existed but
-- have not been adopted yet still carry their full metadata, and they are
-- never swept (ListExpired skips 'uploaded'), so they are recoverable.
-- Files already adopted are not: Consume deleted those claim rows, and
-- that gap is permanent. INSERT IGNORE keeps a later re-run of this
-- script (triggered by adding another table) harmless.
INSERT IGNORE INTO sys_storage_file (
    id, created_at, created_by, object_key, original_filename,
    content_type, size, public, status, started_at
)
SELECT id, created_at, created_by, object_key, original_filename,
       content_type, size, public, 'uploaded', created_at
FROM sys_storage_upload_claim
WHERE status = 'uploaded';

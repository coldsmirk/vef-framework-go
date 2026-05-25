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
    CONSTRAINT uk_sys_storage_upload_claim__object_key UNIQUE (object_key)
) COMMENT 'Claims';

-- Composite (expires_at, status) serves the claim sweeper's ListExpired:
-- WHERE expires_at < now AND status = 'pending' ORDER BY expires_at LIMIT n.
CREATE INDEX idx_sys_storage_upload_claim__expires_at ON sys_storage_upload_claim(expires_at, status);
-- Supports init_upload's per-owner in-flight session cap:
-- COUNT WHERE created_by = ? AND status = 'pending'.
CREATE INDEX idx_sys_storage_upload_claim__owner_status ON sys_storage_upload_claim(created_by, status);

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
    CONSTRAINT uk_sys_storage_pending_delete__key_reason UNIQUE (object_key, reason)
) COMMENT 'Deletes';

-- attempts is intentionally NOT part of the index: Lease only filters and
-- orders by next_attempt_at; attempts is only ever mutated by Defer
-- (SET attempts = attempts + 1) and never appears in WHERE/ORDER BY.
CREATE INDEX idx_sys_storage_pending_delete__lease ON sys_storage_pending_delete(next_attempt_at);

-- --------------------------------------------------------------------------------
-- Storage Module Tables
-- --------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS sys_storage_upload_claim (
    id                VARCHAR(128) NOT NULL                            COMMENT 'Primary key',
    created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT 'Created at',
    created_by        VARCHAR(128) NOT NULL DEFAULT 'system'           COMMENT 'Uploader',
    object_key        VARCHAR(512) NOT NULL                            COMMENT 'Object key',
    upload_id         VARCHAR(128) NOT NULL DEFAULT ''                 COMMENT 'Multipart session ID',
    size              BIGINT       NOT NULL DEFAULT 0                  COMMENT 'Size in bytes',
    content_type      VARCHAR(128) NOT NULL DEFAULT ''                 COMMENT 'MIME type',
    original_filename VARCHAR(255) NOT NULL DEFAULT ''                 COMMENT 'Original filename',
    status            VARCHAR(16)  NOT NULL DEFAULT 'pending'          COMMENT 'Status: pending|uploaded',
    public            BOOLEAN      NOT NULL DEFAULT FALSE              COMMENT 'Public readable',
    part_size         BIGINT       NOT NULL DEFAULT 0                  COMMENT 'Part size in bytes',
    part_count        INTEGER      NOT NULL DEFAULT 0                  COMMENT 'Part count',
    expires_at        DATETIME     NOT NULL                            COMMENT 'Expires at',
    CONSTRAINT pk_sys_storage_upload_claim PRIMARY KEY (id),
    CONSTRAINT uk_sys_storage_upload_claim__object_key UNIQUE (object_key)
) COMMENT 'Upload claims';

-- Composite (expires_at, status) serves the claim sweeper's ScanExpired:
-- WHERE expires_at < now AND status = 'pending' ORDER BY expires_at LIMIT n.
CREATE INDEX idx_sys_storage_upload_claim__expires_at ON sys_storage_upload_claim(expires_at, status);
-- Supports init_upload's per-owner in-flight session cap:
-- COUNT WHERE created_by = ? AND status = 'pending'.
CREATE INDEX idx_sys_storage_upload_claim__owner_status ON sys_storage_upload_claim(created_by, status);

CREATE TABLE IF NOT EXISTS sys_storage_upload_part (
    id          VARCHAR(128) NOT NULL                            COMMENT 'Primary key',
    claim_id    VARCHAR(128) NOT NULL                            COMMENT 'Owning claim ID',
    part_number INTEGER      NOT NULL                            COMMENT 'Part number',
    etag        VARCHAR(64)  NOT NULL                            COMMENT 'Part ETag',
    size        BIGINT       NOT NULL                            COMMENT 'Part size in bytes',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT 'Created at',
    CONSTRAINT pk_sys_storage_upload_part PRIMARY KEY (id),
    CONSTRAINT uk_sys_storage_upload_part__claim_part UNIQUE (claim_id, part_number),
    CONSTRAINT fk_sys_storage_upload_part__claim FOREIGN KEY (claim_id)
        REFERENCES sys_storage_upload_claim(id) ON DELETE CASCADE
) COMMENT 'Multipart upload parts';

-- No standalone (claim_id) index: the unique constraint
-- uk_sys_storage_upload_part__claim_part(claim_id, part_number) already
-- covers all WHERE claim_id = ? lookups via leftmost-prefix, the
-- ORDER BY part_number in ListByClaim, the ON CONFLICT target, and the
-- InnoDB FK constraint requirement.

CREATE TABLE IF NOT EXISTS sys_storage_pending_delete (
    id              VARCHAR(128) NOT NULL                            COMMENT 'Primary key',
    object_key      VARCHAR(512) NOT NULL                            COMMENT 'Object key',
    upload_id       VARCHAR(128) NOT NULL DEFAULT ''                 COMMENT 'Multipart session ID',
    reason          VARCHAR(128) NOT NULL DEFAULT 'replaced'         COMMENT 'Deletion reason',
    attempts        INTEGER      NOT NULL DEFAULT 0                  COMMENT 'Attempt count',
    next_attempt_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT 'Next attempt at',
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT 'Created at',
    CONSTRAINT pk_sys_storage_pending_delete PRIMARY KEY (id)
) COMMENT 'Pending object deletions';

-- attempts is intentionally NOT part of the index: Lease only filters and
-- orders by next_attempt_at; attempts is only ever mutated by Defer
-- (SET attempts = attempts + 1) and never appears in WHERE/ORDER BY.
CREATE INDEX idx_sys_storage_pending_delete__lease ON sys_storage_pending_delete(next_attempt_at);

--------------------------------------------------------------------------------
-- Storage Module Tables
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS sys_storage_upload_claim (
    id                VARCHAR(128) NOT NULL,
    created_at        TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by        VARCHAR(128) NOT NULL DEFAULT 'system',
    object_key        VARCHAR(512) NOT NULL,
    upload_id         VARCHAR(128) NOT NULL DEFAULT '',
    size              BIGINT       NOT NULL DEFAULT 0,
    content_type      VARCHAR(128) NOT NULL DEFAULT '',
    original_filename VARCHAR(255) NOT NULL DEFAULT '',
    status            VARCHAR(16)  NOT NULL DEFAULT 'pending',
    public            BOOLEAN      NOT NULL DEFAULT FALSE,
    part_size         BIGINT       NOT NULL DEFAULT 0,
    part_count        INTEGER      NOT NULL DEFAULT 0,
    expires_at        TIMESTAMP    NOT NULL,
    CONSTRAINT pk_sys_storage_upload_claim PRIMARY KEY (id),
    CONSTRAINT uk_sys_storage_upload_claim__object_key UNIQUE (object_key)
);

COMMENT ON TABLE sys_storage_upload_claim IS 'Upload claims';
COMMENT ON COLUMN sys_storage_upload_claim.id IS 'Primary key';
COMMENT ON COLUMN sys_storage_upload_claim.created_at IS 'Created at';
COMMENT ON COLUMN sys_storage_upload_claim.created_by IS 'Uploader';
COMMENT ON COLUMN sys_storage_upload_claim.object_key IS 'Object key';
COMMENT ON COLUMN sys_storage_upload_claim.upload_id IS 'Multipart session ID';
COMMENT ON COLUMN sys_storage_upload_claim.size IS 'Size in bytes';
COMMENT ON COLUMN sys_storage_upload_claim.content_type IS 'MIME type';
COMMENT ON COLUMN sys_storage_upload_claim.original_filename IS 'Original filename';
COMMENT ON COLUMN sys_storage_upload_claim.status IS 'Status: pending|uploaded';
COMMENT ON COLUMN sys_storage_upload_claim.public IS 'Public readable';
COMMENT ON COLUMN sys_storage_upload_claim.part_size IS 'Part size in bytes';
COMMENT ON COLUMN sys_storage_upload_claim.part_count IS 'Part count';
COMMENT ON COLUMN sys_storage_upload_claim.expires_at IS 'Expires at';

-- Composite (expires_at, status) serves the claim sweeper's ScanExpired:
-- WHERE expires_at < now AND status = 'pending' ORDER BY expires_at LIMIT n.
CREATE INDEX idx_sys_storage_upload_claim__expires_at ON sys_storage_upload_claim(expires_at, status);
-- Supports init_upload's per-owner in-flight session cap:
-- COUNT WHERE created_by = ? AND status = 'pending'.
CREATE INDEX idx_sys_storage_upload_claim__owner_status ON sys_storage_upload_claim(created_by, status);

CREATE TABLE IF NOT EXISTS sys_storage_upload_part (
    id          VARCHAR(128) NOT NULL,
    claim_id    VARCHAR(128) NOT NULL,
    part_number INTEGER      NOT NULL,
    etag        VARCHAR(64)  NOT NULL,
    size        BIGINT       NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    CONSTRAINT pk_sys_storage_upload_part PRIMARY KEY (id),
    CONSTRAINT uk_sys_storage_upload_part__claim_part UNIQUE (claim_id, part_number),
    CONSTRAINT fk_sys_storage_upload_part__claim FOREIGN KEY (claim_id)
        REFERENCES sys_storage_upload_claim(id) ON DELETE CASCADE
);

COMMENT ON TABLE sys_storage_upload_part IS 'Multipart upload parts';
COMMENT ON COLUMN sys_storage_upload_part.id IS 'Primary key';
COMMENT ON COLUMN sys_storage_upload_part.claim_id IS 'Owning claim ID';
COMMENT ON COLUMN sys_storage_upload_part.part_number IS 'Part number';
COMMENT ON COLUMN sys_storage_upload_part.etag IS 'Part ETag';
COMMENT ON COLUMN sys_storage_upload_part.size IS 'Part size in bytes';
COMMENT ON COLUMN sys_storage_upload_part.created_at IS 'Created at';

-- No standalone (claim_id) index: the unique constraint
-- uk_sys_storage_upload_part__claim_part(claim_id, part_number) already
-- covers all WHERE claim_id = ? lookups via leftmost-prefix, the
-- ORDER BY part_number in ListByClaim, the ON CONFLICT target, and the
-- FK cascade probe.

CREATE TABLE IF NOT EXISTS sys_storage_pending_delete (
    id              VARCHAR(128) NOT NULL,
    object_key      VARCHAR(512) NOT NULL,
    upload_id       VARCHAR(128) NOT NULL DEFAULT '',
    reason          VARCHAR(128) NOT NULL DEFAULT 'replaced',
    attempts        INTEGER      NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    created_at      TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    CONSTRAINT pk_sys_storage_pending_delete PRIMARY KEY (id)
);

COMMENT ON TABLE sys_storage_pending_delete IS 'Pending object deletions';
COMMENT ON COLUMN sys_storage_pending_delete.id IS 'Primary key';
COMMENT ON COLUMN sys_storage_pending_delete.object_key IS 'Object key';
COMMENT ON COLUMN sys_storage_pending_delete.upload_id IS 'Multipart session ID';
COMMENT ON COLUMN sys_storage_pending_delete.reason IS 'Deletion reason';
COMMENT ON COLUMN sys_storage_pending_delete.attempts IS 'Attempt count';
COMMENT ON COLUMN sys_storage_pending_delete.next_attempt_at IS 'Next attempt at';
COMMENT ON COLUMN sys_storage_pending_delete.created_at IS 'Created at';

-- attempts is intentionally NOT part of the index: Lease only filters and
-- orders by next_attempt_at; attempts is only ever mutated by Defer
-- (SET attempts = attempts + 1) and never appears in WHERE/ORDER BY.
CREATE INDEX idx_sys_storage_pending_delete__lease ON sys_storage_pending_delete(next_attempt_at);

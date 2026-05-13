--------------------------------------------------------------------------------
-- Storage Module Tables
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS sys_storage_upload_claim (
    id                VARCHAR(32)  NOT NULL,
    created_at        TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by        VARCHAR(32)  NOT NULL DEFAULT 'system',
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

COMMENT ON TABLE sys_storage_upload_claim IS '上传凭证';
COMMENT ON COLUMN sys_storage_upload_claim.id IS '主键';
COMMENT ON COLUMN sys_storage_upload_claim.created_at IS '创建时间';
COMMENT ON COLUMN sys_storage_upload_claim.created_by IS '上传发起人';
COMMENT ON COLUMN sys_storage_upload_claim.object_key IS '对象KEY';
COMMENT ON COLUMN sys_storage_upload_claim.upload_id IS '分片会话ID';
COMMENT ON COLUMN sys_storage_upload_claim.size IS '声明大小';
COMMENT ON COLUMN sys_storage_upload_claim.content_type IS 'MIME类型';
COMMENT ON COLUMN sys_storage_upload_claim.original_filename IS '原始文件名';
COMMENT ON COLUMN sys_storage_upload_claim.status IS '状态';
COMMENT ON COLUMN sys_storage_upload_claim.public IS '是否公开';
COMMENT ON COLUMN sys_storage_upload_claim.part_size IS '分片大小';
COMMENT ON COLUMN sys_storage_upload_claim.part_count IS '分片总数';
COMMENT ON COLUMN sys_storage_upload_claim.expires_at IS '过期时间';

CREATE INDEX idx_sys_storage_upload_claim__expires_at ON sys_storage_upload_claim(expires_at);
CREATE INDEX idx_sys_storage_upload_claim__status ON sys_storage_upload_claim(status);

CREATE TABLE IF NOT EXISTS sys_storage_upload_part (
    id          VARCHAR(32)  NOT NULL,
    claim_id    VARCHAR(32)  NOT NULL,
    part_number INTEGER      NOT NULL,
    etag        VARCHAR(64)  NOT NULL,
    size        BIGINT       NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    CONSTRAINT pk_sys_storage_upload_part PRIMARY KEY (id),
    CONSTRAINT uk_sys_storage_upload_part__claim_part UNIQUE (claim_id, part_number),
    CONSTRAINT fk_sys_storage_upload_part__claim FOREIGN KEY (claim_id)
        REFERENCES sys_storage_upload_claim(id) ON DELETE CASCADE
);

COMMENT ON TABLE sys_storage_upload_part IS '分片记录';
COMMENT ON COLUMN sys_storage_upload_part.id IS '主键';
COMMENT ON COLUMN sys_storage_upload_part.claim_id IS '所属Claim';
COMMENT ON COLUMN sys_storage_upload_part.part_number IS '分片编号';
COMMENT ON COLUMN sys_storage_upload_part.etag IS '分片ETag';
COMMENT ON COLUMN sys_storage_upload_part.size IS '分片字节数';
COMMENT ON COLUMN sys_storage_upload_part.created_at IS '创建时间';

CREATE INDEX idx_sys_storage_upload_part__claim ON sys_storage_upload_part(claim_id);

CREATE TABLE IF NOT EXISTS sys_storage_pending_delete (
    id              VARCHAR(32)  NOT NULL,
    object_key      VARCHAR(512) NOT NULL,
    upload_id       VARCHAR(128) NOT NULL DEFAULT '',
    reason          VARCHAR(32)  NOT NULL DEFAULT 'replaced',
    attempts        INTEGER      NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    created_at      TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    CONSTRAINT pk_sys_storage_pending_delete PRIMARY KEY (id)
);

COMMENT ON TABLE sys_storage_pending_delete IS '对象删除队列';
COMMENT ON COLUMN sys_storage_pending_delete.id IS '主键';
COMMENT ON COLUMN sys_storage_pending_delete.object_key IS '对象KEY';
COMMENT ON COLUMN sys_storage_pending_delete.upload_id IS '分片会话ID';
COMMENT ON COLUMN sys_storage_pending_delete.reason IS '原因';
COMMENT ON COLUMN sys_storage_pending_delete.attempts IS '重试次数';
COMMENT ON COLUMN sys_storage_pending_delete.next_attempt_at IS '下次尝试';
COMMENT ON COLUMN sys_storage_pending_delete.created_at IS '创建时间';

CREATE INDEX idx_sys_storage_pending_delete__lease ON sys_storage_pending_delete(next_attempt_at, attempts);

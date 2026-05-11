--------------------------------------------------------------------------------
-- Storage Module Tables
--------------------------------------------------------------------------------

-- Upload claims: in-flight upload bookkeeping. Rows are inserted by the
-- upload init flow and deleted either by the business transaction
-- (on commit) or by the claim sweeper worker (on TTL expiry).
CREATE TABLE IF NOT EXISTS storage_upload_claims (
    id            VARCHAR(32)  NOT NULL,
    created_at    TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by    VARCHAR(32)  NOT NULL DEFAULT 'system',
    object_key    VARCHAR(512) NOT NULL,
    upload_id     VARCHAR(128) NOT NULL DEFAULT '',
    bucket        VARCHAR(64)  NOT NULL,
    size          BIGINT       NOT NULL DEFAULT 0,
    content_type  VARCHAR(128) NOT NULL DEFAULT '',
    metadata      JSONB,
    expires_at    TIMESTAMP    NOT NULL,
    CONSTRAINT pk_storage_upload_claims PRIMARY KEY (id),
    CONSTRAINT uk_storage_upload_claims__object_key UNIQUE (object_key)
);

COMMENT ON TABLE storage_upload_claims IS '上传凭证（短命）';
COMMENT ON COLUMN storage_upload_claims.id IS '主键';
COMMENT ON COLUMN storage_upload_claims.created_at IS '创建时间';
COMMENT ON COLUMN storage_upload_claims.created_by IS '上传发起人 ID';
COMMENT ON COLUMN storage_upload_claims.object_key IS '对象 key';
COMMENT ON COLUMN storage_upload_claims.upload_id IS 'Multipart 会话 ID（direct 模式为空串）';
COMMENT ON COLUMN storage_upload_claims.bucket IS '所在 bucket';
COMMENT ON COLUMN storage_upload_claims.size IS '客户端声明大小（字节）';
COMMENT ON COLUMN storage_upload_claims.content_type IS 'MIME 类型';
COMMENT ON COLUMN storage_upload_claims.metadata IS '附加元数据（JSON）';
COMMENT ON COLUMN storage_upload_claims.expires_at IS '过期时间，超过则被 sweeper 清理';

CREATE INDEX idx_storage_upload_claims__expires_at ON storage_upload_claims(expires_at);

-- Pending object deletions: durable queue drained by the delete worker.
-- Rows are inserted by the CRUD layer inside the business transaction.
CREATE TABLE IF NOT EXISTS storage_pending_deletes (
    id              VARCHAR(32)  NOT NULL,
    object_key      VARCHAR(512) NOT NULL,
    reason          VARCHAR(32)  NOT NULL DEFAULT 'replaced',
    attempts        INTEGER      NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    created_at      TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    CONSTRAINT pk_storage_pending_deletes PRIMARY KEY (id)
);

COMMENT ON TABLE storage_pending_deletes IS '对象删除队列（短命）';
COMMENT ON COLUMN storage_pending_deletes.id IS '主键';
COMMENT ON COLUMN storage_pending_deletes.object_key IS '待删除对象 key';
COMMENT ON COLUMN storage_pending_deletes.reason IS '调度原因：replaced / deleted / claim_expired';
COMMENT ON COLUMN storage_pending_deletes.attempts IS '失败重试计数';
COMMENT ON COLUMN storage_pending_deletes.next_attempt_at IS '下次尝试时间（用于退避与 lease 可见性超时）';
COMMENT ON COLUMN storage_pending_deletes.created_at IS '创建时间';

CREATE INDEX idx_storage_pending_deletes__lease ON storage_pending_deletes(next_attempt_at, attempts);

-- --------------------------------------------------------------------------------
-- Storage Module Tables
-- --------------------------------------------------------------------------------

-- Upload claims: in-flight upload bookkeeping. Rows are inserted by the
-- upload init flow and deleted either by the business transaction
-- (on commit) or by the claim sweeper worker (on TTL expiry).
CREATE TABLE IF NOT EXISTS storage_upload_claims (
    id            VARCHAR(32)  NOT NULL                       COMMENT '主键',
    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    created_by    VARCHAR(32)  NOT NULL DEFAULT 'system'      COMMENT '上传发起人 ID',
    object_key    VARCHAR(512) NOT NULL                       COMMENT '对象 key',
    upload_id     VARCHAR(128) NOT NULL DEFAULT ''            COMMENT 'Multipart 会话 ID（direct 模式为空串）',
    bucket        VARCHAR(64)  NOT NULL                       COMMENT '所在 bucket',
    size          BIGINT       NOT NULL DEFAULT 0             COMMENT '客户端声明大小（字节）',
    content_type  VARCHAR(128) NOT NULL DEFAULT ''            COMMENT 'MIME 类型',
    metadata      JSON                                        COMMENT '附加元数据（JSON）',
    expires_at    DATETIME     NOT NULL                       COMMENT '过期时间，超过则被 sweeper 清理',
    CONSTRAINT pk_storage_upload_claims PRIMARY KEY (id),
    CONSTRAINT uk_storage_upload_claims__object_key UNIQUE (object_key)
) COMMENT '上传凭证（短命）';

CREATE INDEX idx_storage_upload_claims__expires_at ON storage_upload_claims(expires_at);

-- Pending object deletions: durable queue drained by the delete worker.
-- Rows are inserted by the CRUD layer inside the business transaction.
CREATE TABLE IF NOT EXISTS storage_pending_deletes (
    id              VARCHAR(32)  NOT NULL                       COMMENT '主键',
    object_key      VARCHAR(512) NOT NULL                       COMMENT '待删除对象 key',
    reason          VARCHAR(32)  NOT NULL DEFAULT 'replaced'    COMMENT '调度原因：replaced / deleted / claim_expired',
    attempts        INTEGER      NOT NULL DEFAULT 0             COMMENT '失败重试计数',
    next_attempt_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '下次尝试时间（用于退避与 lease 可见性超时）',
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    CONSTRAINT pk_storage_pending_deletes PRIMARY KEY (id)
) COMMENT '对象删除队列（短命）';

CREATE INDEX idx_storage_pending_deletes__lease ON storage_pending_deletes(next_attempt_at, attempts);

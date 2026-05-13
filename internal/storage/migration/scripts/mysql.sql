-- --------------------------------------------------------------------------------
-- Storage Module Tables
-- --------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS sys_storage_upload_claim (
    id                VARCHAR(32)  NOT NULL                            COMMENT '主键',
    created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT '创建时间',
    created_by        VARCHAR(32)  NOT NULL DEFAULT 'system'           COMMENT '上传发起人',
    object_key        VARCHAR(512) NOT NULL                            COMMENT '对象KEY',
    upload_id         VARCHAR(128) NOT NULL DEFAULT ''                 COMMENT '分片会话ID',
    size              BIGINT       NOT NULL DEFAULT 0                  COMMENT '声明大小',
    content_type      VARCHAR(128) NOT NULL DEFAULT ''                 COMMENT 'MIME类型',
    original_filename VARCHAR(255) NOT NULL DEFAULT ''                 COMMENT '原始文件名',
    status            VARCHAR(16)  NOT NULL DEFAULT 'pending'          COMMENT '状态',
    public            BOOLEAN      NOT NULL DEFAULT FALSE              COMMENT '是否公开',
    part_size         BIGINT       NOT NULL DEFAULT 0                  COMMENT '分片大小',
    part_count        INTEGER      NOT NULL DEFAULT 0                  COMMENT '分片总数',
    expires_at        DATETIME     NOT NULL                            COMMENT '过期时间',
    CONSTRAINT pk_sys_storage_upload_claim PRIMARY KEY (id),
    CONSTRAINT uk_sys_storage_upload_claim__object_key UNIQUE (object_key)
) COMMENT '上传凭证';

CREATE INDEX idx_sys_storage_upload_claim__expires_at ON sys_storage_upload_claim(expires_at);
CREATE INDEX idx_sys_storage_upload_claim__status ON sys_storage_upload_claim(status);

CREATE TABLE IF NOT EXISTS sys_storage_upload_part (
    id          VARCHAR(32)  NOT NULL                            COMMENT '主键',
    claim_id    VARCHAR(32)  NOT NULL                            COMMENT '所属Claim',
    part_number INTEGER      NOT NULL                            COMMENT '分片编号',
    etag        VARCHAR(64)  NOT NULL                            COMMENT '分片ETag',
    size        BIGINT       NOT NULL                            COMMENT '分片字节数',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT '创建时间',
    CONSTRAINT pk_sys_storage_upload_part PRIMARY KEY (id),
    CONSTRAINT uk_sys_storage_upload_part__claim_part UNIQUE (claim_id, part_number),
    CONSTRAINT fk_sys_storage_upload_part__claim FOREIGN KEY (claim_id)
        REFERENCES sys_storage_upload_claim(id) ON DELETE CASCADE
) COMMENT '分片记录';

CREATE INDEX idx_sys_storage_upload_part__claim ON sys_storage_upload_part(claim_id);

CREATE TABLE IF NOT EXISTS sys_storage_pending_delete (
    id              VARCHAR(32)  NOT NULL                            COMMENT '主键',
    object_key      VARCHAR(512) NOT NULL                            COMMENT '对象KEY',
    upload_id       VARCHAR(128) NOT NULL DEFAULT ''                 COMMENT '分片会话ID',
    reason          VARCHAR(32)  NOT NULL DEFAULT 'replaced'         COMMENT '原因',
    attempts        INTEGER      NOT NULL DEFAULT 0                  COMMENT '重试次数',
    next_attempt_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT '下次尝试',
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP  COMMENT '创建时间',
    CONSTRAINT pk_sys_storage_pending_delete PRIMARY KEY (id)
) COMMENT '对象删除队列';

CREATE INDEX idx_sys_storage_pending_delete__lease ON sys_storage_pending_delete(next_attempt_at, attempts);

-- Event Outbox (sys_event_outbox) — MySQL dialect
-- See postgres.sql for column-by-column documentation.

CREATE TABLE IF NOT EXISTS sys_event_outbox (
    id              VARCHAR(32)  NOT NULL,
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by      VARCHAR(32)  NOT NULL DEFAULT 'system',
    event_id        VARCHAR(64)  NOT NULL,
    event_type      VARCHAR(128) NOT NULL,
    source          VARCHAR(128) NOT NULL DEFAULT '',
    trace_id        VARCHAR(64),
    span_id         VARCHAR(64),
    correlation_id  VARCHAR(128),
    headers         JSON,
    payload         JSON         NOT NULL,
    status          VARCHAR(16)  NOT NULL DEFAULT 'pending',
    retry_count     INT          NOT NULL DEFAULT 0,
    last_error      TEXT,
    processed_at    DATETIME,
    retry_after     DATETIME,
    occurred_at     DATETIME     NOT NULL,
    CONSTRAINT pk_sys_event_outbox PRIMARY KEY (id),
    CONSTRAINT uk_sys_event_outbox__event_id UNIQUE (event_id),
    INDEX idx_sys_event_outbox__relay (status, retry_after, created_at),
    INDEX idx_sys_event_outbox__type_created (event_type, created_at)
) COMMENT='Event outbox for transactional publishing';

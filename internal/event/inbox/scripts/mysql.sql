-- Inbox MySQL

CREATE TABLE IF NOT EXISTS sys_event_inbox (
    id              VARCHAR(32)  NOT NULL,
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by      VARCHAR(32)  NOT NULL DEFAULT 'system',
    event_id        VARCHAR(64)  NOT NULL,
    consumer_group  VARCHAR(128) NOT NULL,
    status          VARCHAR(16)  NOT NULL DEFAULT 'processing',
    lock_id         VARCHAR(32),
    locked_until    DATETIME,
    completed_at    DATETIME,
    CONSTRAINT pk_sys_event_inbox PRIMARY KEY (id),
    CONSTRAINT uk_sys_event_inbox__group_event UNIQUE (consumer_group, event_id),
    INDEX idx_sys_event_inbox__completed_at (completed_at)
) COMMENT='Inbox';

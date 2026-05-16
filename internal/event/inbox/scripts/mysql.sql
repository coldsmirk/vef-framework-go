-- Event Inbox (sys_event_inbox) — MySQL dialect
-- See postgres.sql for column-by-column documentation.

CREATE TABLE IF NOT EXISTS sys_event_inbox (
    id              VARCHAR(32)  NOT NULL,
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by      VARCHAR(32)  NOT NULL DEFAULT 'system',
    event_id        VARCHAR(64)  NOT NULL,
    consumer_group  VARCHAR(128) NOT NULL,
    CONSTRAINT pk_sys_event_inbox PRIMARY KEY (id),
    CONSTRAINT uk_sys_event_inbox__group_event UNIQUE (consumer_group, event_id),
    INDEX idx_sys_event_inbox__created_at (created_at)
) COMMENT='Idempotency markers for at-least-once consumer groups';

-- Inbox SQLite

CREATE TABLE IF NOT EXISTS sys_event_inbox (
    id              TEXT     NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by      TEXT     NOT NULL DEFAULT 'system',
    event_id        TEXT     NOT NULL,
    consumer_group  TEXT     NOT NULL,
    status          TEXT     NOT NULL DEFAULT 'processing',
    lock_id         TEXT,
    locked_until    DATETIME,
    completed_at    DATETIME,
    CONSTRAINT pk_sys_event_inbox PRIMARY KEY (id),
    CONSTRAINT uk_sys_event_inbox__group_event UNIQUE (consumer_group, event_id)
);

CREATE INDEX IF NOT EXISTS idx_sys_event_inbox__completed_at ON sys_event_inbox(completed_at);

-- Event Outbox (sys_event_outbox) — SQLite dialect
-- See postgres.sql for column-by-column documentation.

CREATE TABLE IF NOT EXISTS sys_event_outbox (
    id              TEXT     NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by      TEXT     NOT NULL DEFAULT 'system',
    event_id        TEXT     NOT NULL,
    event_type      TEXT     NOT NULL,
    source          TEXT     NOT NULL DEFAULT '',
    trace_id        TEXT,
    span_id         TEXT,
    correlation_id  TEXT,
    headers         TEXT,
    payload         TEXT     NOT NULL,
    status          TEXT     NOT NULL DEFAULT 'pending',
    retry_count     INTEGER  NOT NULL DEFAULT 0,
    last_error      TEXT,
    processed_at    DATETIME,
    retry_after     DATETIME,
    occurred_at     DATETIME NOT NULL,
    CONSTRAINT pk_sys_event_outbox PRIMARY KEY (id),
    CONSTRAINT uk_sys_event_outbox__event_id UNIQUE (event_id)
);

CREATE INDEX IF NOT EXISTS idx_sys_event_outbox__relay ON sys_event_outbox(status, retry_after, created_at);
CREATE INDEX IF NOT EXISTS idx_sys_event_outbox__type_created ON sys_event_outbox(event_type, created_at);

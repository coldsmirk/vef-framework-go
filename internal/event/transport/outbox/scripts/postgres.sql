-- Outbox PG

CREATE TABLE IF NOT EXISTS sys_event_outbox (
    id              VARCHAR(32)  NOT NULL,
    created_at      TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by      VARCHAR(32)  NOT NULL DEFAULT 'system',
    event_id        VARCHAR(64)  NOT NULL,
    event_type      VARCHAR(128) NOT NULL,
    source          VARCHAR(128) NOT NULL DEFAULT '',
    trace_id        VARCHAR(64),
    span_id         VARCHAR(64),
    correlation_id  VARCHAR(128),
    headers         JSONB,
    payload         JSONB        NOT NULL,
    status          VARCHAR(16)  NOT NULL DEFAULT 'pending',
    retry_count     INTEGER      NOT NULL DEFAULT 0,
    last_error      TEXT,
    processed_at    TIMESTAMP,
    retry_after     TIMESTAMP,
    occurred_at     TIMESTAMP    NOT NULL,
    CONSTRAINT pk_sys_event_outbox PRIMARY KEY (id),
    CONSTRAINT uk_sys_event_outbox__event_id UNIQUE (event_id)
);

COMMENT ON TABLE sys_event_outbox IS 'Outbox';
COMMENT ON COLUMN sys_event_outbox.id IS 'ID';
COMMENT ON COLUMN sys_event_outbox.created_at IS 'Created';
COMMENT ON COLUMN sys_event_outbox.created_by IS 'Creator';
COMMENT ON COLUMN sys_event_outbox.event_id IS 'Event ID';
COMMENT ON COLUMN sys_event_outbox.event_type IS 'Type';
COMMENT ON COLUMN sys_event_outbox.source IS 'Source';
COMMENT ON COLUMN sys_event_outbox.trace_id IS 'Trace ID';
COMMENT ON COLUMN sys_event_outbox.span_id IS 'Span ID';
COMMENT ON COLUMN sys_event_outbox.correlation_id IS 'Correlation ID';
COMMENT ON COLUMN sys_event_outbox.headers IS 'Headers';
COMMENT ON COLUMN sys_event_outbox.payload IS 'Payload';
COMMENT ON COLUMN sys_event_outbox.status IS 'Status';
COMMENT ON COLUMN sys_event_outbox.retry_count IS 'Retries';
COMMENT ON COLUMN sys_event_outbox.last_error IS 'Error';
COMMENT ON COLUMN sys_event_outbox.processed_at IS 'Processed';
COMMENT ON COLUMN sys_event_outbox.retry_after IS 'Retry at';
COMMENT ON COLUMN sys_event_outbox.occurred_at IS 'Occurred';

CREATE INDEX idx_sys_event_outbox__relay ON sys_event_outbox(status, retry_after, created_at);
CREATE INDEX idx_sys_event_outbox__type_created ON sys_event_outbox(event_type, created_at);
CREATE INDEX idx_sys_event_outbox__cleanup ON sys_event_outbox(status, processed_at);

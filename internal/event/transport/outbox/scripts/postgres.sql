--------------------------------------------------------------------------------
-- Event Outbox (sys_event_outbox)
--
-- Stores frames published with event.WithTx so that downstream delivery
-- is decoupled from the publishing transaction. The relay loop polls
-- pending and retry-eligible rows under FOR UPDATE SKIP LOCKED, claims
-- them as 'processing', dispatches to the configured sink Transport,
-- then marks completed / failed / dead.
--------------------------------------------------------------------------------

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

COMMENT ON TABLE sys_event_outbox IS 'Event outbox for transactional publishing';
COMMENT ON COLUMN sys_event_outbox.id IS 'Row primary key';
COMMENT ON COLUMN sys_event_outbox.created_at IS 'Insert time';
COMMENT ON COLUMN sys_event_outbox.created_by IS 'Inserting principal';
COMMENT ON COLUMN sys_event_outbox.event_id IS 'Envelope ID, stable across retries';
COMMENT ON COLUMN sys_event_outbox.event_type IS 'Envelope Type, drives routing';
COMMENT ON COLUMN sys_event_outbox.source IS 'Producing application';
COMMENT ON COLUMN sys_event_outbox.trace_id IS 'W3C trace ID';
COMMENT ON COLUMN sys_event_outbox.span_id IS 'W3C span ID';
COMMENT ON COLUMN sys_event_outbox.correlation_id IS 'Caller-supplied correlation key';
COMMENT ON COLUMN sys_event_outbox.headers IS 'Envelope headers (JSON map)';
COMMENT ON COLUMN sys_event_outbox.payload IS 'Canonical JSON of the original Event';
COMMENT ON COLUMN sys_event_outbox.status IS 'pending|processing|completed|failed|dead';
COMMENT ON COLUMN sys_event_outbox.retry_count IS 'Number of failed dispatches';
COMMENT ON COLUMN sys_event_outbox.last_error IS 'Most recent error string';
COMMENT ON COLUMN sys_event_outbox.processed_at IS 'When the row reached completed/dead';
COMMENT ON COLUMN sys_event_outbox.retry_after IS 'Next retry time (also the processing lease deadline)';
COMMENT ON COLUMN sys_event_outbox.occurred_at IS 'Business event time';

-- Composite (status, retry_after, created_at) serves the relay's poll:
-- WHERE status IN (pending,failed,processing) AND retry_after <= now ORDER BY created_at LIMIT n.
CREATE INDEX idx_sys_event_outbox__relay ON sys_event_outbox(status, retry_after, created_at);
-- Supports per-type analytics and cleanup queries.
CREATE INDEX idx_sys_event_outbox__type_created ON sys_event_outbox(event_type, created_at);

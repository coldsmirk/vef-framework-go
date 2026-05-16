--------------------------------------------------------------------------------
-- Event Inbox (sys_event_inbox)
--
-- Idempotency marker table for the consume-side Inbox middleware. A
-- successful insertion of (consumer_group, event_id) proves "first
-- delivery"; a duplicate-key violation means the message has already
-- been processed and the handler should be skipped.
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS sys_event_inbox (
    id              VARCHAR(32)  NOT NULL,
    created_at      TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by      VARCHAR(32)  NOT NULL DEFAULT 'system',
    event_id        VARCHAR(64)  NOT NULL,
    consumer_group  VARCHAR(128) NOT NULL,
    CONSTRAINT pk_sys_event_inbox PRIMARY KEY (id),
    CONSTRAINT uk_sys_event_inbox__group_event UNIQUE (consumer_group, event_id)
);

COMMENT ON TABLE sys_event_inbox IS 'Idempotency markers for at-least-once consumer groups';
COMMENT ON COLUMN sys_event_inbox.id IS 'Row primary key';
COMMENT ON COLUMN sys_event_inbox.created_at IS 'Insert time (drives retention cleanup)';
COMMENT ON COLUMN sys_event_inbox.created_by IS 'Inserting principal';
COMMENT ON COLUMN sys_event_inbox.event_id IS 'Envelope ID consumed';
COMMENT ON COLUMN sys_event_inbox.consumer_group IS 'Dedupe scope';

-- Retention cleanup scans by created_at; covering composite index
-- keeps the sweep cheap.
CREATE INDEX idx_sys_event_inbox__created_at ON sys_event_inbox(created_at);

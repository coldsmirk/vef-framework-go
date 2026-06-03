-- Inbox PG

CREATE TABLE IF NOT EXISTS sys_event_inbox (
    id              VARCHAR(32)  NOT NULL,
    created_at      TIMESTAMP    NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by      VARCHAR(32)  NOT NULL DEFAULT 'system',
    event_id        VARCHAR(64)  NOT NULL,
    consumer_group  VARCHAR(128) NOT NULL,
    status          VARCHAR(16)  NOT NULL DEFAULT 'processing',
    lock_id         VARCHAR(32),
    locked_until    TIMESTAMP,
    completed_at    TIMESTAMP,
    CONSTRAINT pk_sys_event_inbox PRIMARY KEY (id),
    CONSTRAINT uk_sys_event_inbox__group_event UNIQUE (consumer_group, event_id)
);

COMMENT ON TABLE sys_event_inbox IS 'Inbox';
COMMENT ON COLUMN sys_event_inbox.id IS 'ID';
COMMENT ON COLUMN sys_event_inbox.created_at IS 'Created';
COMMENT ON COLUMN sys_event_inbox.created_by IS 'Creator';
COMMENT ON COLUMN sys_event_inbox.event_id IS 'Event ID';
COMMENT ON COLUMN sys_event_inbox.consumer_group IS 'Group';
COMMENT ON COLUMN sys_event_inbox.status IS 'Status';
COMMENT ON COLUMN sys_event_inbox.lock_id IS 'Lock ID';
COMMENT ON COLUMN sys_event_inbox.locked_until IS 'Lease';
COMMENT ON COLUMN sys_event_inbox.completed_at IS 'Done';

CREATE INDEX IF NOT EXISTS idx_sys_event_inbox__completed_at ON sys_event_inbox(completed_at);

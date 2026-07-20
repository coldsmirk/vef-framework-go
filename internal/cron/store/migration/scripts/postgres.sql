--------------------------------------------------------------------------------
-- Cron Durable Schedule Store Tables
--------------------------------------------------------------------------------

-- Persisted schedule (trigger + policies + scheduling state)
CREATE TABLE crn_schedule (
    id VARCHAR(32) CONSTRAINT pk_crn_schedule PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    name VARCHAR(128) NOT NULL,
    job_name VARCHAR(128) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    expr TEXT NOT NULL,
    timezone VARCHAR(64) NOT NULL,
    every_ms BIGINT NOT NULL DEFAULT 0,
    fire_at_unix_ms BIGINT,
    starts_at_unix_ms BIGINT,
    ends_at_unix_ms BIGINT,
    anchor_at_unix_ms BIGINT NOT NULL,
    params JSONB,
    misfire_policy VARCHAR(16) NOT NULL DEFAULT 'fire_now',
    concurrency_policy VARCHAR(16) NOT NULL DEFAULT 'forbid',
    recover BOOLEAN NOT NULL DEFAULT false,
    timeout_ms BIGINT NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    next_fire_at_unix_ms BIGINT,
    last_fire_at_unix_ms BIGINT,
    CONSTRAINT uk_crn_schedule__name UNIQUE (name)
);

COMMENT ON TABLE crn_schedule IS 'Cron Schedule';
COMMENT ON COLUMN crn_schedule.id IS 'ID';
COMMENT ON COLUMN crn_schedule.created_at IS 'Created';
COMMENT ON COLUMN crn_schedule.updated_at IS 'Updated';
COMMENT ON COLUMN crn_schedule.created_by IS 'Creator';
COMMENT ON COLUMN crn_schedule.updated_by IS 'Updater';
COMMENT ON COLUMN crn_schedule.name IS 'Name';
COMMENT ON COLUMN crn_schedule.job_name IS 'Job Name';
COMMENT ON COLUMN crn_schedule.kind IS 'Trigger Kind (cron / interval / once)';
COMMENT ON COLUMN crn_schedule.expr IS 'Cron Expression';
COMMENT ON COLUMN crn_schedule.timezone IS 'IANA Timezone';
COMMENT ON COLUMN crn_schedule.every_ms IS 'Fixed Rate (ms)';
COMMENT ON COLUMN crn_schedule.fire_at_unix_ms IS 'One-Shot Fire Time (Unix ms)';
COMMENT ON COLUMN crn_schedule.starts_at_unix_ms IS 'Window Start (Unix ms)';
COMMENT ON COLUMN crn_schedule.ends_at_unix_ms IS 'Window End (Unix ms)';
COMMENT ON COLUMN crn_schedule.anchor_at_unix_ms IS 'Interval Phase Anchor (Unix ms)';
COMMENT ON COLUMN crn_schedule.params IS 'Handler Params';
COMMENT ON COLUMN crn_schedule.misfire_policy IS 'Misfire Policy (fire_now / skip)';
COMMENT ON COLUMN crn_schedule.concurrency_policy IS 'Concurrency Policy (forbid / allow)';
COMMENT ON COLUMN crn_schedule.recover IS 'Re-fire Abandoned Runs';
COMMENT ON COLUMN crn_schedule.timeout_ms IS 'Per-Run Timeout (ms, 0 = config default)';
COMMENT ON COLUMN crn_schedule.is_enabled IS 'Enabled';
COMMENT ON COLUMN crn_schedule.next_fire_at_unix_ms IS 'Next Due Fire (Unix ms)';
COMMENT ON COLUMN crn_schedule.last_fire_at_unix_ms IS 'Last Executed Fire (Unix ms)';

CREATE INDEX idx_crn_schedule__is_enabled_next_fire_at_unix_ms
    ON crn_schedule(is_enabled, next_fire_at_unix_ms);

-- Durable manual/recovery fire requests, consumed atomically with run claims.
CREATE TABLE crn_fire_request (
    id VARCHAR(32) CONSTRAINT pk_crn_fire_request PRIMARY KEY,
    schedule_id VARCHAR(32) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    scheduled_at_unix_ms BIGINT NOT NULL,
    source_run_id VARCHAR(32),
    CONSTRAINT uk_crn_fire_request__source_run_id UNIQUE (source_run_id)
);

COMMENT ON TABLE crn_fire_request IS 'Cron Explicit Fire Request';
COMMENT ON COLUMN crn_fire_request.id IS 'ID';
COMMENT ON COLUMN crn_fire_request.schedule_id IS 'Schedule ID';
COMMENT ON COLUMN crn_fire_request.kind IS 'Request Kind (manual / recovery)';
COMMENT ON COLUMN crn_fire_request.scheduled_at_unix_ms IS 'Logical Fire Time (Unix ms)';
COMMENT ON COLUMN crn_fire_request.source_run_id IS 'Recovered Run ID';

CREATE INDEX idx_crn_fire_request__schedule_id_scheduled_at_unix_ms
    ON crn_fire_request(schedule_id, scheduled_at_unix_ms, id);

-- Run journal (one row per fire: executed, missed, skipped)
CREATE TABLE crn_run (
    id VARCHAR(32) CONSTRAINT pk_crn_run PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    schedule_id VARCHAR(32) NOT NULL,
    schedule_name VARCHAR(128) NOT NULL,
    job_name VARCHAR(128) NOT NULL,
    scheduled_at_unix_ms BIGINT NOT NULL,
    claimed_at_unix_ms BIGINT NOT NULL,
    status VARCHAR(16) NOT NULL,
    node_id TEXT NOT NULL,
    started_at_unix_ms BIGINT,
    finished_at_unix_ms BIGINT,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    heartbeat_at_unix_ms BIGINT,
    error TEXT NOT NULL,
    missed_count INTEGER NOT NULL DEFAULT 0
);

COMMENT ON TABLE crn_run IS 'Cron Run Journal';
COMMENT ON COLUMN crn_run.id IS 'ID';
COMMENT ON COLUMN crn_run.created_at IS 'Created';
COMMENT ON COLUMN crn_run.created_by IS 'Creator';
COMMENT ON COLUMN crn_run.schedule_id IS 'Schedule ID';
COMMENT ON COLUMN crn_run.schedule_name IS 'Schedule Name';
COMMENT ON COLUMN crn_run.job_name IS 'Job Name';
COMMENT ON COLUMN crn_run.scheduled_at_unix_ms IS 'Logical Fire Time (Unix ms)';
COMMENT ON COLUMN crn_run.claimed_at_unix_ms IS 'Claim Time (Unix ms)';
COMMENT ON COLUMN crn_run.status IS 'Status (running / succeeded / failed / missed / skipped / abandoned / canceled)';
COMMENT ON COLUMN crn_run.node_id IS 'Executing Node';
COMMENT ON COLUMN crn_run.started_at_unix_ms IS 'Execution Start (Unix ms)';
COMMENT ON COLUMN crn_run.finished_at_unix_ms IS 'Execution End (Unix ms)';
COMMENT ON COLUMN crn_run.duration_ms IS 'Duration (ms)';
COMMENT ON COLUMN crn_run.heartbeat_at_unix_ms IS 'Executor Heartbeat (Unix ms)';
COMMENT ON COLUMN crn_run.error IS 'Error';
COMMENT ON COLUMN crn_run.missed_count IS 'Missed Occurrences Covered';

CREATE INDEX idx_crn_run__schedule_id_scheduled_at_unix_ms
    ON crn_run(schedule_id, scheduled_at_unix_ms);
CREATE INDEX idx_crn_run__status_heartbeat_at_unix_ms
    ON crn_run(status, heartbeat_at_unix_ms);
CREATE INDEX idx_crn_run__schedule_id_status ON crn_run(schedule_id, status);
CREATE INDEX idx_crn_run__finished_at_unix_ms ON crn_run(finished_at_unix_ms);
CREATE INDEX idx_crn_run__claimed_at_unix_ms ON crn_run(claimed_at_unix_ms, id);

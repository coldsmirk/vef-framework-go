--------------------------------------------------------------------------------
-- Cron Durable Schedule Store Tables
--------------------------------------------------------------------------------

-- Persisted schedule (trigger + policies + scheduling state)
CREATE TABLE IF NOT EXISTS crn_schedule (
    id VARCHAR(32) CONSTRAINT pk_crn_schedule PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    name VARCHAR(128) NOT NULL,
    job_name VARCHAR(128) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    expr VARCHAR(256) NOT NULL DEFAULT '',
    timezone VARCHAR(64) NOT NULL DEFAULT '',
    every_ms BIGINT NOT NULL DEFAULT 0,
    fire_at TIMESTAMP,
    starts_at TIMESTAMP,
    ends_at TIMESTAMP,
    params JSONB,
    misfire_policy VARCHAR(16) NOT NULL DEFAULT 'fire_now',
    concurrency_policy VARCHAR(16) NOT NULL DEFAULT 'forbid',
    recover BOOLEAN NOT NULL DEFAULT false,
    timeout_ms BIGINT NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    next_fire_at TIMESTAMP,
    last_fire_at TIMESTAMP,
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
COMMENT ON COLUMN crn_schedule.fire_at IS 'One-Shot Fire Time';
COMMENT ON COLUMN crn_schedule.starts_at IS 'Window Start / Interval Anchor';
COMMENT ON COLUMN crn_schedule.ends_at IS 'Window End';
COMMENT ON COLUMN crn_schedule.params IS 'Handler Params';
COMMENT ON COLUMN crn_schedule.misfire_policy IS 'Misfire Policy (fire_now / skip)';
COMMENT ON COLUMN crn_schedule.concurrency_policy IS 'Concurrency Policy (forbid / allow)';
COMMENT ON COLUMN crn_schedule.recover IS 'Re-fire Abandoned Runs';
COMMENT ON COLUMN crn_schedule.timeout_ms IS 'Per-Run Timeout (ms, 0 = config default)';
COMMENT ON COLUMN crn_schedule.is_enabled IS 'Enabled';
COMMENT ON COLUMN crn_schedule.next_fire_at IS 'Next Due Fire';
COMMENT ON COLUMN crn_schedule.last_fire_at IS 'Last Executed Fire';

CREATE INDEX IF NOT EXISTS idx_crn_schedule__is_enabled_next_fire_at
    ON crn_schedule(is_enabled, next_fire_at);

-- Run journal (one row per fire: executed, missed, skipped)
CREATE TABLE IF NOT EXISTS crn_run (
    id VARCHAR(32) CONSTRAINT pk_crn_run PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    schedule_id VARCHAR(32) NOT NULL,
    schedule_name VARCHAR(128) NOT NULL,
    job_name VARCHAR(128) NOT NULL,
    scheduled_at TIMESTAMP NOT NULL,
    status VARCHAR(16) NOT NULL,
    node_id VARCHAR(128) NOT NULL DEFAULT '',
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    heartbeat_at TIMESTAMP,
    error TEXT,
    missed_count INTEGER NOT NULL DEFAULT 0
);

COMMENT ON TABLE crn_run IS 'Cron Run Journal';
COMMENT ON COLUMN crn_run.id IS 'ID';
COMMENT ON COLUMN crn_run.created_at IS 'Created';
COMMENT ON COLUMN crn_run.created_by IS 'Creator';
COMMENT ON COLUMN crn_run.schedule_id IS 'Schedule ID';
COMMENT ON COLUMN crn_run.schedule_name IS 'Schedule Name';
COMMENT ON COLUMN crn_run.job_name IS 'Job Name';
COMMENT ON COLUMN crn_run.scheduled_at IS 'Logical Fire Time';
COMMENT ON COLUMN crn_run.status IS 'Status (running / succeeded / failed / missed / skipped / abandoned / canceled)';
COMMENT ON COLUMN crn_run.node_id IS 'Executing Node';
COMMENT ON COLUMN crn_run.started_at IS 'Execution Start';
COMMENT ON COLUMN crn_run.finished_at IS 'Execution End';
COMMENT ON COLUMN crn_run.duration_ms IS 'Duration (ms)';
COMMENT ON COLUMN crn_run.heartbeat_at IS 'Executor Heartbeat';
COMMENT ON COLUMN crn_run.error IS 'Error';
COMMENT ON COLUMN crn_run.missed_count IS 'Missed Occurrences Covered';

CREATE INDEX IF NOT EXISTS idx_crn_run__schedule_id_scheduled_at ON crn_run(schedule_id, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_crn_run__status_heartbeat_at ON crn_run(status, heartbeat_at);
CREATE INDEX IF NOT EXISTS idx_crn_run__schedule_id_status ON crn_run(schedule_id, status);
CREATE INDEX IF NOT EXISTS idx_crn_run__finished_at ON crn_run(finished_at);
CREATE INDEX IF NOT EXISTS idx_crn_run__created_at ON crn_run(created_at);

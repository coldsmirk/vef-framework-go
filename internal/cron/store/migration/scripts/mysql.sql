-- --------------------------------------------------------------------------------
-- Cron Durable Schedule Store Tables
-- --------------------------------------------------------------------------------

-- Persisted schedule (trigger + policies + scheduling state)
CREATE TABLE IF NOT EXISTS crn_schedule (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',
    name VARCHAR(128) NOT NULL COMMENT 'Name',
    job_name VARCHAR(128) NOT NULL COMMENT 'Job Name',
    kind VARCHAR(16) NOT NULL COMMENT 'Trigger Kind (cron / interval / once)',
    expr VARCHAR(256) NOT NULL DEFAULT '' COMMENT 'Cron Expression',
    timezone VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'IANA Timezone',
    every_ms BIGINT NOT NULL DEFAULT 0 COMMENT 'Fixed Rate (ms)',
    fire_at DATETIME NULL COMMENT 'One-Shot Fire Time',
    starts_at DATETIME NULL COMMENT 'Window Start / Interval Anchor',
    ends_at DATETIME NULL COMMENT 'Window End',
    params JSON COMMENT 'Handler Params',
    misfire_policy VARCHAR(16) NOT NULL DEFAULT 'fire_now' COMMENT 'Misfire Policy (fire_now / skip)',
    concurrency_policy VARCHAR(16) NOT NULL DEFAULT 'forbid' COMMENT 'Concurrency Policy (forbid / allow)',
    recover BOOLEAN NOT NULL DEFAULT false COMMENT 'Re-fire Abandoned Runs',
    timeout_ms BIGINT NOT NULL DEFAULT 0 COMMENT 'Per-Run Timeout (ms, 0 = config default)',
    is_enabled BOOLEAN NOT NULL DEFAULT true COMMENT 'Enabled',
    next_fire_at DATETIME NULL COMMENT 'Next Due Fire',
    last_fire_at DATETIME NULL COMMENT 'Last Executed Fire',
    CONSTRAINT pk_crn_schedule PRIMARY KEY (id),
    CONSTRAINT uk_crn_schedule__name UNIQUE (name),
    -- Inline because MySQL has no CREATE INDEX IF NOT EXISTS, so a
    -- standalone index would error on re-run.
    INDEX idx_crn_schedule__is_enabled_next_fire_at (is_enabled, next_fire_at)
) COMMENT 'Cron Schedule';

-- Run journal (one row per fire: executed, missed, skipped)
CREATE TABLE IF NOT EXISTS crn_run (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    schedule_id VARCHAR(32) NOT NULL COMMENT 'Schedule ID',
    schedule_name VARCHAR(128) NOT NULL COMMENT 'Schedule Name',
    job_name VARCHAR(128) NOT NULL COMMENT 'Job Name',
    scheduled_at DATETIME NOT NULL COMMENT 'Logical Fire Time',
    status VARCHAR(16) NOT NULL COMMENT 'Status (running / succeeded / failed / missed / skipped / abandoned / canceled)',
    node_id VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Executing Node',
    started_at DATETIME NULL COMMENT 'Execution Start',
    finished_at DATETIME NULL COMMENT 'Execution End',
    duration_ms BIGINT NOT NULL DEFAULT 0 COMMENT 'Duration (ms)',
    heartbeat_at DATETIME NULL COMMENT 'Executor Heartbeat',
    error TEXT COMMENT 'Error',
    missed_count INT NOT NULL DEFAULT 0 COMMENT 'Missed Occurrences Covered',
    CONSTRAINT pk_crn_run PRIMARY KEY (id),
    INDEX idx_crn_run__schedule_id_scheduled_at (schedule_id, scheduled_at),
    INDEX idx_crn_run__status_heartbeat_at (status, heartbeat_at),
    INDEX idx_crn_run__schedule_id_status (schedule_id, status),
    INDEX idx_crn_run__finished_at (finished_at),
    INDEX idx_crn_run__created_at (created_at)
) COMMENT 'Cron Run Journal';

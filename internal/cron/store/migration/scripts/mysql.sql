-- --------------------------------------------------------------------------------
-- Cron Durable Schedule Store Tables
-- --------------------------------------------------------------------------------

-- Persisted schedule (trigger + policies + scheduling state)
CREATE TABLE crn_schedule (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',
    name VARCHAR(128) NOT NULL COMMENT 'Name',
    job_name VARCHAR(128) NOT NULL COMMENT 'Job Name',
    kind VARCHAR(16) NOT NULL COMMENT 'Trigger Kind (cron / interval / once)',
    expr TEXT NOT NULL COMMENT 'Cron Expression',
    timezone VARCHAR(64) NOT NULL COMMENT 'IANA Timezone',
    every_ms BIGINT NOT NULL DEFAULT 0 COMMENT 'Fixed Rate (ms)',
    fire_at_unix_ms BIGINT NULL COMMENT 'One-Shot Fire Time (Unix ms)',
    starts_at_unix_ms BIGINT NULL COMMENT 'Window Start (Unix ms)',
    ends_at_unix_ms BIGINT NULL COMMENT 'Window End (Unix ms)',
    anchor_at_unix_ms BIGINT NOT NULL COMMENT 'Interval Phase Anchor (Unix ms)',
    params JSON COMMENT 'Handler Params',
    misfire_policy VARCHAR(16) NOT NULL DEFAULT 'fire_now' COMMENT 'Misfire Policy (fire_now / skip)',
    concurrency_policy VARCHAR(16) NOT NULL DEFAULT 'forbid' COMMENT 'Concurrency Policy (forbid / allow)',
    recover BOOLEAN NOT NULL DEFAULT false COMMENT 'Re-fire Abandoned Runs',
    timeout_ms BIGINT NOT NULL DEFAULT 0 COMMENT 'Per-Run Timeout (ms, 0 = config default)',
    is_enabled BOOLEAN NOT NULL DEFAULT true COMMENT 'Enabled',
    next_fire_at_unix_ms BIGINT NULL COMMENT 'Next Due Fire (Unix ms)',
    last_fire_at_unix_ms BIGINT NULL COMMENT 'Last Executed Fire (Unix ms)',
    CONSTRAINT pk_crn_schedule PRIMARY KEY (id),
    CONSTRAINT uk_crn_schedule__name UNIQUE (name),
    INDEX idx_crn_schedule__is_enabled_next_fire_at_unix_ms (is_enabled, next_fire_at_unix_ms)
) COMMENT 'Cron Schedule';

-- Durable manual/recovery fire requests, consumed atomically with run claims.
CREATE TABLE crn_fire_request (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    schedule_id VARCHAR(32) NOT NULL COMMENT 'Schedule ID',
    kind VARCHAR(16) NOT NULL COMMENT 'Request Kind (manual / recovery)',
    scheduled_at_unix_ms BIGINT NOT NULL COMMENT 'Logical Fire Time (Unix ms)',
    source_run_id VARCHAR(32) NULL COMMENT 'Recovered Run ID',
    CONSTRAINT pk_crn_fire_request PRIMARY KEY (id),
    CONSTRAINT uk_crn_fire_request__source_run_id UNIQUE (source_run_id),
    INDEX idx_crn_fire_request__schedule_id_scheduled_at_unix_ms (schedule_id, scheduled_at_unix_ms, id)
) COMMENT 'Cron Explicit Fire Request';

-- Run journal (one row per fire: executed, missed, skipped)
CREATE TABLE crn_run (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    schedule_id VARCHAR(32) NOT NULL COMMENT 'Schedule ID',
    schedule_name VARCHAR(128) NOT NULL COMMENT 'Schedule Name',
    job_name VARCHAR(128) NOT NULL COMMENT 'Job Name',
    scheduled_at_unix_ms BIGINT NOT NULL COMMENT 'Logical Fire Time (Unix ms)',
    claimed_at_unix_ms BIGINT NOT NULL COMMENT 'Claim Time (Unix ms)',
    status VARCHAR(16) NOT NULL COMMENT 'Status (running / succeeded / failed / missed / skipped / abandoned / canceled)',
    node_id TEXT NOT NULL COMMENT 'Executing Node',
    started_at_unix_ms BIGINT NULL COMMENT 'Execution Start (Unix ms)',
    finished_at_unix_ms BIGINT NULL COMMENT 'Execution End (Unix ms)',
    duration_ms BIGINT NOT NULL DEFAULT 0 COMMENT 'Duration (ms)',
    heartbeat_at_unix_ms BIGINT NULL COMMENT 'Executor Heartbeat (Unix ms)',
    error TEXT NOT NULL COMMENT 'Error',
    missed_count INT NOT NULL DEFAULT 0 COMMENT 'Missed Occurrences Covered',
    CONSTRAINT pk_crn_run PRIMARY KEY (id),
    INDEX idx_crn_run__schedule_id_scheduled_at_unix_ms (schedule_id, scheduled_at_unix_ms),
    INDEX idx_crn_run__status_heartbeat_at_unix_ms (status, heartbeat_at_unix_ms),
    INDEX idx_crn_run__schedule_id_status (schedule_id, status),
    INDEX idx_crn_run__finished_at_unix_ms (finished_at_unix_ms),
    INDEX idx_crn_run__claimed_at_unix_ms (claimed_at_unix_ms, id)
) COMMENT 'Cron Run Journal';

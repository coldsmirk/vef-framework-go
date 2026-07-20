--------------------------------------------------------------------------------
-- Cron Durable Schedule Store Tables
--------------------------------------------------------------------------------

-- Persisted schedule (trigger + policies + scheduling state)
CREATE TABLE crn_schedule (
    id VARCHAR(32) NOT NULL CONSTRAINT pk_crn_schedule PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
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
    recover BOOLEAN NOT NULL DEFAULT 0,
    timeout_ms BIGINT NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    next_fire_at_unix_ms BIGINT,
    last_fire_at_unix_ms BIGINT,
    CONSTRAINT uk_crn_schedule__name UNIQUE (name)
);

CREATE INDEX idx_crn_schedule__is_enabled_next_fire_at_unix_ms
    ON crn_schedule(is_enabled, next_fire_at_unix_ms);

-- Durable manual/recovery fire requests, consumed atomically with run claims.
CREATE TABLE crn_fire_request (
    id VARCHAR(32) NOT NULL CONSTRAINT pk_crn_fire_request PRIMARY KEY,
    schedule_id VARCHAR(32) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    scheduled_at_unix_ms BIGINT NOT NULL,
    source_run_id VARCHAR(32),
    CONSTRAINT uk_crn_fire_request__source_run_id UNIQUE (source_run_id)
);

CREATE INDEX idx_crn_fire_request__schedule_id_scheduled_at_unix_ms
    ON crn_fire_request(schedule_id, scheduled_at_unix_ms, id);

-- Run journal (one row per fire: executed, missed, skipped)
CREATE TABLE crn_run (
    id VARCHAR(32) NOT NULL CONSTRAINT pk_crn_run PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
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

CREATE INDEX idx_crn_run__schedule_id_scheduled_at_unix_ms
    ON crn_run(schedule_id, scheduled_at_unix_ms);
CREATE INDEX idx_crn_run__status_heartbeat_at_unix_ms
    ON crn_run(status, heartbeat_at_unix_ms);
CREATE INDEX idx_crn_run__schedule_id_status ON crn_run(schedule_id, status);
CREATE INDEX idx_crn_run__finished_at_unix_ms ON crn_run(finished_at_unix_ms);
CREATE INDEX idx_crn_run__claimed_at_unix_ms ON crn_run(claimed_at_unix_ms);

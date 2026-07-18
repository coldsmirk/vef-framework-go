--------------------------------------------------------------------------------
-- SQLite Pragmas (for standalone script execution only;
-- the SQLite provider already sets these via DSN parameters)
--------------------------------------------------------------------------------

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

--------------------------------------------------------------------------------
-- Cron Durable Schedule Store Tables
--------------------------------------------------------------------------------

-- Persisted schedule (trigger + policies + scheduling state)
CREATE TABLE IF NOT EXISTS crn_schedule (
    id VARCHAR(32) CONSTRAINT pk_crn_schedule PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
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
    recover BOOLEAN NOT NULL DEFAULT 0,
    timeout_ms BIGINT NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    next_fire_at TIMESTAMP,
    last_fire_at TIMESTAMP,
    CONSTRAINT uk_crn_schedule__name UNIQUE (name)
);

CREATE INDEX IF NOT EXISTS idx_crn_schedule__is_enabled_next_fire_at
    ON crn_schedule(is_enabled, next_fire_at);

-- Run journal (one row per fire: executed, missed, skipped)
CREATE TABLE IF NOT EXISTS crn_run (
    id VARCHAR(32) CONSTRAINT pk_crn_run PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
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

CREATE INDEX IF NOT EXISTS idx_crn_run__schedule_id_scheduled_at ON crn_run(schedule_id, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_crn_run__status_heartbeat_at ON crn_run(status, heartbeat_at);
CREATE INDEX IF NOT EXISTS idx_crn_run__schedule_id_status ON crn_run(schedule_id, status);
CREATE INDEX IF NOT EXISTS idx_crn_run__finished_at ON crn_run(finished_at);
CREATE INDEX IF NOT EXISTS idx_crn_run__created_at ON crn_run(created_at);

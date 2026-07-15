--------------------------------------------------------------------------------
-- SQLite Pragmas (for standalone script execution only;
-- the SQLite provider already sets these via DSN parameters)
--------------------------------------------------------------------------------

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

--------------------------------------------------------------------------------
-- Integration Engine Tables
--------------------------------------------------------------------------------

-- Integration contract (standard operation definition)
CREATE TABLE IF NOT EXISTS itg_contract (
    id VARCHAR(32) CONSTRAINT pk_itg_contract PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    code VARCHAR(128) NOT NULL,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(512),
    input_schema JSONB,
    output_schema JSONB,
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    CONSTRAINT uk_itg_contract__code UNIQUE (code)
);

-- External system instance
CREATE TABLE IF NOT EXISTS itg_system (
    id VARCHAR(32) CONSTRAINT pk_itg_system PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    code VARCHAR(128) NOT NULL,
    name VARCHAR(128) NOT NULL,
    base_url VARCHAR(512) NOT NULL DEFAULT '',
    auth JSONB,
    params JSONB,
    timeout_ms INTEGER NOT NULL DEFAULT 0,
    retry JSONB,
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    CONSTRAINT uk_itg_system__code UNIQUE (code)
);

-- Adapter (system x contract script binding)
CREATE TABLE IF NOT EXISTS itg_adapter (
    id VARCHAR(32) CONSTRAINT pk_itg_adapter PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    system_id VARCHAR(32) NOT NULL,
    contract_id VARCHAR(32) NOT NULL,
    script TEXT NOT NULL,
    timeout_ms INTEGER NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    CONSTRAINT uk_itg_adapter__system_id_contract_id UNIQUE (system_id, contract_id),
    CONSTRAINT fk_itg_adapter__system_id FOREIGN KEY (system_id)
        REFERENCES itg_system(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_itg_adapter__contract_id FOREIGN KEY (contract_id)
        REFERENCES itg_contract(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_itg_adapter__contract_id ON itg_adapter(contract_id);

-- Route (route key -> system, optionally scoped to a contract).
-- contract_id has no FK so the '' wildcard sentinel stays representable;
-- referential integrity is validated at save time.
CREATE TABLE IF NOT EXISTS itg_route (
    id VARCHAR(32) CONSTRAINT pk_itg_route PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    route_key VARCHAR(128) NOT NULL DEFAULT '',
    contract_id VARCHAR(32) NOT NULL DEFAULT '',
    system_id VARCHAR(32) NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    CONSTRAINT uk_itg_route__route_key_contract_id UNIQUE (route_key, contract_id),
    CONSTRAINT fk_itg_route__system_id FOREIGN KEY (system_id)
        REFERENCES itg_system(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_itg_route__system_id ON itg_route(system_id);

-- Invocation log
CREATE TABLE IF NOT EXISTS itg_invocation_log (
    id VARCHAR(32) CONSTRAINT pk_itg_invocation_log PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    system_code VARCHAR(128) NOT NULL,
    contract_code VARCHAR(128) NOT NULL,
    failure_kind VARCHAR(16) NOT NULL DEFAULT '',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    input JSONB,
    output JSONB,
    http_trace JSONB,
    error TEXT,
    request_id VARCHAR(64) NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_itg_invocation_log__created_at ON itg_invocation_log(created_at);
CREATE INDEX IF NOT EXISTS idx_itg_invocation_log__system_code_contract_code
    ON itg_invocation_log(system_code, contract_code);

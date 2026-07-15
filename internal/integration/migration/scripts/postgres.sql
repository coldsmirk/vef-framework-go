--------------------------------------------------------------------------------
-- Integration Engine Tables
--------------------------------------------------------------------------------

-- Integration contract (standard operation definition)
CREATE TABLE IF NOT EXISTS itg_contract (
    id VARCHAR(32) CONSTRAINT pk_itg_contract PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    code VARCHAR(128) NOT NULL,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(512),
    input_schema JSONB,
    output_schema JSONB,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    CONSTRAINT uk_itg_contract__code UNIQUE (code)
);

COMMENT ON TABLE itg_contract IS 'Integration Contract';
COMMENT ON COLUMN itg_contract.id IS 'ID';
COMMENT ON COLUMN itg_contract.created_at IS 'Created';
COMMENT ON COLUMN itg_contract.updated_at IS 'Updated';
COMMENT ON COLUMN itg_contract.created_by IS 'Creator';
COMMENT ON COLUMN itg_contract.updated_by IS 'Updater';
COMMENT ON COLUMN itg_contract.code IS 'Code';
COMMENT ON COLUMN itg_contract.name IS 'Name';
COMMENT ON COLUMN itg_contract.description IS 'Description';
COMMENT ON COLUMN itg_contract.input_schema IS 'Input JSON Schema';
COMMENT ON COLUMN itg_contract.output_schema IS 'Output JSON Schema';
COMMENT ON COLUMN itg_contract.is_enabled IS 'Enabled';

-- External system instance
CREATE TABLE IF NOT EXISTS itg_system (
    id VARCHAR(32) CONSTRAINT pk_itg_system PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    code VARCHAR(128) NOT NULL,
    name VARCHAR(128) NOT NULL,
    base_url VARCHAR(512) NOT NULL DEFAULT '',
    auth JSONB,
    inbound_auth JSONB,
    data_source JSONB,
    params JSONB,
    timeout_ms INTEGER NOT NULL DEFAULT 0,
    retry JSONB,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    CONSTRAINT uk_itg_system__code UNIQUE (code)
);

COMMENT ON TABLE itg_system IS 'External System';
COMMENT ON COLUMN itg_system.id IS 'ID';
COMMENT ON COLUMN itg_system.created_at IS 'Created';
COMMENT ON COLUMN itg_system.updated_at IS 'Updated';
COMMENT ON COLUMN itg_system.created_by IS 'Creator';
COMMENT ON COLUMN itg_system.updated_by IS 'Updater';
COMMENT ON COLUMN itg_system.code IS 'Code';
COMMENT ON COLUMN itg_system.name IS 'Name';
COMMENT ON COLUMN itg_system.base_url IS 'Base URL';
COMMENT ON COLUMN itg_system.auth IS 'Auth Config';
COMMENT ON COLUMN itg_system.inbound_auth IS 'Inbound Auth Config';
COMMENT ON COLUMN itg_system.params IS 'Script Params';
COMMENT ON COLUMN itg_system.timeout_ms IS 'Call Timeout (ms)';
COMMENT ON COLUMN itg_system.retry IS 'Retry Policy';
COMMENT ON COLUMN itg_system.is_enabled IS 'Enabled';

-- Adapter (system x contract script binding)
CREATE TABLE IF NOT EXISTS itg_adapter (
    id VARCHAR(32) CONSTRAINT pk_itg_adapter PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    system_id VARCHAR(32) NOT NULL,
    contract_id VARCHAR(32) NOT NULL,
    direction VARCHAR(16) NOT NULL DEFAULT 'outbound',
    script TEXT NOT NULL,
    timeout_ms INTEGER NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    CONSTRAINT uk_itg_adapter__system_id_contract_id_direction UNIQUE (system_id, contract_id, direction),
    CONSTRAINT fk_itg_adapter__system_id FOREIGN KEY (system_id)
        REFERENCES itg_system(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_itg_adapter__contract_id FOREIGN KEY (contract_id)
        REFERENCES itg_contract(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

COMMENT ON TABLE itg_adapter IS 'Integration Adapter';
COMMENT ON COLUMN itg_adapter.id IS 'ID';
COMMENT ON COLUMN itg_adapter.created_at IS 'Created';
COMMENT ON COLUMN itg_adapter.updated_at IS 'Updated';
COMMENT ON COLUMN itg_adapter.created_by IS 'Creator';
COMMENT ON COLUMN itg_adapter.updated_by IS 'Updater';
COMMENT ON COLUMN itg_adapter.system_id IS 'System';
COMMENT ON COLUMN itg_adapter.contract_id IS 'Contract';
COMMENT ON COLUMN itg_adapter.direction IS 'Flow Direction (outbound / inbound)';
COMMENT ON COLUMN itg_adapter.script IS 'Adapter Script';
COMMENT ON COLUMN itg_adapter.timeout_ms IS 'Timeout Override (ms)';
COMMENT ON COLUMN itg_adapter.is_enabled IS 'Enabled';

CREATE INDEX IF NOT EXISTS idx_itg_adapter__contract_id ON itg_adapter(contract_id);

-- Route (route key -> system, optionally scoped to a contract)
CREATE TABLE IF NOT EXISTS itg_route (
    id VARCHAR(32) CONSTRAINT pk_itg_route PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    route_key VARCHAR(128) NOT NULL DEFAULT '',
    -- Empty scopes the rule to every contract; no FK so the wildcard
    -- sentinel stays representable (referential integrity is validated at
    -- save time).
    contract_id VARCHAR(32) NOT NULL DEFAULT '',
    system_id VARCHAR(32) NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    CONSTRAINT uk_itg_route__route_key_contract_id UNIQUE (route_key, contract_id),
    CONSTRAINT fk_itg_route__system_id FOREIGN KEY (system_id)
        REFERENCES itg_system(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

COMMENT ON TABLE itg_route IS 'Integration Route';
COMMENT ON COLUMN itg_route.id IS 'ID';
COMMENT ON COLUMN itg_route.created_at IS 'Created';
COMMENT ON COLUMN itg_route.updated_at IS 'Updated';
COMMENT ON COLUMN itg_route.created_by IS 'Creator';
COMMENT ON COLUMN itg_route.updated_by IS 'Updater';
COMMENT ON COLUMN itg_route.route_key IS 'Route Key';
COMMENT ON COLUMN itg_route.contract_id IS 'Contract Scope';
COMMENT ON COLUMN itg_route.system_id IS 'Target System';
COMMENT ON COLUMN itg_route.is_enabled IS 'Enabled';

CREATE INDEX IF NOT EXISTS idx_itg_route__system_id ON itg_route(system_id);

-- Invocation log
CREATE TABLE IF NOT EXISTS itg_invocation_log (
    id VARCHAR(32) CONSTRAINT pk_itg_invocation_log PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    system_code VARCHAR(128) NOT NULL,
    contract_code VARCHAR(128) NOT NULL,
    direction VARCHAR(16) NOT NULL DEFAULT 'outbound',
    failure_kind VARCHAR(16) NOT NULL DEFAULT '',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    input JSONB,
    output JSONB,
    http_trace JSONB,
    error TEXT,
    request_id VARCHAR(64) NOT NULL DEFAULT ''
);

COMMENT ON TABLE itg_invocation_log IS 'Integration Invocation Log';
COMMENT ON COLUMN itg_invocation_log.id IS 'ID';
COMMENT ON COLUMN itg_invocation_log.created_at IS 'Created';
COMMENT ON COLUMN itg_invocation_log.created_by IS 'Creator';
COMMENT ON COLUMN itg_invocation_log.system_code IS 'System';
COMMENT ON COLUMN itg_invocation_log.contract_code IS 'Contract';
COMMENT ON COLUMN itg_invocation_log.direction IS 'Flow Direction (outbound / inbound)';
COMMENT ON COLUMN itg_invocation_log.failure_kind IS 'Failure Kind (empty = success)';
COMMENT ON COLUMN itg_invocation_log.duration_ms IS 'Duration (ms)';
COMMENT ON COLUMN itg_invocation_log.input IS 'Input Capture';
COMMENT ON COLUMN itg_invocation_log.output IS 'Output Capture';
COMMENT ON COLUMN itg_invocation_log.http_trace IS 'HTTP Trace';
COMMENT ON COLUMN itg_invocation_log.error IS 'Error';
COMMENT ON COLUMN itg_invocation_log.request_id IS 'Request ID';

CREATE INDEX IF NOT EXISTS idx_itg_invocation_log__created_at ON itg_invocation_log(created_at);
CREATE INDEX IF NOT EXISTS idx_itg_invocation_log__system_code_contract_code
    ON itg_invocation_log(system_code, contract_code);

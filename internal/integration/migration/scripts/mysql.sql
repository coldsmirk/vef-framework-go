-- --------------------------------------------------------------------------------
-- Integration Engine Tables
-- --------------------------------------------------------------------------------

-- Integration contract (standard operation definition)
CREATE TABLE IF NOT EXISTS itg_contract (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',
    code VARCHAR(128) NOT NULL COMMENT 'Code',
    name VARCHAR(128) NOT NULL COMMENT 'Name',
    description VARCHAR(512) COMMENT 'Description',
    labels JSON COMMENT 'Labels',
    input_schema JSON COMMENT 'Input JSON Schema',
    output_schema JSON COMMENT 'Output JSON Schema',
    is_enabled BOOLEAN NOT NULL DEFAULT true COMMENT 'Enabled',
    CONSTRAINT pk_itg_contract PRIMARY KEY (id),
    CONSTRAINT uk_itg_contract__code UNIQUE (code)
) COMMENT 'Integration Contract';

-- External system instance
CREATE TABLE IF NOT EXISTS itg_system (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',
    code VARCHAR(128) NOT NULL COMMENT 'Code',
    name VARCHAR(128) NOT NULL COMMENT 'Name',
    base_url VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'Base URL',
    outbound_auth JSON COMMENT 'Outbound Auth Config',
    outbound_envelope JSON COMMENT 'Outbound Envelope Scripts',
    inbound_auth JSON COMMENT 'Inbound Auth Config',
    data_source JSON COMMENT 'Data Source',
    params JSON COMMENT 'Script Params',
    timeout_ms INTEGER NOT NULL DEFAULT 0 COMMENT 'Call Timeout (ms)',
    retry JSON COMMENT 'Retry Policy',
    is_enabled BOOLEAN NOT NULL DEFAULT true COMMENT 'Enabled',
    CONSTRAINT pk_itg_system PRIMARY KEY (id),
    CONSTRAINT uk_itg_system__code UNIQUE (code)
) COMMENT 'External System';

-- Adapter (system x contract script binding)
CREATE TABLE IF NOT EXISTS itg_adapter (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',
    system_id VARCHAR(32) NOT NULL COMMENT 'System',
    contract_id VARCHAR(32) NOT NULL COMMENT 'Contract',
    direction VARCHAR(16) NOT NULL DEFAULT 'outbound' COMMENT 'Flow Direction (outbound / inbound)',
    script TEXT NOT NULL COMMENT 'Adapter Script',
    timeout_ms INTEGER NOT NULL DEFAULT 0 COMMENT 'Timeout Override (ms)',
    is_enabled BOOLEAN NOT NULL DEFAULT true COMMENT 'Enabled',
    CONSTRAINT pk_itg_adapter PRIMARY KEY (id),
    CONSTRAINT uk_itg_adapter__system_id_contract_id_direction UNIQUE (system_id, contract_id, direction),
    CONSTRAINT fk_itg_adapter__system_id FOREIGN KEY (system_id)
        REFERENCES itg_system(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_itg_adapter__contract_id FOREIGN KEY (contract_id)
        REFERENCES itg_contract(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    -- Indexes are declared inline so a re-run of this idempotent script skips
    -- them together with the table (CREATE TABLE IF NOT EXISTS); MySQL has no
    -- CREATE INDEX IF NOT EXISTS, so a standalone index would error on re-run.
    INDEX idx_itg_adapter__contract_id (contract_id)
) COMMENT 'Integration Adapter';

-- Route (route key -> system, optionally scoped to a contract)
CREATE TABLE IF NOT EXISTS itg_route (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',
    route_key VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Route Key',
    -- Empty scopes the rule to every contract; no FK so the wildcard
    -- sentinel stays representable (referential integrity is validated at
    -- save time).
    contract_id VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'Contract Scope',
    system_id VARCHAR(32) NOT NULL COMMENT 'Target System',
    is_enabled BOOLEAN NOT NULL DEFAULT true COMMENT 'Enabled',
    CONSTRAINT pk_itg_route PRIMARY KEY (id),
    CONSTRAINT uk_itg_route__route_key_contract_id UNIQUE (route_key, contract_id),
    CONSTRAINT fk_itg_route__system_id FOREIGN KEY (system_id)
        REFERENCES itg_system(id) ON DELETE RESTRICT ON UPDATE CASCADE
) COMMENT 'Integration Route';

-- Code map (per-system code set value translation)
CREATE TABLE IF NOT EXISTS itg_code_map (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',
    system_id VARCHAR(32) NOT NULL COMMENT 'System',
    code_set VARCHAR(128) NOT NULL COMMENT 'Code Set',
    name VARCHAR(128) NOT NULL COMMENT 'Name',
    entries JSON COMMENT 'Mapping Entries',
    on_unmapped VARCHAR(16) NOT NULL DEFAULT 'reject' COMMENT 'Unmapped Policy (reject / passthrough / fallback)',
    fallback_canonical JSON COMMENT 'Fallback Canonical Value',
    fallback_external JSON COMMENT 'Fallback External Value',
    is_enabled BOOLEAN NOT NULL DEFAULT true COMMENT 'Enabled',
    CONSTRAINT pk_itg_code_map PRIMARY KEY (id),
    CONSTRAINT uk_itg_code_map__system_id_code_set UNIQUE (system_id, code_set),
    CONSTRAINT fk_itg_code_map__system_id FOREIGN KEY (system_id)
        REFERENCES itg_system(id) ON DELETE RESTRICT ON UPDATE CASCADE
) COMMENT 'Integration Code Map';

-- Invocation log
CREATE TABLE IF NOT EXISTS itg_invocation_log (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    system_code VARCHAR(128) NOT NULL COMMENT 'System',
    contract_code VARCHAR(128) NOT NULL COMMENT 'Contract',
    direction VARCHAR(16) NOT NULL DEFAULT 'outbound' COMMENT 'Flow Direction (outbound / inbound)',
    failure_kind VARCHAR(16) NOT NULL DEFAULT '' COMMENT 'Failure Kind (empty = success)',
    duration_ms BIGINT NOT NULL DEFAULT 0 COMMENT 'Duration (ms)',
    input JSON COMMENT 'Input Capture',
    output JSON COMMENT 'Output Capture',
    http_trace JSON COMMENT 'HTTP Trace',
    error TEXT COMMENT 'Error',
    request_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'Request ID',
    CONSTRAINT pk_itg_invocation_log PRIMARY KEY (id),
    INDEX idx_itg_invocation_log__created_at (created_at),
    INDEX idx_itg_invocation_log__system_code_contract_code (system_code, contract_code)
) COMMENT 'Integration Invocation Log';

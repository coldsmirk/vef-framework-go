--------------------------------------------------------------------------------
-- SQLite Pragmas (for standalone script execution only;
-- the SQLite provider already sets these via DSN parameters)
--------------------------------------------------------------------------------

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

--------------------------------------------------------------------------------
-- Flow Definition Tables
--------------------------------------------------------------------------------

-- Flow category
CREATE TABLE IF NOT EXISTS apv_flow_category (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_category PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    tenant_id VARCHAR(32) NOT NULL,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    icon VARCHAR(128),
    parent_id VARCHAR(32),
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    remark VARCHAR(256),
    CONSTRAINT uk_apv_flow_category__tenant_id_code UNIQUE (tenant_id, code),
    CONSTRAINT fk_apv_flow_category__parent_id FOREIGN KEY (parent_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_flow_category__tenant_id ON apv_flow_category(tenant_id);
CREATE INDEX IF NOT EXISTS idx_apv_flow_category__parent_id ON apv_flow_category(parent_id);

-- Flow definition
CREATE TABLE IF NOT EXISTS apv_flow (
    id VARCHAR(32) CONSTRAINT pk_apv_flow PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    tenant_id VARCHAR(32) NOT NULL,
    category_id VARCHAR(32) NOT NULL,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    icon VARCHAR(128),
    description VARCHAR(512),
    -- Data binding
    binding_mode VARCHAR(16) NOT NULL DEFAULT 'standalone',
    business_table VARCHAR(64),
    business_pk_field VARCHAR(64),
    business_title_field VARCHAR(64),
    business_status_field VARCHAR(64),
    -- Permission config
    admin_user_ids TEXT NOT NULL DEFAULT '[]',
    is_all_initiation_allowed BOOLEAN NOT NULL DEFAULT 1,
    -- Other
    instance_title_template VARCHAR(256) NOT NULL DEFAULT '{{.flowName}}-{{.instanceNo}}',
    is_active BOOLEAN NOT NULL DEFAULT 0,
    current_version INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT uk_apv_flow__tenant_id_code UNIQUE (tenant_id, code),
    CONSTRAINT fk_apv_flow__category_id FOREIGN KEY (category_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_flow__category_id ON apv_flow(category_id);
CREATE INDEX IF NOT EXISTS idx_apv_flow__tenant_id ON apv_flow(tenant_id);

-- Flow initiator config
CREATE TABLE IF NOT EXISTS apv_flow_initiator (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_initiator PRIMARY KEY,
    flow_id VARCHAR(32) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    ids TEXT NOT NULL DEFAULT '[]',
    CONSTRAINT fk_apv_flow_initiator__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_flow_initiator__flow_id ON apv_flow_initiator(flow_id);

-- Flow version
CREATE TABLE IF NOT EXISTS apv_flow_version (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_version PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',

    flow_id VARCHAR(32) NOT NULL,
    version INTEGER NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'draft',
    description VARCHAR(256),
    -- Design data
    storage_mode VARCHAR(8) NOT NULL DEFAULT 'json',
    flow_schema TEXT,
    form_schema TEXT,
    -- Publish info
    published_at TIMESTAMP,
    published_by VARCHAR(32),
    CONSTRAINT uk_apv_flow_version__flow_id_version UNIQUE (flow_id, version),
    CONSTRAINT fk_apv_flow_version__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_flow_version__flow_id_status ON apv_flow_version(flow_id, status);
-- Ensure at most one published version per flow
CREATE UNIQUE INDEX IF NOT EXISTS uk_apv_flow_version__flow_id_published ON apv_flow_version(flow_id) WHERE status = 'published';

-- Flow node
CREATE TABLE IF NOT EXISTS apv_flow_node (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_node PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',

    flow_version_id VARCHAR(32) NOT NULL,
    key VARCHAR(64) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(512),
    -- Execution type config
    execution_type VARCHAR(16) NOT NULL DEFAULT 'manual',
    -- Approval behavior config (for approval nodes)
    approval_method VARCHAR(16) NOT NULL DEFAULT 'parallel',
    pass_rule VARCHAR(16) NOT NULL DEFAULT 'all',
    pass_ratio REAL NOT NULL DEFAULT 1.00 CONSTRAINT ck_apv_flow_node__pass_ratio CHECK (pass_ratio >= 0 AND pass_ratio <= 1),
    -- Empty assignee config
    empty_assignee_action VARCHAR(32) NOT NULL DEFAULT 'auto_pass',
    fallback_user_ids TEXT NOT NULL DEFAULT '[]',
    admin_user_ids TEXT NOT NULL DEFAULT '[]',
    same_applicant_action VARCHAR(32) NOT NULL DEFAULT 'self_approve',
    -- Rollback config
    is_rollback_allowed BOOLEAN NOT NULL DEFAULT 1,
    rollback_type VARCHAR(16) NOT NULL DEFAULT 'previous',
    rollback_data_strategy VARCHAR(16),
    rollback_target_keys TEXT,
    -- Dynamic assignee config
    is_add_assignee_allowed BOOLEAN NOT NULL DEFAULT 1,
    add_assignee_types TEXT NOT NULL DEFAULT '["before", "after", "parallel"]',
    is_remove_assignee_allowed BOOLEAN NOT NULL DEFAULT 1,
    -- Field permissions config
    field_permissions TEXT NOT NULL DEFAULT '{}',
    -- CC config
    is_manual_cc_allowed BOOLEAN NOT NULL DEFAULT 1,
    -- Other config
    is_transfer_allowed BOOLEAN NOT NULL DEFAULT 1,
    is_opinion_required BOOLEAN NOT NULL DEFAULT 0,
    timeout_hours INTEGER NOT NULL DEFAULT 0 CONSTRAINT ck_apv_flow_node__timeout_hours CHECK (timeout_hours >= 0),
    timeout_action VARCHAR(16) NOT NULL DEFAULT 'none',
    timeout_notify_before_hours INTEGER NOT NULL DEFAULT 0 CONSTRAINT ck_apv_flow_node__timeout_notify_before_hours CHECK (timeout_notify_before_hours >= 0),
    urge_cooldown_minutes INTEGER NOT NULL DEFAULT 0 CONSTRAINT ck_apv_flow_node__urge_cooldown_minutes CHECK (urge_cooldown_minutes >= 0),
    -- Advanced config
    consecutive_approver_action VARCHAR(32) NOT NULL DEFAULT 'none',
    is_read_confirm_required BOOLEAN NOT NULL DEFAULT 0,
    branches TEXT,
    CONSTRAINT uk_apv_flow_node__flow_version_id_key UNIQUE (flow_version_id, key),
    CONSTRAINT fk_apv_flow_node__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- Node assignee config
CREATE TABLE IF NOT EXISTS apv_flow_node_assignee (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_node_assignee PRIMARY KEY,
    node_id VARCHAR(32) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    ids TEXT NOT NULL DEFAULT '[]',
    form_field VARCHAR(64),
    sort_order INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT fk_apv_flow_node_assignee__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_flow_node_assignee__node_id ON apv_flow_node_assignee(node_id);

-- Node CC config
CREATE TABLE IF NOT EXISTS apv_flow_node_cc (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_node_cc PRIMARY KEY,
    node_id VARCHAR(32) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    ids TEXT NOT NULL DEFAULT '[]',
    form_field VARCHAR(64),
    timing VARCHAR(16) NOT NULL DEFAULT 'always',
    CONSTRAINT fk_apv_flow_node_cc__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_flow_node_cc__node_id ON apv_flow_node_cc(node_id);

-- Flow edge (directed connection between nodes)
CREATE TABLE IF NOT EXISTS apv_flow_edge (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_edge PRIMARY KEY,
    flow_version_id VARCHAR(32) NOT NULL,
    key VARCHAR(64),
    source_node_id VARCHAR(32) NOT NULL,
    source_node_key VARCHAR(64) NOT NULL,
    target_node_id VARCHAR(32) NOT NULL,
    target_node_key VARCHAR(64) NOT NULL,
    source_handle VARCHAR(32),
    CONSTRAINT fk_apv_flow_edge__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_flow_edge__source_node_id FOREIGN KEY (source_node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_flow_edge__target_node_id FOREIGN KEY (target_node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_flow_edge__flow_version_id_source_node_id ON apv_flow_edge(flow_version_id, source_node_id);
CREATE INDEX IF NOT EXISTS idx_apv_flow_edge__source_node_id ON apv_flow_edge(source_node_id);
CREATE INDEX IF NOT EXISTS idx_apv_flow_edge__target_node_id ON apv_flow_edge(target_node_id);

--------------------------------------------------------------------------------
-- Form Field Definition
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS apv_flow_form_field (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_form_field PRIMARY KEY,
    flow_version_id VARCHAR(32) NOT NULL,
    name VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    label VARCHAR(128) NOT NULL,
    placeholder VARCHAR(256),
    default_value TEXT,
    is_required BOOLEAN NOT NULL DEFAULT 0,
    is_readonly BOOLEAN NOT NULL DEFAULT 0,
    validation TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    meta TEXT,
    CONSTRAINT uk_apv_flow_form_field__flow_version_id_name UNIQUE (flow_version_id, name),
    CONSTRAINT fk_apv_flow_form_field__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE CASCADE ON UPDATE CASCADE
);

--------------------------------------------------------------------------------
-- Runtime Tables
--------------------------------------------------------------------------------

-- Flow instance
CREATE TABLE IF NOT EXISTS apv_instance (
    id VARCHAR(32) CONSTRAINT pk_apv_instance PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    tenant_id VARCHAR(32) NOT NULL,
    flow_id VARCHAR(32) NOT NULL,
    flow_version_id VARCHAR(32) NOT NULL,
    -- Application info
    title VARCHAR(256) NOT NULL,
    instance_no VARCHAR(64) NOT NULL,
    applicant_id VARCHAR(32) NOT NULL,
    applicant_name VARCHAR(128) NOT NULL DEFAULT '',
    applicant_dept_id VARCHAR(32),
    applicant_dept_name VARCHAR(128),
    -- Status info
    status VARCHAR(16) NOT NULL DEFAULT 'running',
    current_node_id VARCHAR(32),
    finished_at TIMESTAMP,
    -- Business association
    business_record_id VARCHAR(128),
    -- Form data
    form_data TEXT,
    CONSTRAINT fk_apv_instance__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_instance__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT uk_apv_instance__instance_no UNIQUE (instance_no)
);

CREATE INDEX IF NOT EXISTS idx_apv_instance__tenant_id ON apv_instance(tenant_id);
CREATE INDEX IF NOT EXISTS idx_apv_instance__tenant_id_status_created_at ON apv_instance(tenant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_apv_instance__tenant_id_applicant_id_status ON apv_instance(tenant_id, applicant_id, status);
CREATE INDEX IF NOT EXISTS idx_apv_instance__flow_id_status_created_at ON apv_instance(flow_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_apv_instance__applicant_id_status_created_at ON apv_instance(applicant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_apv_instance__current_node_id ON apv_instance(current_node_id);

-- Approval task
CREATE TABLE IF NOT EXISTS apv_task (
    id VARCHAR(32) CONSTRAINT pk_apv_task PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    tenant_id VARCHAR(32) NOT NULL,
    instance_id VARCHAR(32) NOT NULL,
    node_id VARCHAR(32) NOT NULL,
    -- Assignee info
    assignee_id VARCHAR(32) NOT NULL CONSTRAINT ck_apv_task__assignee_id_not_empty CHECK (TRIM(assignee_id) <> ''),
    assignee_name VARCHAR(128) NOT NULL DEFAULT '',
    delegator_id VARCHAR(32),
    delegator_name VARCHAR(128),
    sort_order INTEGER NOT NULL DEFAULT 0,
    -- Task status
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    read_at TIMESTAMP,
    -- Dynamic addition source
    parent_task_id VARCHAR(32),
    add_assignee_type VARCHAR(16),
    -- Timeout info
    deadline TIMESTAMP,
    is_timeout BOOLEAN NOT NULL DEFAULT 0,
    is_pre_warning_sent BOOLEAN NOT NULL DEFAULT 0,
    -- Time record
    finished_at TIMESTAMP,
    CONSTRAINT fk_apv_task__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_task__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_task__parent_task_id FOREIGN KEY (parent_task_id) REFERENCES apv_task(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_task__tenant_id ON apv_task(tenant_id);
CREATE INDEX IF NOT EXISTS idx_apv_task__tenant_id_assignee_id_status ON apv_task(tenant_id, assignee_id, status);
CREATE INDEX IF NOT EXISTS idx_apv_task__instance_id_node_id_status ON apv_task(instance_id, node_id, status);
CREATE INDEX IF NOT EXISTS idx_apv_task__assignee_id_status_created_at ON apv_task(assignee_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_apv_task__instance_id_status_assignee_id ON apv_task(instance_id, status, assignee_id);
CREATE INDEX IF NOT EXISTS idx_apv_task__deadline_active ON apv_task(deadline) WHERE deadline IS NOT NULL AND is_timeout = 0 AND status IN ('pending', 'waiting');
CREATE UNIQUE INDEX IF NOT EXISTS uk_apv_task__instance_id_node_id_assignee_id_active ON apv_task(instance_id, node_id, assignee_id) WHERE status IN ('pending', 'waiting');

-- Action log
CREATE TABLE IF NOT EXISTS apv_action_log (
    id VARCHAR(32) CONSTRAINT pk_apv_action_log PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    instance_id VARCHAR(32) NOT NULL,
    node_id VARCHAR(32),
    task_id VARCHAR(32),
    -- Action info
    action VARCHAR(16) NOT NULL,
    operator_id VARCHAR(32) NOT NULL,
    operator_name VARCHAR(128) NOT NULL,
    operator_dept_id VARCHAR(32),
    operator_dept_name VARCHAR(128),
    ip_address VARCHAR(64),
    user_agent VARCHAR(512),
    opinion TEXT,
    meta TEXT,
    -- Transfer/rollback info
    transfer_to_id VARCHAR(32),
    transfer_to_name VARCHAR(128),
    rollback_to_node_id VARCHAR(32),
    -- Dynamic assignee info
    add_assignee_type VARCHAR(16),
    added_assignee_ids TEXT NOT NULL DEFAULT '[]',
    removed_assignee_ids TEXT NOT NULL DEFAULT '[]',
    -- CC info
    cc_user_ids TEXT NOT NULL DEFAULT '[]',
    -- Attachments
    attachments TEXT,
    CONSTRAINT fk_apv_action_log__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_action_log__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_action_log__task_id FOREIGN KEY (task_id) REFERENCES apv_task(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_action_log__operator_id ON apv_action_log(operator_id);
CREATE INDEX IF NOT EXISTS idx_apv_action_log__instance_id_created_at ON apv_action_log(instance_id, created_at);

-- Parallel approval record
CREATE TABLE IF NOT EXISTS apv_parallel_record (
    id VARCHAR(32) CONSTRAINT pk_apv_parallel_record PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    instance_id VARCHAR(32) NOT NULL,
    node_id VARCHAR(32) NOT NULL,
    task_id VARCHAR(32) NOT NULL,
    assignee_id VARCHAR(32) NOT NULL,
    decision VARCHAR(16),
    opinion TEXT,
    CONSTRAINT fk_apv_parallel_record__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- For parallel pass-rule evaluation: count results per node
CREATE INDEX IF NOT EXISTS idx_apv_parallel_record__instance_id_node_id ON apv_parallel_record(instance_id, node_id);
-- For looking up individual vote by task
CREATE INDEX IF NOT EXISTS idx_apv_parallel_record__instance_id_task_id ON apv_parallel_record(instance_id, task_id);

-- CC record
CREATE TABLE IF NOT EXISTS apv_cc_record (
    id VARCHAR(32) CONSTRAINT pk_apv_cc_record PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    instance_id VARCHAR(32) NOT NULL,
    node_id VARCHAR(32),
    task_id VARCHAR(32),
    cc_user_id VARCHAR(32) NOT NULL,
    cc_user_name VARCHAR(128) NOT NULL DEFAULT '',
    is_manual BOOLEAN NOT NULL DEFAULT 0,
    read_at TIMESTAMP,
    CONSTRAINT fk_apv_cc_record__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_cc_record__instance_id ON apv_cc_record(instance_id);
CREATE INDEX IF NOT EXISTS idx_apv_cc_record__cc_user_id_read_at ON apv_cc_record(cc_user_id, read_at);
CREATE UNIQUE INDEX IF NOT EXISTS uk_apv_cc_record__instance_id_node_id_cc_user_id ON apv_cc_record(instance_id, node_id, cc_user_id) WHERE node_id IS NOT NULL;

--------------------------------------------------------------------------------
-- Extension Tables
--------------------------------------------------------------------------------

-- Approval delegation
CREATE TABLE IF NOT EXISTS apv_delegation (
    id VARCHAR(32) CONSTRAINT pk_apv_delegation PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    delegator_id VARCHAR(32) NOT NULL,
    delegatee_id VARCHAR(32) NOT NULL,
    flow_category_id VARCHAR(32),
    flow_id VARCHAR(32),
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    reason VARCHAR(256),
    CONSTRAINT fk_apv_delegation__flow_category_id FOREIGN KEY (flow_category_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_delegation__flow_id FOREIGN KEY (flow_id)
        REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT ck_apv_delegation__time_range CHECK (start_time < end_time),
    CONSTRAINT ck_apv_delegation__no_self CHECK (delegator_id != delegatee_id)
);

-- For "my received delegations" query (reserved for future use)
CREATE INDEX IF NOT EXISTS idx_apv_delegation__delegatee_id_is_active_end_time ON apv_delegation(delegatee_id, is_active, end_time);
-- For delegation chain resolution in engine (active use)
CREATE INDEX IF NOT EXISTS idx_apv_delegation__delegator_id_is_active ON apv_delegation(delegator_id, is_active);

-- Form snapshot (for rollback strategies: snapshot/merge)
CREATE TABLE IF NOT EXISTS apv_form_snapshot (
    id VARCHAR(32) CONSTRAINT pk_apv_form_snapshot PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    instance_id VARCHAR(32) NOT NULL,
    node_id VARCHAR(32) NOT NULL,
    form_data TEXT NOT NULL,
    CONSTRAINT fk_apv_form_snapshot__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_form_snapshot__instance_id_node_id ON apv_form_snapshot(instance_id, node_id);

--------------------------------------------------------------------------------
-- Auxiliary Tables
--------------------------------------------------------------------------------

-- Event outbox (optional, for transactional event publishing)
CREATE TABLE IF NOT EXISTS apv_event_outbox (
    id VARCHAR(32) CONSTRAINT pk_apv_event_outbox PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    event_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    payload TEXT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    processed_at TIMESTAMP,
    retry_after TIMESTAMP,
    CONSTRAINT uk_apv_event_outbox__event_id UNIQUE (event_id)
);

CREATE INDEX IF NOT EXISTS idx_apv_event_outbox__relay ON apv_event_outbox(status, retry_after, created_at) WHERE status IN ('pending', 'failed', 'processing');

-- Urge record
CREATE TABLE IF NOT EXISTS apv_urge_record (
    id VARCHAR(32) CONSTRAINT pk_apv_urge_record PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    instance_id VARCHAR(32) NOT NULL,
    node_id VARCHAR(32) NOT NULL,
    task_id VARCHAR(32),
    urger_id VARCHAR(32) NOT NULL,
    urger_name VARCHAR(128) NOT NULL DEFAULT '',
    target_user_id VARCHAR(32) NOT NULL,
    target_user_name VARCHAR(128) NOT NULL DEFAULT '',
    message TEXT NOT NULL,
    CONSTRAINT fk_apv_urge_record__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_urge_record__task_id_urger_id_created_at ON apv_urge_record(task_id, urger_id, created_at);
CREATE INDEX IF NOT EXISTS idx_apv_urge_record__instance_id ON apv_urge_record(instance_id);

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
    labels TEXT,
    -- Data binding
    binding_mode VARCHAR(16) NOT NULL DEFAULT 'standalone',
    business_binding TEXT,
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
    form_fields TEXT,
    -- Publish info
    published_at TIMESTAMP,
    published_by VARCHAR(32),
    business_binding TEXT,
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
    pass_ratio REAL NOT NULL DEFAULT 100.00 CONSTRAINT ck_apv_flow_node__pass_ratio CHECK (pass_ratio >= 0 AND pass_ratio <= 100),
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
    -- Composed edge id (xy-edge__<source><handle>-<target>): holds two node keys, not one.
    key VARCHAR(256),
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
    flow_code VARCHAR(64) NOT NULL,
    flow_version_id VARCHAR(32) NOT NULL,
    -- Application info
    title VARCHAR(256) NOT NULL,
    instance_no VARCHAR(64) NOT NULL,
    applicant_id VARCHAR(32) NOT NULL,
    applicant_name VARCHAR(128) NOT NULL DEFAULT '',
    applicant_department_id VARCHAR(32),
    applicant_department_name VARCHAR(128),
    -- Status info
    status VARCHAR(16) NOT NULL DEFAULT 'running',
    current_node_id VARCHAR(32),
    finished_at TIMESTAMP,
    -- Business association
    business_ref VARCHAR(512),
    -- Form data
    form_data TEXT,
    -- Host-supplied global variables snapshotted at instance start
    globals TEXT,
    business_projection_id VARCHAR(32),
    CONSTRAINT fk_apv_instance__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_instance__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT uk_apv_instance__instance_no UNIQUE (instance_no)
);

CREATE INDEX IF NOT EXISTS idx_apv_instance__tenant_id ON apv_instance(tenant_id);
CREATE INDEX IF NOT EXISTS idx_apv_instance__business_ref ON apv_instance(business_ref);
CREATE INDEX IF NOT EXISTS idx_apv_instance__business_projection_id ON apv_instance(business_projection_id);
CREATE INDEX IF NOT EXISTS idx_apv_instance__tenant_id_status_created_at ON apv_instance(tenant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_apv_instance__tenant_id_applicant_id_status ON apv_instance(tenant_id, applicant_id, status);
CREATE INDEX IF NOT EXISTS idx_apv_instance__flow_id_status_created_at ON apv_instance(flow_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_apv_instance__applicant_id_status_created_at ON apv_instance(applicant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_apv_instance__current_node_id ON apv_instance(current_node_id);

-- Durable desired state for one business-table target
CREATE TABLE IF NOT EXISTS apv_business_projection (
    id VARCHAR(32) CONSTRAINT pk_apv_business_projection PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    updated_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    tenant_id VARCHAR(32) NOT NULL,
    flow_id VARCHAR(32) NOT NULL,
    flow_version_id VARCHAR(32) NOT NULL,
    owner_instance_id VARCHAR(32) NOT NULL,
    applied_owner_instance_id VARCHAR(32),
    target_hash VARCHAR(64) NOT NULL,
    consistency VARCHAR(16) NOT NULL,
    binding TEXT NOT NULL,
    record_key TEXT NOT NULL,
    desired_status VARCHAR(16) NOT NULL,
    desired_started_at TIMESTAMP NOT NULL,
    desired_finished_at TIMESTAMP,
    desired_revision INTEGER NOT NULL,
    applied_revision INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP,
    lease_until TIMESTAMP,
    last_error TEXT,
    applied_at TIMESTAMP,
    CONSTRAINT uk_apv_business_projection__target_hash UNIQUE (target_hash)
);

CREATE INDEX IF NOT EXISTS idx_apv_business_projection__tenant_id_status ON apv_business_projection(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_apv_business_projection__status_next_attempt_at ON apv_business_projection(status, next_attempt_at);
CREATE INDEX IF NOT EXISTS idx_apv_business_projection__owner_instance_id ON apv_business_projection(owner_instance_id);

-- Node visit (one traversal of a flow node by an instance; the engine begins
-- a visit on node entry and stamps the outcome when the node concludes)
CREATE TABLE IF NOT EXISTS apv_node_visit (
    id VARCHAR(32) CONSTRAINT pk_apv_node_visit PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    tenant_id VARCHAR(32) NOT NULL,
    instance_id VARCHAR(32) NOT NULL,
    node_id VARCHAR(32) NOT NULL,
    sequence INTEGER NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    finished_at TIMESTAMP,
    CONSTRAINT fk_apv_node_visit__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_node_visit__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT uk_apv_node_visit__instance_id_sequence UNIQUE (instance_id, sequence)
);

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
    visit_id VARCHAR(32) NOT NULL,
    -- Assignee info
    assignee_id VARCHAR(32) NOT NULL CONSTRAINT ck_apv_task__assignee_id_not_empty CHECK (TRIM(assignee_id) <> ''),
    assignee_name VARCHAR(128) NOT NULL DEFAULT '',
    assignee_department_id VARCHAR(32),
    assignee_department_name VARCHAR(128),
    delegator_id VARCHAR(32),
    delegator_name VARCHAR(128),
    delegator_department_id VARCHAR(32),
    delegator_department_name VARCHAR(128),
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
    CONSTRAINT fk_apv_task__visit_id FOREIGN KEY (visit_id) REFERENCES apv_node_visit(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_task__parent_task_id FOREIGN KEY (parent_task_id) REFERENCES apv_task(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_task__tenant_id ON apv_task(tenant_id);
CREATE INDEX IF NOT EXISTS idx_apv_task__tenant_id_assignee_id_status ON apv_task(tenant_id, assignee_id, status);
CREATE INDEX IF NOT EXISTS idx_apv_task__instance_id_node_id_status ON apv_task(instance_id, node_id, status);
CREATE INDEX IF NOT EXISTS idx_apv_task__assignee_id_status_created_at ON apv_task(assignee_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_apv_task__assignee_id_status_finished_at ON apv_task(assignee_id, status, finished_at);
CREATE INDEX IF NOT EXISTS idx_apv_task__instance_id_status_assignee_id ON apv_task(instance_id, status, assignee_id);
CREATE INDEX IF NOT EXISTS idx_apv_task__visit_id ON apv_task(visit_id);
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
    operator_department_id VARCHAR(32),
    operator_department_name VARCHAR(128),
    ip_address VARCHAR(64),
    user_agent VARCHAR(512),
    opinion TEXT,
    meta TEXT,
    -- Transfer/rollback info
    transfer_to_id VARCHAR(32),
    transfer_to_name VARCHAR(128),
    transfer_to_department_id VARCHAR(32),
    transfer_to_department_name VARCHAR(128),
    rollback_to_node_id VARCHAR(32),
    -- Person lists as UserBrief object arrays: [{id, name, departmentId, departmentName}]
    add_assignee_type VARCHAR(16),
    added_assignees TEXT NOT NULL DEFAULT '[]',
    removed_assignees TEXT NOT NULL DEFAULT '[]',
    cc_users TEXT NOT NULL DEFAULT '[]',
    -- Attachments
    attachments TEXT,
    CONSTRAINT fk_apv_action_log__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_action_log__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_action_log__task_id FOREIGN KEY (task_id) REFERENCES apv_task(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_action_log__operator_id ON apv_action_log(operator_id);
CREATE INDEX IF NOT EXISTS idx_apv_action_log__instance_id_created_at ON apv_action_log(instance_id, created_at);

-- CC record
CREATE TABLE IF NOT EXISTS apv_cc_record (
    id VARCHAR(32) CONSTRAINT pk_apv_cc_record PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    instance_id VARCHAR(32) NOT NULL,
    node_id VARCHAR(32),
    -- One node traversal owns each record; nil for instance-level records
    visit_id VARCHAR(32),
    task_id VARCHAR(32),
    cc_user_id VARCHAR(32) NOT NULL,
    cc_user_name VARCHAR(128) NOT NULL DEFAULT '',
    cc_user_department_id VARCHAR(32),
    cc_user_department_name VARCHAR(128),
    is_manual BOOLEAN NOT NULL DEFAULT 0,
    read_at TIMESTAMP,
    CONSTRAINT fk_apv_cc_record__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_cc_record__instance_id ON apv_cc_record(instance_id);
CREATE INDEX IF NOT EXISTS idx_apv_cc_record__cc_user_id_read_at ON apv_cc_record(cc_user_id, read_at);
CREATE UNIQUE INDEX IF NOT EXISTS uk_apv_cc_record__visit_id_cc_user_id ON apv_cc_record(visit_id, cc_user_id) WHERE visit_id IS NOT NULL;

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
    starts_at TIMESTAMP NOT NULL,
    ends_at TIMESTAMP NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    reason VARCHAR(256),
    CONSTRAINT fk_apv_delegation__flow_category_id FOREIGN KEY (flow_category_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_delegation__flow_id FOREIGN KEY (flow_id)
        REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT ck_apv_delegation__time_range CHECK (starts_at < ends_at),
    CONSTRAINT ck_apv_delegation__no_self CHECK (delegator_id != delegatee_id)
);

-- For "my received delegations" query (reserved for future use)
CREATE INDEX IF NOT EXISTS idx_apv_delegation__delegatee_id_is_active_ends_at ON apv_delegation(delegatee_id, is_active, ends_at);
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

-- (apv_event_outbox removed; framework now provides a generic outbox
-- via internal/event/transport/outbox / sys_event_outbox.)

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
    urger_department_id VARCHAR(32),
    urger_department_name VARCHAR(128),
    target_user_id VARCHAR(32) NOT NULL,
    target_user_name VARCHAR(128) NOT NULL DEFAULT '',
    target_user_department_id VARCHAR(32),
    target_user_department_name VARCHAR(128),
    message TEXT NOT NULL,
    CONSTRAINT fk_apv_urge_record__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_urge_record__task_id_urger_id_created_at ON apv_urge_record(task_id, urger_id, created_at);
CREATE INDEX IF NOT EXISTS idx_apv_urge_record__instance_id ON apv_urge_record(instance_id);

--------------------------------------------------------------------------------
-- Table-Storage Metadata (StorageTable mode)
--------------------------------------------------------------------------------

-- Form table: one physical form table per published version
CREATE TABLE IF NOT EXISTS apv_form_table (
    id VARCHAR(32) CONSTRAINT pk_apv_form_table PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    flow_id VARCHAR(32) NOT NULL,
    version_id VARCHAR(32) NOT NULL,
    physical_table_name VARCHAR(64) NOT NULL,
    -- Detail-table field this child table projects; '' = the main table
    source_field_key VARCHAR(64) NOT NULL DEFAULT '',
    CONSTRAINT fk_apv_form_table__version_id FOREIGN KEY (version_id) REFERENCES apv_flow_version(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_apv_form_table__version_id_source_field_key ON apv_form_table(version_id, source_field_key);

-- Form table column: column definitions projected from the form schema
CREATE TABLE IF NOT EXISTS apv_form_table_column (
    id VARCHAR(32) CONSTRAINT pk_apv_form_table_column PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime')),
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    form_table_id VARCHAR(32) NOT NULL,
    column_name VARCHAR(64) NOT NULL,
    column_type VARCHAR(32) NOT NULL,
    is_nullable BOOLEAN NOT NULL DEFAULT 1,
    source_field_key VARCHAR(64),
    sort_order INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT fk_apv_form_table_column__form_table_id FOREIGN KEY (form_table_id) REFERENCES apv_form_table(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apv_form_table_column__form_table_id ON apv_form_table_column(form_table_id);

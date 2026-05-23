--------------------------------------------------------------------------------
-- Flow Definition Tables
--------------------------------------------------------------------------------

-- Flow category
CREATE TABLE IF NOT EXISTS apv_flow_category (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_category PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    tenant_id VARCHAR(32) NOT NULL,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    icon VARCHAR(128),
    parent_id VARCHAR(32),
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    remark VARCHAR(256),
    CONSTRAINT uk_apv_flow_category__tenant_id_code UNIQUE (tenant_id, code),
    CONSTRAINT fk_apv_flow_category__parent_id FOREIGN KEY (parent_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

COMMENT ON TABLE apv_flow_category IS 'Flow Category';
COMMENT ON COLUMN apv_flow_category.id IS 'ID';
COMMENT ON COLUMN apv_flow_category.created_at IS 'Created';
COMMENT ON COLUMN apv_flow_category.updated_at IS 'Updated';
COMMENT ON COLUMN apv_flow_category.created_by IS 'Creator';
COMMENT ON COLUMN apv_flow_category.updated_by IS 'Updater';
COMMENT ON COLUMN apv_flow_category.tenant_id IS 'Tenant';
COMMENT ON COLUMN apv_flow_category.code IS 'Code';
COMMENT ON COLUMN apv_flow_category.name IS 'Name';
COMMENT ON COLUMN apv_flow_category.icon IS 'Icon';
COMMENT ON COLUMN apv_flow_category.parent_id IS 'Parent';
COMMENT ON COLUMN apv_flow_category.sort_order IS 'Sort';
COMMENT ON COLUMN apv_flow_category.is_active IS 'Active';
COMMENT ON COLUMN apv_flow_category.remark IS 'Remark';

CREATE INDEX idx_apv_flow_category__tenant_id ON apv_flow_category(tenant_id);
CREATE INDEX idx_apv_flow_category__parent_id ON apv_flow_category(parent_id);

-- Flow definition
CREATE TABLE IF NOT EXISTS apv_flow (
    id VARCHAR(32) CONSTRAINT pk_apv_flow PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
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
    admin_user_ids JSONB NOT NULL DEFAULT '[]',
    is_all_initiation_allowed BOOLEAN NOT NULL DEFAULT true,
    -- Other
    instance_title_template VARCHAR(256) NOT NULL DEFAULT '{{.flowName}}-{{.instanceNo}}',
    is_active BOOLEAN NOT NULL DEFAULT false,
    current_version INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT uk_apv_flow__tenant_id_code UNIQUE (tenant_id, code),
    CONSTRAINT fk_apv_flow__category_id FOREIGN KEY (category_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

COMMENT ON TABLE apv_flow IS 'Flow';
COMMENT ON COLUMN apv_flow.id IS 'ID';
COMMENT ON COLUMN apv_flow.created_at IS 'Created';
COMMENT ON COLUMN apv_flow.updated_at IS 'Updated';
COMMENT ON COLUMN apv_flow.created_by IS 'Creator';
COMMENT ON COLUMN apv_flow.updated_by IS 'Updater';
COMMENT ON COLUMN apv_flow.tenant_id IS 'Tenant';
COMMENT ON COLUMN apv_flow.category_id IS 'Category';
COMMENT ON COLUMN apv_flow.code IS 'Code';
COMMENT ON COLUMN apv_flow.name IS 'Name';
COMMENT ON COLUMN apv_flow.icon IS 'Icon';
COMMENT ON COLUMN apv_flow.description IS 'Description';
COMMENT ON COLUMN apv_flow.binding_mode IS 'Binding Mode';
COMMENT ON COLUMN apv_flow.business_table IS 'Biz Table';
COMMENT ON COLUMN apv_flow.business_pk_field IS 'Biz PK';
COMMENT ON COLUMN apv_flow.business_title_field IS 'Title Field';
COMMENT ON COLUMN apv_flow.business_status_field IS 'Status Field';
COMMENT ON COLUMN apv_flow.admin_user_ids IS 'Admins';
COMMENT ON COLUMN apv_flow.is_all_initiation_allowed IS 'Open Start';
COMMENT ON COLUMN apv_flow.instance_title_template IS 'Title Template';
COMMENT ON COLUMN apv_flow.is_active IS 'Active';
COMMENT ON COLUMN apv_flow.current_version IS 'Version';

CREATE INDEX idx_apv_flow__category_id ON apv_flow(category_id);
CREATE INDEX idx_apv_flow__tenant_id ON apv_flow(tenant_id);

-- Flow initiator config
CREATE TABLE IF NOT EXISTS apv_flow_initiator (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_initiator PRIMARY KEY,
    flow_id VARCHAR(32) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    ids JSONB NOT NULL DEFAULT '[]',
    CONSTRAINT fk_apv_flow_initiator__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE CASCADE ON UPDATE CASCADE
);

COMMENT ON TABLE apv_flow_initiator IS 'Initiator';
COMMENT ON COLUMN apv_flow_initiator.id IS 'ID';
COMMENT ON COLUMN apv_flow_initiator.flow_id IS 'Flow';
COMMENT ON COLUMN apv_flow_initiator.kind IS 'Kind';
COMMENT ON COLUMN apv_flow_initiator.ids IS 'Subjects';

CREATE INDEX idx_apv_flow_initiator__flow_id ON apv_flow_initiator(flow_id);

-- Flow version
CREATE TABLE IF NOT EXISTS apv_flow_version (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_version PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',

    flow_id VARCHAR(32) NOT NULL,
    version INTEGER NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'draft',
    description VARCHAR(256),
    -- Design data
    storage_mode VARCHAR(8) NOT NULL DEFAULT 'json',
    flow_schema JSONB,
    form_schema JSONB,
    -- Publish info
    published_at TIMESTAMP,
    published_by VARCHAR(32),
    CONSTRAINT uk_apv_flow_version__flow_id_version UNIQUE (flow_id, version),
    CONSTRAINT fk_apv_flow_version__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

COMMENT ON TABLE apv_flow_version IS 'Version';
COMMENT ON COLUMN apv_flow_version.id IS 'ID';
COMMENT ON COLUMN apv_flow_version.created_at IS 'Created';
COMMENT ON COLUMN apv_flow_version.updated_at IS 'Updated';
COMMENT ON COLUMN apv_flow_version.created_by IS 'Creator';
COMMENT ON COLUMN apv_flow_version.updated_by IS 'Updater';
COMMENT ON COLUMN apv_flow_version.flow_id IS 'Flow';
COMMENT ON COLUMN apv_flow_version.version IS 'Version';
COMMENT ON COLUMN apv_flow_version.status IS 'Status';
COMMENT ON COLUMN apv_flow_version.description IS 'Description';
COMMENT ON COLUMN apv_flow_version.storage_mode IS 'Storage Mode';
COMMENT ON COLUMN apv_flow_version.flow_schema IS 'Flow Schema';
COMMENT ON COLUMN apv_flow_version.form_schema IS 'Form Schema';
COMMENT ON COLUMN apv_flow_version.published_at IS 'Published';
COMMENT ON COLUMN apv_flow_version.published_by IS 'Publisher';

CREATE INDEX idx_apv_flow_version__flow_id_status ON apv_flow_version(flow_id, status);
-- Ensure at most one published version per flow
CREATE UNIQUE INDEX uk_apv_flow_version__flow_id_published ON apv_flow_version(flow_id) WHERE status = 'published';

-- Flow node
CREATE TABLE IF NOT EXISTS apv_flow_node (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_node PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
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
    pass_ratio NUMERIC(3,2) NOT NULL DEFAULT 1.00 CONSTRAINT ck_apv_flow_node__pass_ratio CHECK (pass_ratio >= 0 AND pass_ratio <= 1),
    -- Empty assignee config
    empty_assignee_action VARCHAR(32) NOT NULL DEFAULT 'auto_pass',
    fallback_user_ids JSONB NOT NULL DEFAULT '[]',
    admin_user_ids JSONB NOT NULL DEFAULT '[]',
    same_applicant_action VARCHAR(32) NOT NULL DEFAULT 'self_approve',
    -- Rollback config
    is_rollback_allowed BOOLEAN NOT NULL DEFAULT true,
    rollback_type VARCHAR(16) NOT NULL DEFAULT 'previous',
    rollback_data_strategy VARCHAR(16),
    rollback_target_keys JSONB,
    -- Dynamic assignee config
    is_add_assignee_allowed BOOLEAN NOT NULL DEFAULT true,
    add_assignee_types JSONB NOT NULL DEFAULT '["before", "after", "parallel"]',
    is_remove_assignee_allowed BOOLEAN NOT NULL DEFAULT true,
    -- Field permissions config
    field_permissions JSONB NOT NULL DEFAULT '{}',
    -- CC config
    is_manual_cc_allowed BOOLEAN NOT NULL DEFAULT true,
    -- Other config
    is_transfer_allowed BOOLEAN NOT NULL DEFAULT true,
    is_opinion_required BOOLEAN NOT NULL DEFAULT false,
    timeout_hours INTEGER NOT NULL DEFAULT 0 CONSTRAINT ck_apv_flow_node__timeout_hours CHECK (timeout_hours >= 0),
    timeout_action VARCHAR(16) NOT NULL DEFAULT 'none',
    timeout_notify_before_hours INTEGER NOT NULL DEFAULT 0 CONSTRAINT ck_apv_flow_node__timeout_notify_before_hours CHECK (timeout_notify_before_hours >= 0),
    urge_cooldown_minutes INTEGER NOT NULL DEFAULT 0 CONSTRAINT ck_apv_flow_node__urge_cooldown_minutes CHECK (urge_cooldown_minutes >= 0),
    -- Advanced config
    consecutive_approver_action VARCHAR(32) NOT NULL DEFAULT 'none',
    is_read_confirm_required BOOLEAN NOT NULL DEFAULT false,
    branches JSONB,
    CONSTRAINT uk_apv_flow_node__flow_version_id_key UNIQUE (flow_version_id, key),
    CONSTRAINT fk_apv_flow_node__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE CASCADE ON UPDATE CASCADE
);

COMMENT ON TABLE apv_flow_node IS 'Node';
COMMENT ON COLUMN apv_flow_node.id IS 'ID';
COMMENT ON COLUMN apv_flow_node.created_at IS 'Created';
COMMENT ON COLUMN apv_flow_node.updated_at IS 'Updated';
COMMENT ON COLUMN apv_flow_node.created_by IS 'Creator';
COMMENT ON COLUMN apv_flow_node.updated_by IS 'Updater';
COMMENT ON COLUMN apv_flow_node.flow_version_id IS 'Version';
COMMENT ON COLUMN apv_flow_node.key IS 'Key';
COMMENT ON COLUMN apv_flow_node.kind IS 'Kind';
COMMENT ON COLUMN apv_flow_node.name IS 'Name';
COMMENT ON COLUMN apv_flow_node.description IS 'Description';
COMMENT ON COLUMN apv_flow_node.execution_type IS 'Exec Type';
COMMENT ON COLUMN apv_flow_node.approval_method IS 'Method';
COMMENT ON COLUMN apv_flow_node.pass_rule IS 'Pass Rule';
COMMENT ON COLUMN apv_flow_node.pass_ratio IS 'Pass Ratio';
COMMENT ON COLUMN apv_flow_node.empty_assignee_action IS 'Empty Action';
COMMENT ON COLUMN apv_flow_node.fallback_user_ids IS 'Fallbacks';
COMMENT ON COLUMN apv_flow_node.admin_user_ids IS 'Admins';
COMMENT ON COLUMN apv_flow_node.same_applicant_action IS 'Self Action';
COMMENT ON COLUMN apv_flow_node.is_rollback_allowed IS 'Rollback';
COMMENT ON COLUMN apv_flow_node.rollback_type IS 'Rollback Type';
COMMENT ON COLUMN apv_flow_node.rollback_data_strategy IS 'Rollback Data';
COMMENT ON COLUMN apv_flow_node.rollback_target_keys IS 'Rollback Targets';
COMMENT ON COLUMN apv_flow_node.is_add_assignee_allowed IS 'Add Allowed';
COMMENT ON COLUMN apv_flow_node.add_assignee_types IS 'Add Types';
COMMENT ON COLUMN apv_flow_node.is_remove_assignee_allowed IS 'Remove Allowed';
COMMENT ON COLUMN apv_flow_node.field_permissions IS 'Field Perms';
COMMENT ON COLUMN apv_flow_node.is_manual_cc_allowed IS 'Manual CC';
COMMENT ON COLUMN apv_flow_node.is_transfer_allowed IS 'Transfer';
COMMENT ON COLUMN apv_flow_node.is_opinion_required IS 'Opinion';
COMMENT ON COLUMN apv_flow_node.timeout_hours IS 'Timeout';
COMMENT ON COLUMN apv_flow_node.timeout_action IS 'Timeout Action';
COMMENT ON COLUMN apv_flow_node.timeout_notify_before_hours IS 'Notify Before';
COMMENT ON COLUMN apv_flow_node.urge_cooldown_minutes IS 'Urge Cooldown';
COMMENT ON COLUMN apv_flow_node.consecutive_approver_action IS 'Consecutive';
COMMENT ON COLUMN apv_flow_node.is_read_confirm_required IS 'Read Confirm';
COMMENT ON COLUMN apv_flow_node.branches IS 'Branches';

-- Node assignee config
CREATE TABLE IF NOT EXISTS apv_flow_node_assignee (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_node_assignee PRIMARY KEY,
    node_id VARCHAR(32) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    ids JSONB NOT NULL DEFAULT '[]',
    form_field VARCHAR(64),
    sort_order INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT fk_apv_flow_node_assignee__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE
);

COMMENT ON TABLE apv_flow_node_assignee IS 'Assignee';
COMMENT ON COLUMN apv_flow_node_assignee.id IS 'ID';
COMMENT ON COLUMN apv_flow_node_assignee.node_id IS 'Node';
COMMENT ON COLUMN apv_flow_node_assignee.kind IS 'Kind';
COMMENT ON COLUMN apv_flow_node_assignee.ids IS 'Subjects';
COMMENT ON COLUMN apv_flow_node_assignee.form_field IS 'Form Field';
COMMENT ON COLUMN apv_flow_node_assignee.sort_order IS 'Sort';

CREATE INDEX idx_apv_flow_node_assignee__node_id ON apv_flow_node_assignee(node_id);

-- Node CC config
CREATE TABLE IF NOT EXISTS apv_flow_node_cc (
    id VARCHAR(32) CONSTRAINT pk_apv_flow_node_cc PRIMARY KEY,
    node_id VARCHAR(32) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    ids JSONB NOT NULL DEFAULT '[]',
    form_field VARCHAR(64),
    timing VARCHAR(16) NOT NULL DEFAULT 'always',
    CONSTRAINT fk_apv_flow_node_cc__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE
);

COMMENT ON TABLE apv_flow_node_cc IS 'Node CC';
COMMENT ON COLUMN apv_flow_node_cc.id IS 'ID';
COMMENT ON COLUMN apv_flow_node_cc.node_id IS 'Node';
COMMENT ON COLUMN apv_flow_node_cc.kind IS 'Kind';
COMMENT ON COLUMN apv_flow_node_cc.ids IS 'Subjects';
COMMENT ON COLUMN apv_flow_node_cc.form_field IS 'Form Field';
COMMENT ON COLUMN apv_flow_node_cc.timing IS 'Timing';

CREATE INDEX idx_apv_flow_node_cc__node_id ON apv_flow_node_cc(node_id);

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

COMMENT ON TABLE apv_flow_edge IS 'Edge';
COMMENT ON COLUMN apv_flow_edge.id IS 'ID';
COMMENT ON COLUMN apv_flow_edge.flow_version_id IS 'Version';
COMMENT ON COLUMN apv_flow_edge.key IS 'Key';
COMMENT ON COLUMN apv_flow_edge.source_node_id IS 'Source';
COMMENT ON COLUMN apv_flow_edge.source_node_key IS 'Source Key';
COMMENT ON COLUMN apv_flow_edge.target_node_id IS 'Target';
COMMENT ON COLUMN apv_flow_edge.target_node_key IS 'Target Key';
COMMENT ON COLUMN apv_flow_edge.source_handle IS 'Handle';

CREATE INDEX idx_apv_flow_edge__flow_version_id_source_node_id ON apv_flow_edge(flow_version_id, source_node_id);
CREATE INDEX idx_apv_flow_edge__source_node_id ON apv_flow_edge(source_node_id);
CREATE INDEX idx_apv_flow_edge__target_node_id ON apv_flow_edge(target_node_id);

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
    is_required BOOLEAN NOT NULL DEFAULT false,
    is_readonly BOOLEAN NOT NULL DEFAULT false,
    validation JSONB,
    sort_order INTEGER NOT NULL DEFAULT 0,
    meta JSONB,
    CONSTRAINT uk_apv_flow_form_field__flow_version_id_name UNIQUE (flow_version_id, name),
    CONSTRAINT fk_apv_flow_form_field__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE CASCADE ON UPDATE CASCADE
);

COMMENT ON TABLE apv_flow_form_field IS 'Form Field';
COMMENT ON COLUMN apv_flow_form_field.id IS 'ID';
COMMENT ON COLUMN apv_flow_form_field.flow_version_id IS 'Version';
COMMENT ON COLUMN apv_flow_form_field.name IS 'Name';
COMMENT ON COLUMN apv_flow_form_field.kind IS 'Kind';
COMMENT ON COLUMN apv_flow_form_field.label IS 'Label';
COMMENT ON COLUMN apv_flow_form_field.placeholder IS 'Placeholder';
COMMENT ON COLUMN apv_flow_form_field.default_value IS 'Default';
COMMENT ON COLUMN apv_flow_form_field.is_required IS 'Required';
COMMENT ON COLUMN apv_flow_form_field.is_readonly IS 'Readonly';
COMMENT ON COLUMN apv_flow_form_field.validation IS 'Validation';
COMMENT ON COLUMN apv_flow_form_field.sort_order IS 'Sort';
COMMENT ON COLUMN apv_flow_form_field.meta IS 'Meta';

--------------------------------------------------------------------------------
-- Runtime Tables
--------------------------------------------------------------------------------

-- Flow instance
CREATE TABLE IF NOT EXISTS apv_instance (
    id VARCHAR(32) CONSTRAINT pk_apv_instance PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
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
    applicant_department_id VARCHAR(32),
    applicant_department_name VARCHAR(128),
    -- Status info
    status VARCHAR(16) NOT NULL DEFAULT 'running',
    current_node_id VARCHAR(32),
    finished_at TIMESTAMP,
    -- Business association
    business_record_id VARCHAR(128),
    -- Form data
    form_data JSONB,
    CONSTRAINT fk_apv_instance__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_instance__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT uk_apv_instance__instance_no UNIQUE (instance_no)
);

COMMENT ON TABLE apv_instance IS 'Instance';
COMMENT ON COLUMN apv_instance.id IS 'ID';
COMMENT ON COLUMN apv_instance.created_at IS 'Created';
COMMENT ON COLUMN apv_instance.updated_at IS 'Updated';
COMMENT ON COLUMN apv_instance.created_by IS 'Creator';
COMMENT ON COLUMN apv_instance.updated_by IS 'Updater';
COMMENT ON COLUMN apv_instance.tenant_id IS 'Tenant';
COMMENT ON COLUMN apv_instance.flow_id IS 'Flow';
COMMENT ON COLUMN apv_instance.flow_version_id IS 'Version';
COMMENT ON COLUMN apv_instance.title IS 'Title';
COMMENT ON COLUMN apv_instance.instance_no IS 'No.';
COMMENT ON COLUMN apv_instance.applicant_id IS 'Applicant';
COMMENT ON COLUMN apv_instance.applicant_name IS 'Applicant Name';
COMMENT ON COLUMN apv_instance.applicant_department_id IS 'Department';
COMMENT ON COLUMN apv_instance.applicant_department_name IS 'Dept Name';
COMMENT ON COLUMN apv_instance.status IS 'Status';
COMMENT ON COLUMN apv_instance.current_node_id IS 'Current Node';
COMMENT ON COLUMN apv_instance.finished_at IS 'Finished';
COMMENT ON COLUMN apv_instance.business_record_id IS 'Biz Record';
COMMENT ON COLUMN apv_instance.form_data IS 'Form Data';

CREATE INDEX idx_apv_instance__tenant_id ON apv_instance(tenant_id);
CREATE INDEX idx_apv_instance__tenant_id_status_created_at ON apv_instance(tenant_id, status, created_at DESC);
CREATE INDEX idx_apv_instance__tenant_id_applicant_id_status ON apv_instance(tenant_id, applicant_id, status);
CREATE INDEX idx_apv_instance__flow_id_status_created_at ON apv_instance(flow_id, status, created_at);
CREATE INDEX idx_apv_instance__applicant_id_status_created_at ON apv_instance(applicant_id, status, created_at DESC);
CREATE INDEX idx_apv_instance__current_node_id ON apv_instance(current_node_id);
--------------------------------------------------------------------------------
-- Form Data Storage (GIN index for JSON hybrid mode)
--------------------------------------------------------------------------------

-- Create GIN index on form_data JSONB field for efficient queries
CREATE INDEX idx_apv_instance__form_data ON apv_instance USING GIN (form_data);

-- Approval task
CREATE TABLE IF NOT EXISTS apv_task (
    id VARCHAR(32) CONSTRAINT pk_apv_task PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    tenant_id VARCHAR(32) NOT NULL,
    instance_id VARCHAR(32) NOT NULL,
    node_id VARCHAR(32) NOT NULL,
    -- Assignee info
    assignee_id VARCHAR(32) NOT NULL,
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
    is_timeout BOOLEAN NOT NULL DEFAULT false,
    is_pre_warning_sent BOOLEAN NOT NULL DEFAULT false,
    -- Time record
    finished_at TIMESTAMP,
    CONSTRAINT ck_apv_task__assignee_id_not_empty CHECK (btrim(assignee_id) <> ''),
    CONSTRAINT fk_apv_task__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_task__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_task__parent_task_id FOREIGN KEY (parent_task_id) REFERENCES apv_task(id) ON DELETE SET NULL ON UPDATE CASCADE
);

COMMENT ON TABLE apv_task IS 'Task';
COMMENT ON COLUMN apv_task.id IS 'ID';
COMMENT ON COLUMN apv_task.created_at IS 'Created';
COMMENT ON COLUMN apv_task.updated_at IS 'Updated';
COMMENT ON COLUMN apv_task.created_by IS 'Creator';
COMMENT ON COLUMN apv_task.updated_by IS 'Updater';
COMMENT ON COLUMN apv_task.tenant_id IS 'Tenant';
COMMENT ON COLUMN apv_task.instance_id IS 'Instance';
COMMENT ON COLUMN apv_task.node_id IS 'Node';
COMMENT ON COLUMN apv_task.assignee_id IS 'Assignee';
COMMENT ON COLUMN apv_task.assignee_name IS 'Assignee Name';
COMMENT ON COLUMN apv_task.delegator_id IS 'Delegator';
COMMENT ON COLUMN apv_task.delegator_name IS 'Delegator Name';
COMMENT ON COLUMN apv_task.sort_order IS 'Sort';
COMMENT ON COLUMN apv_task.status IS 'Status';
COMMENT ON COLUMN apv_task.read_at IS 'Read';
COMMENT ON COLUMN apv_task.parent_task_id IS 'Parent';
COMMENT ON COLUMN apv_task.add_assignee_type IS 'Add Type';
COMMENT ON COLUMN apv_task.deadline IS 'Deadline';
COMMENT ON COLUMN apv_task.is_timeout IS 'Timeout';
COMMENT ON COLUMN apv_task.is_pre_warning_sent IS 'Pre-warned';
COMMENT ON COLUMN apv_task.finished_at IS 'Finished';

CREATE INDEX idx_apv_task__tenant_id ON apv_task(tenant_id);
CREATE INDEX idx_apv_task__tenant_id_assignee_id_status ON apv_task(tenant_id, assignee_id, status);
CREATE INDEX idx_apv_task__instance_id_node_id_status ON apv_task(instance_id, node_id, status);
CREATE INDEX idx_apv_task__assignee_id_status_created_at ON apv_task(assignee_id, status, created_at);
CREATE INDEX idx_apv_task__instance_id_status_assignee_id ON apv_task(instance_id, status, assignee_id);
CREATE INDEX idx_apv_task__deadline_active ON apv_task(deadline) WHERE deadline IS NOT NULL AND is_timeout = FALSE AND status IN ('pending', 'waiting');
CREATE UNIQUE INDEX uk_apv_task__instance_id_node_id_assignee_id_active ON apv_task(instance_id, node_id, assignee_id) WHERE status IN ('pending', 'waiting');

-- Action log
CREATE TABLE IF NOT EXISTS apv_action_log (
    id VARCHAR(32) CONSTRAINT pk_apv_action_log PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
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
    meta JSONB,
    -- Transfer/rollback info
    transfer_to_id VARCHAR(32),
    transfer_to_name VARCHAR(128),
    rollback_to_node_id VARCHAR(32),
    -- Dynamic assignee info
    add_assignee_type VARCHAR(16),
    added_assignee_ids JSONB NOT NULL DEFAULT '[]',
    removed_assignee_ids JSONB NOT NULL DEFAULT '[]',
    -- CC info
    cc_user_ids JSONB NOT NULL DEFAULT '[]',
    -- Attachments
    attachments JSONB,
    CONSTRAINT fk_apv_action_log__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_action_log__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_action_log__task_id FOREIGN KEY (task_id) REFERENCES apv_task(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

COMMENT ON TABLE apv_action_log IS 'Action Log';
COMMENT ON COLUMN apv_action_log.id IS 'ID';
COMMENT ON COLUMN apv_action_log.created_at IS 'Performed';
COMMENT ON COLUMN apv_action_log.created_by IS 'Creator';
COMMENT ON COLUMN apv_action_log.instance_id IS 'Instance';
COMMENT ON COLUMN apv_action_log.node_id IS 'Node';
COMMENT ON COLUMN apv_action_log.task_id IS 'Task';
COMMENT ON COLUMN apv_action_log.action IS 'Action';
COMMENT ON COLUMN apv_action_log.operator_id IS 'Operator';
COMMENT ON COLUMN apv_action_log.operator_name IS 'Operator Name';
COMMENT ON COLUMN apv_action_log.operator_department_id IS 'Department';
COMMENT ON COLUMN apv_action_log.operator_department_name IS 'Dept Name';
COMMENT ON COLUMN apv_action_log.ip_address IS 'IP';
COMMENT ON COLUMN apv_action_log.user_agent IS 'User Agent';
COMMENT ON COLUMN apv_action_log.opinion IS 'Opinion';
COMMENT ON COLUMN apv_action_log.meta IS 'Meta';
COMMENT ON COLUMN apv_action_log.transfer_to_id IS 'Transferee';
COMMENT ON COLUMN apv_action_log.transfer_to_name IS 'Transferee Name';
COMMENT ON COLUMN apv_action_log.rollback_to_node_id IS 'Rollback Node';
COMMENT ON COLUMN apv_action_log.add_assignee_type IS 'Add Type';
COMMENT ON COLUMN apv_action_log.added_assignee_ids IS 'Added';
COMMENT ON COLUMN apv_action_log.removed_assignee_ids IS 'Removed';
COMMENT ON COLUMN apv_action_log.cc_user_ids IS 'CC List';
COMMENT ON COLUMN apv_action_log.attachments IS 'Attachments';

CREATE INDEX idx_apv_action_log__operator_id ON apv_action_log(operator_id);
CREATE INDEX idx_apv_action_log__instance_id_created_at ON apv_action_log(instance_id, created_at);

-- CC record
CREATE TABLE IF NOT EXISTS apv_cc_record (
    id VARCHAR(32) CONSTRAINT pk_apv_cc_record PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    instance_id VARCHAR(32) NOT NULL,
    node_id VARCHAR(32),
    task_id VARCHAR(32),
    cc_user_id VARCHAR(32) NOT NULL,
    cc_user_name VARCHAR(128) NOT NULL DEFAULT '',
    is_manual BOOLEAN NOT NULL DEFAULT false,
    read_at TIMESTAMP,
    CONSTRAINT fk_apv_cc_record__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE
);

COMMENT ON TABLE apv_cc_record IS 'CC Record';
COMMENT ON COLUMN apv_cc_record.id IS 'ID';
COMMENT ON COLUMN apv_cc_record.created_at IS 'Sent';
COMMENT ON COLUMN apv_cc_record.created_by IS 'Creator';
COMMENT ON COLUMN apv_cc_record.instance_id IS 'Instance';
COMMENT ON COLUMN apv_cc_record.node_id IS 'Node';
COMMENT ON COLUMN apv_cc_record.task_id IS 'Task';
COMMENT ON COLUMN apv_cc_record.cc_user_id IS 'User';
COMMENT ON COLUMN apv_cc_record.cc_user_name IS 'User Name';
COMMENT ON COLUMN apv_cc_record.is_manual IS 'Manual';
COMMENT ON COLUMN apv_cc_record.read_at IS 'Read';

CREATE INDEX idx_apv_cc_record__instance_id ON apv_cc_record(instance_id);
CREATE INDEX idx_apv_cc_record__cc_user_id_read_at ON apv_cc_record(cc_user_id, read_at);
CREATE UNIQUE INDEX uk_apv_cc_record__instance_id_node_id_cc_user_id ON apv_cc_record(instance_id, node_id, cc_user_id) WHERE node_id IS NOT NULL;

--------------------------------------------------------------------------------
-- Extension Tables
--------------------------------------------------------------------------------

-- Approval delegation
CREATE TABLE IF NOT EXISTS apv_delegation (
    id VARCHAR(32) CONSTRAINT pk_apv_delegation PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    delegator_id VARCHAR(32) NOT NULL,
    delegatee_id VARCHAR(32) NOT NULL,
    flow_category_id VARCHAR(32),
    flow_id VARCHAR(32),
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    reason VARCHAR(256),
    CONSTRAINT fk_apv_delegation__flow_category_id FOREIGN KEY (flow_category_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_delegation__flow_id FOREIGN KEY (flow_id)
        REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT ck_apv_delegation__time_range CHECK (start_time < end_time),
    CONSTRAINT ck_apv_delegation__no_self CHECK (delegator_id != delegatee_id)
);

COMMENT ON TABLE apv_delegation IS 'Delegation';
COMMENT ON COLUMN apv_delegation.id IS 'ID';
COMMENT ON COLUMN apv_delegation.created_at IS 'Created';
COMMENT ON COLUMN apv_delegation.updated_at IS 'Updated';
COMMENT ON COLUMN apv_delegation.created_by IS 'Creator';
COMMENT ON COLUMN apv_delegation.updated_by IS 'Updater';
COMMENT ON COLUMN apv_delegation.delegator_id IS 'Delegator';
COMMENT ON COLUMN apv_delegation.delegatee_id IS 'Delegatee';
COMMENT ON COLUMN apv_delegation.flow_category_id IS 'Category';
COMMENT ON COLUMN apv_delegation.flow_id IS 'Flow';
COMMENT ON COLUMN apv_delegation.start_time IS 'Start';
COMMENT ON COLUMN apv_delegation.end_time IS 'End';
COMMENT ON COLUMN apv_delegation.is_active IS 'Active';
COMMENT ON COLUMN apv_delegation.reason IS 'Reason';

-- For "my received delegations" query (reserved for future use)
CREATE INDEX idx_apv_delegation__delegatee_id_is_active_end_time ON apv_delegation(delegatee_id, is_active, end_time);
-- For delegation chain resolution in engine (active use)
CREATE INDEX idx_apv_delegation__delegator_id_is_active ON apv_delegation(delegator_id, is_active);

-- Form snapshot (for rollback strategies: snapshot/merge)
CREATE TABLE IF NOT EXISTS apv_form_snapshot (
    id VARCHAR(32) CONSTRAINT pk_apv_form_snapshot PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    instance_id VARCHAR(32) NOT NULL,
    node_id VARCHAR(32) NOT NULL,
    form_data JSONB NOT NULL,
    CONSTRAINT fk_apv_form_snapshot__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE
);

COMMENT ON TABLE apv_form_snapshot IS 'Snapshot'; -- Use for rollback strategies: snapshot/merge
COMMENT ON COLUMN apv_form_snapshot.id IS 'ID';
COMMENT ON COLUMN apv_form_snapshot.created_at IS 'Generated';
COMMENT ON COLUMN apv_form_snapshot.created_by IS 'Creator';
COMMENT ON COLUMN apv_form_snapshot.instance_id IS 'Instance';
COMMENT ON COLUMN apv_form_snapshot.node_id IS 'Node';
COMMENT ON COLUMN apv_form_snapshot.form_data IS 'Form Data';

CREATE INDEX idx_apv_form_snapshot__instance_id_node_id ON apv_form_snapshot(instance_id, node_id);

--------------------------------------------------------------------------------
-- Auxiliary Tables
--------------------------------------------------------------------------------

-- (apv_event_outbox removed; framework now provides a generic outbox
-- via internal/event/transport/outbox / sys_event_outbox.)

-- Urge record
CREATE TABLE IF NOT EXISTS apv_urge_record (
    id VARCHAR(32) CONSTRAINT pk_apv_urge_record PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
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

COMMENT ON TABLE apv_urge_record IS 'Urge';
COMMENT ON COLUMN apv_urge_record.id IS 'ID';
COMMENT ON COLUMN apv_urge_record.created_at IS 'Urged';
COMMENT ON COLUMN apv_urge_record.created_by IS 'Creator';
COMMENT ON COLUMN apv_urge_record.instance_id IS 'Instance';
COMMENT ON COLUMN apv_urge_record.node_id IS 'Node';
COMMENT ON COLUMN apv_urge_record.task_id IS 'Task';
COMMENT ON COLUMN apv_urge_record.urger_id IS 'Urger';
COMMENT ON COLUMN apv_urge_record.urger_name IS 'Urger Name';
COMMENT ON COLUMN apv_urge_record.target_user_id IS 'Target';
COMMENT ON COLUMN apv_urge_record.target_user_name IS 'Target Name';
COMMENT ON COLUMN apv_urge_record.message IS 'Message';

CREATE INDEX idx_apv_urge_record__task_id_urger_id_created_at ON apv_urge_record(task_id, urger_id, created_at);
CREATE INDEX idx_apv_urge_record__instance_id ON apv_urge_record(instance_id);

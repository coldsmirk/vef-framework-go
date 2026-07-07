-- --------------------------------------------------------------------------------
-- Flow Definition Tables
-- --------------------------------------------------------------------------------

-- Flow category
CREATE TABLE IF NOT EXISTS apv_flow_category (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',
    tenant_id VARCHAR(32) NOT NULL COMMENT 'Tenant',
    code VARCHAR(64) NOT NULL COMMENT 'Code',
    name VARCHAR(128) NOT NULL COMMENT 'Name',
    icon VARCHAR(128) COMMENT 'Icon',
    parent_id VARCHAR(32) COMMENT 'Parent',
    sort_order INTEGER NOT NULL DEFAULT 0 COMMENT 'Sort',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT 'Active',
    remark VARCHAR(256) COMMENT 'Remark',
    CONSTRAINT pk_apv_flow_category PRIMARY KEY (id),
    CONSTRAINT uk_apv_flow_category__tenant_id_code UNIQUE (tenant_id, code),
    CONSTRAINT fk_apv_flow_category__parent_id FOREIGN KEY (parent_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    -- Indexes are declared inline so a re-run of this idempotent script skips
    -- them together with the table (CREATE TABLE IF NOT EXISTS); MySQL has no
    -- CREATE INDEX IF NOT EXISTS, so a standalone index would error on re-run.
    INDEX idx_apv_flow_category__tenant_id (tenant_id),
    INDEX idx_apv_flow_category__parent_id (parent_id)
) COMMENT 'Flow Category';

-- Flow definition
CREATE TABLE IF NOT EXISTS apv_flow (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',
    tenant_id VARCHAR(32) NOT NULL COMMENT 'Tenant',
    category_id VARCHAR(32) NOT NULL COMMENT 'Category',
    code VARCHAR(64) NOT NULL COMMENT 'Code',
    name VARCHAR(128) NOT NULL COMMENT 'Name',
    icon VARCHAR(128) COMMENT 'Icon',
    description VARCHAR(512) COMMENT 'Description',
    -- Data binding
    binding_mode VARCHAR(16) NOT NULL DEFAULT 'standalone' COMMENT 'Binding Mode',
    business_table VARCHAR(64) COMMENT 'Biz Table',
    business_pk_field VARCHAR(64) COMMENT 'Biz PK',
    business_status_field VARCHAR(64) COMMENT 'Status Field',
    business_instance_id_field VARCHAR(64) COMMENT 'Instance ID Field',
    business_started_at_field VARCHAR(64) COMMENT 'Started At Field',
    business_finished_at_field VARCHAR(64) COMMENT 'Finished At Field',
    -- Permission config
    admin_user_ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT 'Admins',
    is_all_initiation_allowed BOOLEAN NOT NULL DEFAULT true COMMENT 'Open Start',
    -- Other
    instance_title_template VARCHAR(256) NOT NULL DEFAULT '{{.flowName}}-{{.instanceNo}}' COMMENT 'Title Template',
    is_active BOOLEAN NOT NULL DEFAULT false COMMENT 'Active',
    current_version INTEGER NOT NULL DEFAULT 0 COMMENT 'Version',
    CONSTRAINT pk_apv_flow PRIMARY KEY (id),
    CONSTRAINT uk_apv_flow__tenant_id_code UNIQUE (tenant_id, code),
    CONSTRAINT fk_apv_flow__category_id FOREIGN KEY (category_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    INDEX idx_apv_flow__category_id (category_id),
    INDEX idx_apv_flow__tenant_id (tenant_id)
) COMMENT 'Flow';

-- Flow initiator config
CREATE TABLE IF NOT EXISTS apv_flow_initiator (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    flow_id VARCHAR(32) NOT NULL COMMENT 'Flow',
    kind VARCHAR(16) NOT NULL COMMENT 'Kind',
    ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT 'Subjects',
    CONSTRAINT pk_apv_flow_initiator PRIMARY KEY (id),
    CONSTRAINT fk_apv_flow_initiator__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE CASCADE ON UPDATE CASCADE,
    INDEX idx_apv_flow_initiator__flow_id (flow_id)
) COMMENT 'Initiator';

-- Flow version
CREATE TABLE IF NOT EXISTS apv_flow_version (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',

    flow_id VARCHAR(32) NOT NULL COMMENT 'Flow',
    version INTEGER NOT NULL COMMENT 'Version',
    status VARCHAR(16) NOT NULL DEFAULT 'draft' COMMENT 'Status',
    description VARCHAR(256) COMMENT 'Description',
    -- Design data
    storage_mode VARCHAR(8) NOT NULL DEFAULT 'json' COMMENT 'Storage Mode',
    flow_schema JSON COMMENT 'Flow Schema',
    form_schema JSON COMMENT 'Form Schema',
    -- Publish info
    published_at DATETIME NULL COMMENT 'Published',
    published_by VARCHAR(32) COMMENT 'Publisher',
    -- Generated flag for partial unique semantics: at most one published version per flow
    is_published_flag TINYINT AS (CASE WHEN status = 'published' THEN 1 ELSE NULL END) STORED,
    CONSTRAINT pk_apv_flow_version PRIMARY KEY (id),
    CONSTRAINT uk_apv_flow_version__flow_id_version UNIQUE (flow_id, version),
    CONSTRAINT uk_apv_flow_version__flow_id_published UNIQUE (flow_id, is_published_flag),
    CONSTRAINT fk_apv_flow_version__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    INDEX idx_apv_flow_version__flow_id_status (flow_id, status)
) COMMENT 'Version';

-- Flow node
CREATE TABLE IF NOT EXISTS apv_flow_node (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',

    flow_version_id VARCHAR(32) NOT NULL COMMENT 'Version',
    `key` VARCHAR(64) NOT NULL COMMENT 'Key',
    kind VARCHAR(16) NOT NULL COMMENT 'Kind',
    name VARCHAR(128) NOT NULL COMMENT 'Name',
    description VARCHAR(512) COMMENT 'Description',
    -- Execution type config
    execution_type VARCHAR(16) NOT NULL DEFAULT 'manual' COMMENT 'Exec Type',
    -- Approval behavior config (for approval nodes)
    approval_method VARCHAR(16) NOT NULL DEFAULT 'parallel' COMMENT 'Method',
    pass_rule VARCHAR(16) NOT NULL DEFAULT 'all' COMMENT 'Pass Rule',
    pass_ratio DECIMAL(5,2) NOT NULL DEFAULT 100.00 COMMENT 'Pass Ratio',
    -- Empty assignee config
    empty_assignee_action VARCHAR(32) NOT NULL DEFAULT 'auto_pass' COMMENT 'Empty Action',
    fallback_user_ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT 'Fallbacks',
    admin_user_ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT 'Admins',
    same_applicant_action VARCHAR(32) NOT NULL DEFAULT 'self_approve' COMMENT 'Self Action',
    -- Rollback config
    is_rollback_allowed BOOLEAN NOT NULL DEFAULT true COMMENT 'Rollback',
    rollback_type VARCHAR(16) NOT NULL DEFAULT 'previous' COMMENT 'Rollback Type',
    rollback_data_strategy VARCHAR(16) COMMENT 'Rollback Data',
    rollback_target_keys JSON COMMENT 'Rollback Targets',
    -- Dynamic assignee config
    is_add_assignee_allowed BOOLEAN NOT NULL DEFAULT true COMMENT 'Add Allowed',
    add_assignee_types JSON NOT NULL DEFAULT (JSON_ARRAY('before', 'after', 'parallel')) COMMENT 'Add Types',
    is_remove_assignee_allowed BOOLEAN NOT NULL DEFAULT true COMMENT 'Remove Allowed',
    -- Field permissions config
    field_permissions JSON NOT NULL DEFAULT (JSON_OBJECT()) COMMENT 'Field Perms',
    -- CC config
    is_manual_cc_allowed BOOLEAN NOT NULL DEFAULT true COMMENT 'Manual CC',
    -- Other config
    is_transfer_allowed BOOLEAN NOT NULL DEFAULT true COMMENT 'Transfer',
    is_opinion_required BOOLEAN NOT NULL DEFAULT false COMMENT 'Opinion',
    timeout_hours INTEGER NOT NULL DEFAULT 0 COMMENT 'Timeout',
    timeout_action VARCHAR(16) NOT NULL DEFAULT 'none' COMMENT 'Timeout Action',
    timeout_notify_before_hours INTEGER NOT NULL DEFAULT 0 COMMENT 'Notify Before',
    urge_cooldown_minutes INTEGER NOT NULL DEFAULT 0 COMMENT 'Urge Cooldown',
    -- Advanced config
    consecutive_approver_action VARCHAR(32) NOT NULL DEFAULT 'none' COMMENT 'Consecutive',
    is_read_confirm_required BOOLEAN NOT NULL DEFAULT false COMMENT 'Read Confirm',
    branches JSON COMMENT 'Branches',
    CONSTRAINT pk_apv_flow_node PRIMARY KEY (id),
    CONSTRAINT uk_apv_flow_node__flow_version_id_key UNIQUE (flow_version_id, `key`),
    CONSTRAINT fk_apv_flow_node__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT ck_apv_flow_node__pass_ratio CHECK (pass_ratio >= 0 AND pass_ratio <= 100),
    CONSTRAINT ck_apv_flow_node__timeout_hours CHECK (timeout_hours >= 0),
    CONSTRAINT ck_apv_flow_node__timeout_notify_before_hours CHECK (timeout_notify_before_hours >= 0),
    CONSTRAINT ck_apv_flow_node__urge_cooldown_minutes CHECK (urge_cooldown_minutes >= 0)
) COMMENT 'Node';

-- Node assignee config
CREATE TABLE IF NOT EXISTS apv_flow_node_assignee (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    node_id VARCHAR(32) NOT NULL COMMENT 'Node',
    kind VARCHAR(16) NOT NULL COMMENT 'Kind',
    ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT 'Subjects',
    form_field VARCHAR(64) COMMENT 'Form Field',
    sort_order INTEGER NOT NULL DEFAULT 0 COMMENT 'Sort',
    CONSTRAINT pk_apv_flow_node_assignee PRIMARY KEY (id),
    CONSTRAINT fk_apv_flow_node_assignee__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE,
    INDEX idx_apv_flow_node_assignee__node_id (node_id)
) COMMENT 'Assignee';

-- Node CC config
CREATE TABLE IF NOT EXISTS apv_flow_node_cc (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    node_id VARCHAR(32) NOT NULL COMMENT 'Node',
    kind VARCHAR(16) NOT NULL COMMENT 'Kind',
    ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT 'Subjects',
    form_field VARCHAR(64) COMMENT 'Form Field',
    timing VARCHAR(16) NOT NULL DEFAULT 'always' COMMENT 'Timing',
    CONSTRAINT pk_apv_flow_node_cc PRIMARY KEY (id),
    CONSTRAINT fk_apv_flow_node_cc__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE,
    INDEX idx_apv_flow_node_cc__node_id (node_id)
) COMMENT 'Node CC';

-- Flow edge (directed connection between nodes)
CREATE TABLE IF NOT EXISTS apv_flow_edge (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    flow_version_id VARCHAR(32) NOT NULL COMMENT 'Version',
    `key` VARCHAR(64) COMMENT 'Key',
    source_node_id VARCHAR(32) NOT NULL COMMENT 'Source',
    source_node_key VARCHAR(64) NOT NULL COMMENT 'Source Key',
    target_node_id VARCHAR(32) NOT NULL COMMENT 'Target',
    target_node_key VARCHAR(64) NOT NULL COMMENT 'Target Key',
    source_handle VARCHAR(32) COMMENT 'Handle',
    CONSTRAINT pk_apv_flow_edge PRIMARY KEY (id),
    CONSTRAINT fk_apv_flow_edge__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_flow_edge__source_node_id FOREIGN KEY (source_node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_flow_edge__target_node_id FOREIGN KEY (target_node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE,
    INDEX idx_apv_flow_edge__flow_version_id_source_node_id (flow_version_id, source_node_id),
    INDEX idx_apv_flow_edge__source_node_id (source_node_id),
    INDEX idx_apv_flow_edge__target_node_id (target_node_id)
) COMMENT 'Edge';

-- --------------------------------------------------------------------------------
-- Runtime Tables
-- --------------------------------------------------------------------------------

-- Flow instance
CREATE TABLE IF NOT EXISTS apv_instance (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',
    tenant_id VARCHAR(32) NOT NULL COMMENT 'Tenant',
    flow_id VARCHAR(32) NOT NULL COMMENT 'Flow',
    flow_code VARCHAR(64) NOT NULL COMMENT 'Flow Code',
    flow_version_id VARCHAR(32) NOT NULL COMMENT 'Version',
    -- Application info
    title VARCHAR(256) NOT NULL COMMENT 'Title',
    instance_no VARCHAR(64) NOT NULL COMMENT 'No.',
    applicant_id VARCHAR(32) NOT NULL COMMENT 'Applicant',
    applicant_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Applicant Name',
    applicant_department_id VARCHAR(32) COMMENT 'Department',
    applicant_department_name VARCHAR(128) COMMENT 'Dept Name',
    -- Status info
    status VARCHAR(16) NOT NULL DEFAULT 'running' COMMENT 'Status',
    current_node_id VARCHAR(32) COMMENT 'Current Node',
    finished_at DATETIME NULL COMMENT 'Finished',
    -- Business association
    business_ref VARCHAR(512) COMMENT 'Biz Ref',
    -- Form data
    form_data JSON COMMENT 'Form Data',
    -- Host-supplied global variables snapshotted at instance start
    globals JSON COMMENT 'Instance Globals',
    CONSTRAINT pk_apv_instance PRIMARY KEY (id),
    CONSTRAINT fk_apv_instance__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_instance__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT uk_apv_instance__instance_no UNIQUE (instance_no),
    INDEX idx_apv_instance__tenant_id (tenant_id),
    INDEX idx_apv_instance__tenant_id_status_created_at (tenant_id, status, created_at DESC),
    INDEX idx_apv_instance__business_ref (business_ref),
    INDEX idx_apv_instance__tenant_id_applicant_id_status (tenant_id, applicant_id, status),
    INDEX idx_apv_instance__flow_id_status_created_at (flow_id, status, created_at),
    INDEX idx_apv_instance__applicant_id_status_created_at (applicant_id, status, created_at DESC),
    INDEX idx_apv_instance__current_node_id (current_node_id)
) COMMENT 'Instance';

-- --------------------------------------------------------------------------------
-- Form Data Storage (JSON index)
-- --------------------------------------------------------------------------------

-- MySQL 8.0.17+ supports multi-valued indexes on JSON arrays via CAST(... AS ... ARRAY).
-- For general JSON field queries, a regular index is not directly applicable.
-- Use application-level indexing or virtual columns for specific JSON paths as needed.

-- Node visit (one traversal of a flow node by an instance; the engine begins
-- a visit on node entry and stamps the outcome when the node concludes)
CREATE TABLE IF NOT EXISTS apv_node_visit (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Entered',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    tenant_id VARCHAR(32) NOT NULL COMMENT 'Tenant',
    instance_id VARCHAR(32) NOT NULL COMMENT 'Instance',
    node_id VARCHAR(32) NOT NULL COMMENT 'Node',
    sequence INTEGER NOT NULL COMMENT 'Step No.',
    status VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'Status',
    finished_at DATETIME NULL COMMENT 'Finished',
    CONSTRAINT pk_apv_node_visit PRIMARY KEY (id),
    CONSTRAINT fk_apv_node_visit__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_node_visit__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT uk_apv_node_visit__instance_id_sequence UNIQUE (instance_id, sequence)
) COMMENT 'Node Visit';

-- Approval task
CREATE TABLE IF NOT EXISTS apv_task (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',
    tenant_id VARCHAR(32) NOT NULL COMMENT 'Tenant',
    instance_id VARCHAR(32) NOT NULL COMMENT 'Instance',
    node_id VARCHAR(32) NOT NULL COMMENT 'Node',
    visit_id VARCHAR(32) NOT NULL COMMENT 'Visit',
    -- Assignee info
    assignee_id VARCHAR(32) NOT NULL COMMENT 'Assignee',
    assignee_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Assignee Name',
    assignee_department_id VARCHAR(32) COMMENT 'Assignee Dept',
    assignee_department_name VARCHAR(128) COMMENT 'Assignee Dept Name',
    delegator_id VARCHAR(32) COMMENT 'Delegator',
    delegator_name VARCHAR(128) COMMENT 'Delegator Name',
    delegator_department_id VARCHAR(32) COMMENT 'Delegator Dept',
    delegator_department_name VARCHAR(128) COMMENT 'Delegator Dept Name',
    sort_order INTEGER NOT NULL DEFAULT 0 COMMENT 'Sort',
    -- Task status
    status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'Status',
    read_at DATETIME NULL COMMENT 'Read',
    -- Dynamic addition source
    parent_task_id VARCHAR(32) COMMENT 'Parent',
    add_assignee_type VARCHAR(16) COMMENT 'Add Type',
    -- Timeout info
    deadline DATETIME NULL COMMENT 'Deadline',
    is_timeout BOOLEAN NOT NULL DEFAULT false COMMENT 'Timeout',
    is_pre_warning_sent BOOLEAN NOT NULL DEFAULT false COMMENT 'Pre-warned',
    -- Time record
    finished_at DATETIME NULL COMMENT 'Finished',
    -- Generated flag for partial unique semantics on active tasks
    active_flag TINYINT AS (CASE WHEN status IN ('pending', 'waiting') THEN 1 ELSE NULL END) STORED,
    CONSTRAINT pk_apv_task PRIMARY KEY (id),
    CONSTRAINT ck_apv_task__assignee_id_not_empty CHECK (TRIM(assignee_id) <> ''),
    CONSTRAINT fk_apv_task__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_task__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_task__visit_id FOREIGN KEY (visit_id) REFERENCES apv_node_visit(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_task__parent_task_id FOREIGN KEY (parent_task_id) REFERENCES apv_task(id) ON DELETE SET NULL ON UPDATE CASCADE,
    CONSTRAINT uk_apv_task__instance_id_node_id_assignee_id_active UNIQUE (instance_id, node_id, assignee_id, active_flag),
    INDEX idx_apv_task__tenant_id (tenant_id),
    INDEX idx_apv_task__tenant_id_assignee_id_status (tenant_id, assignee_id, status),
    INDEX idx_apv_task__instance_id_node_id_status (instance_id, node_id, status),
    INDEX idx_apv_task__assignee_id_status_created_at (assignee_id, status, created_at),
    INDEX idx_apv_task__assignee_id_status_finished_at (assignee_id, status, finished_at),
    INDEX idx_apv_task__instance_id_status_assignee_id (instance_id, status, assignee_id),
    INDEX idx_apv_task__visit_id (visit_id),
    INDEX idx_apv_task__deadline_active (is_timeout, status, deadline)
) COMMENT 'Task';

-- Action log
CREATE TABLE IF NOT EXISTS apv_action_log (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Performed',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    instance_id VARCHAR(32) NOT NULL COMMENT 'Instance',
    node_id VARCHAR(32) COMMENT 'Node',
    task_id VARCHAR(32) COMMENT 'Task',
    -- Action info
    action VARCHAR(16) NOT NULL COMMENT 'Action',
    operator_id VARCHAR(32) NOT NULL COMMENT 'Operator',
    operator_name VARCHAR(128) NOT NULL COMMENT 'Operator Name',
    operator_department_id VARCHAR(32) COMMENT 'Department',
    operator_department_name VARCHAR(128) COMMENT 'Dept Name',
    ip_address VARCHAR(64) COMMENT 'IP',
    user_agent VARCHAR(512) COMMENT 'User Agent',
    opinion TEXT COMMENT 'Opinion',
    meta JSON COMMENT 'Meta',
    -- Transfer/rollback info
    transfer_to_id VARCHAR(32) COMMENT 'Transferee',
    transfer_to_name VARCHAR(128) COMMENT 'Transferee Name',
    transfer_to_department_id VARCHAR(32) COMMENT 'Transferee Dept',
    transfer_to_department_name VARCHAR(128) COMMENT 'Transferee Dept Name',
    rollback_to_node_id VARCHAR(32) COMMENT 'Rollback Node',
    -- Person lists as UserBrief object arrays: [{id, name, departmentId, departmentName}]
    add_assignee_type VARCHAR(16) COMMENT 'Add Type',
    added_assignees JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT 'Added',
    removed_assignees JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT 'Removed',
    cc_users JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT 'CC List',
    -- Attachments
    attachments JSON COMMENT 'Attachments',
    CONSTRAINT pk_apv_action_log PRIMARY KEY (id),
    CONSTRAINT fk_apv_action_log__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_action_log__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_action_log__task_id FOREIGN KEY (task_id) REFERENCES apv_task(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    INDEX idx_apv_action_log__operator_id (operator_id),
    INDEX idx_apv_action_log__instance_id_created_at (instance_id, created_at)
) COMMENT 'Action Log';

-- CC record
CREATE TABLE IF NOT EXISTS apv_cc_record (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Sent',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    instance_id VARCHAR(32) NOT NULL COMMENT 'Instance',
    node_id VARCHAR(32) COMMENT 'Node',
    visit_id VARCHAR(32) COMMENT 'Visit',
    task_id VARCHAR(32) COMMENT 'Task',
    cc_user_id VARCHAR(32) NOT NULL COMMENT 'User',
    cc_user_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'User Name',
    cc_user_department_id VARCHAR(32) COMMENT 'User Dept',
    cc_user_department_name VARCHAR(128) COMMENT 'User Dept Name',
    is_manual BOOLEAN NOT NULL DEFAULT false COMMENT 'Manual',
    read_at DATETIME NULL COMMENT 'Read',
    CONSTRAINT pk_apv_cc_record PRIMARY KEY (id),
    CONSTRAINT fk_apv_cc_record__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE,
    -- NULL visit_ids never conflict, so the pair enforces per-visit dedup only
    CONSTRAINT uk_apv_cc_record__visit_id_cc_user_id UNIQUE (visit_id, cc_user_id),
    INDEX idx_apv_cc_record__instance_id (instance_id),
    INDEX idx_apv_cc_record__cc_user_id_read_at (cc_user_id, read_at)
) COMMENT 'CC Record';

-- --------------------------------------------------------------------------------
-- Extension Tables
-- --------------------------------------------------------------------------------

-- Approval delegation
CREATE TABLE IF NOT EXISTS apv_delegation (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Updater',
    delegator_id VARCHAR(32) NOT NULL COMMENT 'Delegator',
    delegatee_id VARCHAR(32) NOT NULL COMMENT 'Delegatee',
    flow_category_id VARCHAR(32) COMMENT 'Category',
    flow_id VARCHAR(32) COMMENT 'Flow',
    start_time DATETIME NOT NULL COMMENT 'Start',
    end_time DATETIME NOT NULL COMMENT 'End',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT 'Active',
    reason VARCHAR(256) COMMENT 'Reason',
    CONSTRAINT pk_apv_delegation PRIMARY KEY (id),
    CONSTRAINT fk_apv_delegation__flow_category_id FOREIGN KEY (flow_category_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_delegation__flow_id FOREIGN KEY (flow_id)
        REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT ck_apv_delegation__time_range CHECK (start_time < end_time),
    CONSTRAINT ck_apv_delegation__no_self CHECK (delegator_id != delegatee_id),
    -- delegatee index: "my received delegations" (reserved); delegator index:
    -- delegation chain resolution in engine (active use)
    INDEX idx_apv_delegation__delegatee_id_is_active_end_time (delegatee_id, is_active, end_time),
    INDEX idx_apv_delegation__delegator_id_is_active (delegator_id, is_active)
) COMMENT 'Delegation';

-- Form snapshot (for rollback strategies: snapshot/merge)
CREATE TABLE IF NOT EXISTS apv_form_snapshot (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Generated',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    instance_id VARCHAR(32) NOT NULL COMMENT 'Instance',
    node_id VARCHAR(32) NOT NULL COMMENT 'Node',
    form_data JSON NOT NULL COMMENT 'Form Data',
    CONSTRAINT pk_apv_form_snapshot PRIMARY KEY (id),
    CONSTRAINT fk_apv_form_snapshot__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE,
    INDEX idx_apv_form_snapshot__instance_id_node_id (instance_id, node_id)
) COMMENT 'Snapshot';

-- --------------------------------------------------------------------------------
-- Auxiliary Tables
-- --------------------------------------------------------------------------------

-- (apv_event_outbox removed; framework now provides a generic outbox
-- via internal/event/transport/outbox / sys_event_outbox.)

-- Urge record
CREATE TABLE IF NOT EXISTS apv_urge_record (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Urged',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    instance_id VARCHAR(32) NOT NULL COMMENT 'Instance',
    node_id VARCHAR(32) NOT NULL COMMENT 'Node',
    task_id VARCHAR(32) COMMENT 'Task',
    urger_id VARCHAR(32) NOT NULL COMMENT 'Urger',
    urger_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Urger Name',
    urger_department_id VARCHAR(32) COMMENT 'Urger Dept',
    urger_department_name VARCHAR(128) COMMENT 'Urger Dept Name',
    target_user_id VARCHAR(32) NOT NULL COMMENT 'Target',
    target_user_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Target Name',
    target_user_department_id VARCHAR(32) COMMENT 'Target Dept',
    target_user_department_name VARCHAR(128) COMMENT 'Target Dept Name',
    message TEXT NOT NULL COMMENT 'Message',
    CONSTRAINT pk_apv_urge_record PRIMARY KEY (id),
    CONSTRAINT fk_apv_urge_record__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE,
    INDEX idx_apv_urge_record__task_id_urger_id_created_at (task_id, urger_id, created_at),
    INDEX idx_apv_urge_record__instance_id (instance_id)
) COMMENT 'Urge';

-- --------------------------------------------------------------------------------
-- Table-Storage Metadata (StorageTable mode)
-- --------------------------------------------------------------------------------

-- Form table: one physical form table per published version
CREATE TABLE IF NOT EXISTS apv_form_table (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    flow_id VARCHAR(32) NOT NULL COMMENT 'Flow',
    version_id VARCHAR(32) NOT NULL COMMENT 'Version',
    physical_table_name VARCHAR(64) NOT NULL COMMENT 'Physical Table Name',
    source_field_key VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'Source Field',
    CONSTRAINT pk_apv_form_table PRIMARY KEY (id),
    CONSTRAINT fk_apv_form_table__version_id FOREIGN KEY (version_id) REFERENCES apv_flow_version(id) ON DELETE CASCADE ON UPDATE CASCADE,
    UNIQUE INDEX uk_apv_form_table__version_id_source_field_key (version_id, source_field_key)
) COMMENT 'Form Table';

-- Form table column: column definitions projected from the form schema
CREATE TABLE IF NOT EXISTS apv_form_table_column (
    id VARCHAR(32) NOT NULL COMMENT 'ID',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'Creator',
    form_table_id VARCHAR(32) NOT NULL COMMENT 'Form Table',
    column_name VARCHAR(64) NOT NULL COMMENT 'Column Name',
    column_type VARCHAR(32) NOT NULL COMMENT 'Column Type',
    is_nullable BOOLEAN NOT NULL DEFAULT true COMMENT 'Nullable',
    source_field_key VARCHAR(64) COMMENT 'Source Field Key',
    sort_order INTEGER NOT NULL DEFAULT 0 COMMENT 'Sort',
    CONSTRAINT pk_apv_form_table_column PRIMARY KEY (id),
    CONSTRAINT fk_apv_form_table_column__form_table_id FOREIGN KEY (form_table_id) REFERENCES apv_form_table(id) ON DELETE CASCADE ON UPDATE CASCADE,
    INDEX idx_apv_form_table_column__form_table_id (form_table_id)
) COMMENT 'Form Table Column';

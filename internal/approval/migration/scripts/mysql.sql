-- --------------------------------------------------------------------------------
-- Flow Definition Tables
-- --------------------------------------------------------------------------------

-- Flow category
CREATE TABLE IF NOT EXISTS apv_flow_category (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人ID',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '更新人ID',
    tenant_id VARCHAR(32) NOT NULL COMMENT '租户ID',
    code VARCHAR(64) NOT NULL COMMENT '分类编码',
    name VARCHAR(128) NOT NULL COMMENT '分类名称',
    icon VARCHAR(128) COMMENT '分类图标',
    parent_id VARCHAR(32) COMMENT '父分类ID',
    sort_order INTEGER NOT NULL DEFAULT 0 COMMENT '排序',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT '是否启用',
    remark VARCHAR(256) COMMENT '备注',
    CONSTRAINT pk_apv_flow_category PRIMARY KEY (id),
    CONSTRAINT uk_apv_flow_category__tenant_id_code UNIQUE (tenant_id, code),
    CONSTRAINT fk_apv_flow_category__parent_id FOREIGN KEY (parent_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE
) COMMENT '流程分类';

CREATE INDEX idx_apv_flow_category__tenant_id ON apv_flow_category(tenant_id);
CREATE INDEX idx_apv_flow_category__parent_id ON apv_flow_category(parent_id);

-- Flow definition
CREATE TABLE IF NOT EXISTS apv_flow (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人ID',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '更新人ID',
    tenant_id VARCHAR(32) NOT NULL COMMENT '租户ID',
    category_id VARCHAR(32) NOT NULL COMMENT '分类ID',
    code VARCHAR(64) NOT NULL COMMENT '流程编码',
    name VARCHAR(128) NOT NULL COMMENT '流程名称',
    icon VARCHAR(128) COMMENT '流程图标',
    description VARCHAR(512) COMMENT '描述',
    -- Data binding
    binding_mode VARCHAR(16) NOT NULL DEFAULT 'standalone' COMMENT '数据绑定模式',
    business_table VARCHAR(64) COMMENT '业务表名',
    business_pk_field VARCHAR(64) COMMENT '业务表主键字段',
    business_title_field VARCHAR(64) COMMENT '标题字段映射',
    business_status_field VARCHAR(64) COMMENT '状态字段映射',
    -- Permission config
    admin_user_ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT '流程管理员ID',
    is_all_initiation_allowed BOOLEAN NOT NULL DEFAULT true COMMENT '是否允许所有人发起',
    -- Other
    instance_title_template VARCHAR(256) NOT NULL DEFAULT '{{.flowName}}-{{.instanceNo}}' COMMENT '实例标题模板',
    is_active BOOLEAN NOT NULL DEFAULT false COMMENT '是否启用',
    current_version INTEGER NOT NULL DEFAULT 0 COMMENT '当前版本号',
    CONSTRAINT pk_apv_flow PRIMARY KEY (id),
    CONSTRAINT uk_apv_flow__tenant_id_code UNIQUE (tenant_id, code),
    CONSTRAINT fk_apv_flow__category_id FOREIGN KEY (category_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE
) COMMENT '流程定义';

CREATE INDEX idx_apv_flow__category_id ON apv_flow(category_id);
CREATE INDEX idx_apv_flow__tenant_id ON apv_flow(tenant_id);

-- Flow initiator config
CREATE TABLE IF NOT EXISTS apv_flow_initiator (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    flow_id VARCHAR(32) NOT NULL COMMENT '流程ID',
    kind VARCHAR(16) NOT NULL COMMENT '发起人类型',
    ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT '发起人ID',
    CONSTRAINT pk_apv_flow_initiator PRIMARY KEY (id),
    CONSTRAINT fk_apv_flow_initiator__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE CASCADE ON UPDATE CASCADE
) COMMENT '流程发起人';

CREATE INDEX idx_apv_flow_initiator__flow_id ON apv_flow_initiator(flow_id);

-- Flow version
CREATE TABLE IF NOT EXISTS apv_flow_version (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人ID',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '更新人ID',

    flow_id VARCHAR(32) NOT NULL COMMENT '流程ID',
    version INTEGER NOT NULL COMMENT '版本号',
    status VARCHAR(16) NOT NULL DEFAULT 'draft' COMMENT '版本状态',
    description VARCHAR(256) COMMENT '版本描述',
    -- Design data
    storage_mode VARCHAR(8) NOT NULL DEFAULT 'json' COMMENT '表单存储模式',
    flow_schema JSON COMMENT '流程结构定义',
    form_schema JSON COMMENT '表单结构定义',
    -- Publish info
    published_at DATETIME NULL COMMENT '发布时间',
    published_by VARCHAR(32) COMMENT '发布人ID',
    -- Generated flag for partial unique semantics: at most one published version per flow
    is_published_flag TINYINT AS (CASE WHEN status = 'published' THEN 1 ELSE NULL END) STORED,
    CONSTRAINT pk_apv_flow_version PRIMARY KEY (id),
    CONSTRAINT uk_apv_flow_version__flow_id_version UNIQUE (flow_id, version),
    CONSTRAINT uk_apv_flow_version__flow_id_published UNIQUE (flow_id, is_published_flag),
    CONSTRAINT fk_apv_flow_version__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE
) COMMENT '流程版本';

CREATE INDEX idx_apv_flow_version__flow_id_status ON apv_flow_version(flow_id, status);

-- Flow node
CREATE TABLE IF NOT EXISTS apv_flow_node (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人ID',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '更新人ID',

    flow_version_id VARCHAR(32) NOT NULL COMMENT '流程版本ID',
    `key` VARCHAR(64) NOT NULL COMMENT '节点标识',
    kind VARCHAR(16) NOT NULL COMMENT '节点类型',
    name VARCHAR(128) NOT NULL COMMENT '节点名称',
    description VARCHAR(512) COMMENT '节点描述',
    -- Execution type config
    execution_type VARCHAR(16) NOT NULL DEFAULT 'manual' COMMENT '执行类型',
    -- Approval behavior config (for approval nodes)
    approval_method VARCHAR(16) NOT NULL DEFAULT 'parallel' COMMENT '审批方式',
    pass_rule VARCHAR(16) NOT NULL DEFAULT 'all' COMMENT '通过规则',
    pass_ratio DECIMAL(3,2) NOT NULL DEFAULT 1.00 COMMENT '通过比例',
    -- Empty assignee config
    empty_assignee_action VARCHAR(32) NOT NULL DEFAULT 'auto_pass' COMMENT '无处理人时处理方式',
    fallback_user_ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT '备选处理人ID',
    admin_user_ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT '审批管理员ID',
    same_applicant_action VARCHAR(32) NOT NULL DEFAULT 'self_approve' COMMENT '审批人与提交人同一人时处理方式',
    -- Rollback config
    is_rollback_allowed BOOLEAN NOT NULL DEFAULT true COMMENT '是否允许回退',
    rollback_type VARCHAR(16) NOT NULL DEFAULT 'previous' COMMENT '回退方式',
    rollback_data_strategy VARCHAR(16) COMMENT '回退时表单数据策略',
    rollback_target_keys JSON COMMENT '指定回退目标节点Key列表',
    -- Dynamic assignee config
    is_add_assignee_allowed BOOLEAN NOT NULL DEFAULT true COMMENT '是否允许动态添加审批人',
    add_assignee_types JSON NOT NULL DEFAULT (JSON_ARRAY('before', 'after', 'parallel')) COMMENT '动态添加审批人的方式',
    is_remove_assignee_allowed BOOLEAN NOT NULL DEFAULT true COMMENT '是否允许移除审批人',
    -- Field permissions config
    field_permissions JSON NOT NULL DEFAULT (JSON_OBJECT()) COMMENT '字段权限配置',
    -- CC config
    is_manual_cc_allowed BOOLEAN NOT NULL DEFAULT true COMMENT '是否允许处理时手动指定抄送人',
    -- Other config
    is_transfer_allowed BOOLEAN NOT NULL DEFAULT true COMMENT '是否允许转交',
    is_opinion_required BOOLEAN NOT NULL DEFAULT false COMMENT '是否必填审批意见',
    timeout_hours INTEGER NOT NULL DEFAULT 0 COMMENT '超时时间',
    timeout_action VARCHAR(16) NOT NULL DEFAULT 'none' COMMENT '超时动作',
    timeout_notify_before_hours INTEGER NOT NULL DEFAULT 0 COMMENT '超时通知的提前小时数',
    urge_cooldown_minutes INTEGER NOT NULL DEFAULT 0 COMMENT '催办冷却时间',
    -- Advanced config
    consecutive_approver_action VARCHAR(32) NOT NULL DEFAULT 'none' COMMENT '相邻审批节点同一审批人处理方式',
    is_read_confirm_required BOOLEAN NOT NULL DEFAULT false COMMENT '是否需要全员已阅后才继续',
    branches JSON COMMENT '条件分支配置',
    CONSTRAINT pk_apv_flow_node PRIMARY KEY (id),
    CONSTRAINT uk_apv_flow_node__flow_version_id_key UNIQUE (flow_version_id, `key`),
    CONSTRAINT fk_apv_flow_node__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT ck_apv_flow_node__pass_ratio CHECK (pass_ratio >= 0 AND pass_ratio <= 1),
    CONSTRAINT ck_apv_flow_node__timeout_hours CHECK (timeout_hours >= 0),
    CONSTRAINT ck_apv_flow_node__timeout_notify_before_hours CHECK (timeout_notify_before_hours >= 0),
    CONSTRAINT ck_apv_flow_node__urge_cooldown_minutes CHECK (urge_cooldown_minutes >= 0)
) COMMENT '流程节点';

-- Node assignee config
CREATE TABLE IF NOT EXISTS apv_flow_node_assignee (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    node_id VARCHAR(32) NOT NULL COMMENT '节点ID',
    kind VARCHAR(16) NOT NULL COMMENT '处理人类型',
    ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT '处理人ID',
    form_field VARCHAR(64) COMMENT '表单字段',
    sort_order INTEGER NOT NULL DEFAULT 0 COMMENT '排序',
    CONSTRAINT pk_apv_flow_node_assignee PRIMARY KEY (id),
    CONSTRAINT fk_apv_flow_node_assignee__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE
) COMMENT '节点处理人';

CREATE INDEX idx_apv_flow_node_assignee__node_id ON apv_flow_node_assignee(node_id);

-- Node CC config
CREATE TABLE IF NOT EXISTS apv_flow_node_cc (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    node_id VARCHAR(32) NOT NULL COMMENT '节点ID',
    kind VARCHAR(16) NOT NULL COMMENT '抄送人类型',
    ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT '抄送人ID',
    form_field VARCHAR(64) COMMENT '表单字段',
    timing VARCHAR(16) NOT NULL DEFAULT 'always' COMMENT '抄送时机',
    CONSTRAINT pk_apv_flow_node_cc PRIMARY KEY (id),
    CONSTRAINT fk_apv_flow_node_cc__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE
) COMMENT '节点抄送人';

CREATE INDEX idx_apv_flow_node_cc__node_id ON apv_flow_node_cc(node_id);

-- Flow edge (directed connection between nodes)
CREATE TABLE IF NOT EXISTS apv_flow_edge (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    flow_version_id VARCHAR(32) NOT NULL COMMENT '流程版本ID',
    `key` VARCHAR(64) COMMENT '连线标识',
    source_node_id VARCHAR(32) NOT NULL COMMENT '来源节点ID',
    source_node_key VARCHAR(64) NOT NULL COMMENT '来源节点标识',
    target_node_id VARCHAR(32) NOT NULL COMMENT '目标节点ID',
    target_node_key VARCHAR(64) NOT NULL COMMENT '目标节点标识',
    source_handle VARCHAR(32) COMMENT '来源节点句柄',
    CONSTRAINT pk_apv_flow_edge PRIMARY KEY (id),
    CONSTRAINT fk_apv_flow_edge__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_flow_edge__source_node_id FOREIGN KEY (source_node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_flow_edge__target_node_id FOREIGN KEY (target_node_id) REFERENCES apv_flow_node(id) ON DELETE CASCADE ON UPDATE CASCADE
) COMMENT '流程连线';

CREATE INDEX idx_apv_flow_edge__flow_version_id_source_node_id ON apv_flow_edge(flow_version_id, source_node_id);
CREATE INDEX idx_apv_flow_edge__source_node_id ON apv_flow_edge(source_node_id);
CREATE INDEX idx_apv_flow_edge__target_node_id ON apv_flow_edge(target_node_id);

-- --------------------------------------------------------------------------------
-- Form Field Definition
-- --------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS apv_flow_form_field (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    flow_version_id VARCHAR(32) NOT NULL COMMENT '流程版本ID',
    name VARCHAR(64) NOT NULL COMMENT '名称',
    kind VARCHAR(32) NOT NULL COMMENT '类型',
    label VARCHAR(128) NOT NULL COMMENT '显示名称',
    placeholder VARCHAR(256) COMMENT '占位提示',
    default_value TEXT COMMENT '默认值',
    is_required BOOLEAN NOT NULL DEFAULT false COMMENT '是否必填',
    is_readonly BOOLEAN NOT NULL DEFAULT false COMMENT '是否只读',
    validation JSON COMMENT '校验规则',
    sort_order INTEGER NOT NULL DEFAULT 0 COMMENT '显示顺序',
    meta JSON COMMENT '元信息',
    CONSTRAINT pk_apv_flow_form_field PRIMARY KEY (id),
    CONSTRAINT uk_apv_flow_form_field__flow_version_id_name UNIQUE (flow_version_id, name),
    CONSTRAINT fk_apv_flow_form_field__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE CASCADE ON UPDATE CASCADE
) COMMENT '流程表单字段';

-- --------------------------------------------------------------------------------
-- Runtime Tables
-- --------------------------------------------------------------------------------

-- Flow instance
CREATE TABLE IF NOT EXISTS apv_instance (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人ID',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '更新人ID',
    tenant_id VARCHAR(32) NOT NULL COMMENT '租户ID',
    flow_id VARCHAR(32) NOT NULL COMMENT '流程ID',
    flow_version_id VARCHAR(32) NOT NULL COMMENT '流程版本ID',
    -- Application info
    title VARCHAR(256) NOT NULL COMMENT '申请标题',
    instance_no VARCHAR(64) NOT NULL COMMENT '实例编号',
    applicant_id VARCHAR(32) NOT NULL COMMENT '申请人ID',
    applicant_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '申请人姓名',
    applicant_dept_id VARCHAR(32) COMMENT '申请人部门ID',
    applicant_dept_name VARCHAR(128) COMMENT '申请人部门名称',
    -- Status info
    status VARCHAR(16) NOT NULL DEFAULT 'running' COMMENT '实例状态',
    current_node_id VARCHAR(32) COMMENT '当前节点ID',
    finished_at DATETIME NULL COMMENT '完成时间',
    -- Business association
    business_record_id VARCHAR(128) COMMENT '业务记录ID',
    -- Form data
    form_data JSON COMMENT '表单数据',
    CONSTRAINT pk_apv_instance PRIMARY KEY (id),
    CONSTRAINT fk_apv_instance__flow_id FOREIGN KEY (flow_id) REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_instance__flow_version_id FOREIGN KEY (flow_version_id) REFERENCES apv_flow_version(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT uk_apv_instance__instance_no UNIQUE (instance_no)
) COMMENT '流程实例';

CREATE INDEX idx_apv_instance__tenant_id ON apv_instance(tenant_id);
CREATE INDEX idx_apv_instance__tenant_id_status_created_at ON apv_instance(tenant_id, status, created_at DESC);
CREATE INDEX idx_apv_instance__tenant_id_applicant_id_status ON apv_instance(tenant_id, applicant_id, status);
CREATE INDEX idx_apv_instance__flow_id_status_created_at ON apv_instance(flow_id, status, created_at);
CREATE INDEX idx_apv_instance__applicant_id_status_created_at ON apv_instance(applicant_id, status, created_at DESC);
CREATE INDEX idx_apv_instance__current_node_id ON apv_instance(current_node_id);

-- --------------------------------------------------------------------------------
-- Form Data Storage (JSON index)
-- --------------------------------------------------------------------------------

-- MySQL 8.0.17+ supports multi-valued indexes on JSON arrays via CAST(... AS ... ARRAY).
-- For general JSON field queries, a regular index is not directly applicable.
-- Use application-level indexing or virtual columns for specific JSON paths as needed.

-- Approval task
CREATE TABLE IF NOT EXISTS apv_task (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人ID',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '更新人ID',
    tenant_id VARCHAR(32) NOT NULL COMMENT '租户ID',
    instance_id VARCHAR(32) NOT NULL COMMENT '流程实例ID',
    node_id VARCHAR(32) NOT NULL COMMENT '节点ID',
    -- Assignee info
    assignee_id VARCHAR(32) NOT NULL COMMENT '审批人ID',
    assignee_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '审批人姓名',
    delegator_id VARCHAR(32) COMMENT '委托人ID',
    delegator_name VARCHAR(128) COMMENT '委托人姓名',
    sort_order INTEGER NOT NULL DEFAULT 0 COMMENT '审批顺序',
    -- Task status
    status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT '任务状态',
    read_at DATETIME NULL COMMENT '阅读时间',
    -- Dynamic addition source
    parent_task_id VARCHAR(32) COMMENT '来源任务ID',
    add_assignee_type VARCHAR(16) COMMENT '添加方式',
    -- Timeout info
    deadline DATETIME NULL COMMENT '截止时间',
    is_timeout BOOLEAN NOT NULL DEFAULT false COMMENT '是否已超时',
    is_pre_warning_sent BOOLEAN NOT NULL DEFAULT false COMMENT '是否已发送预警通知',
    -- Time record
    finished_at DATETIME NULL COMMENT '完成时间',
    -- Generated flag for partial unique semantics on active tasks
    active_flag TINYINT AS (CASE WHEN status IN ('pending', 'waiting') THEN 1 ELSE NULL END) STORED,
    CONSTRAINT pk_apv_task PRIMARY KEY (id),
    CONSTRAINT ck_apv_task__assignee_id_not_empty CHECK (TRIM(assignee_id) <> ''),
    CONSTRAINT fk_apv_task__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_task__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_task__parent_task_id FOREIGN KEY (parent_task_id) REFERENCES apv_task(id) ON DELETE SET NULL ON UPDATE CASCADE,
    CONSTRAINT uk_apv_task__instance_id_node_id_assignee_id_active UNIQUE (instance_id, node_id, assignee_id, active_flag)
) COMMENT '审批任务';

CREATE INDEX idx_apv_task__tenant_id ON apv_task(tenant_id);
CREATE INDEX idx_apv_task__tenant_id_assignee_id_status ON apv_task(tenant_id, assignee_id, status);
CREATE INDEX idx_apv_task__instance_id_node_id_status ON apv_task(instance_id, node_id, status);
CREATE INDEX idx_apv_task__assignee_id_status_created_at ON apv_task(assignee_id, status, created_at);
CREATE INDEX idx_apv_task__instance_id_status_assignee_id ON apv_task(instance_id, status, assignee_id);
CREATE INDEX idx_apv_task__deadline_active ON apv_task(is_timeout, status, deadline);

-- Action log
CREATE TABLE IF NOT EXISTS apv_action_log (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '操作时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '操作人ID',
    instance_id VARCHAR(32) NOT NULL COMMENT '流程实例ID',
    node_id VARCHAR(32) COMMENT '节点ID',
    task_id VARCHAR(32) COMMENT '任务ID',
    -- Action info
    action VARCHAR(16) NOT NULL COMMENT '操作类型',
    operator_id VARCHAR(32) NOT NULL COMMENT '操作人ID',
    operator_name VARCHAR(128) NOT NULL COMMENT '操作人姓名',
    operator_dept_id VARCHAR(32) COMMENT '操作人部门ID',
    operator_dept_name VARCHAR(128) COMMENT '操作人部门名称',
    ip_address VARCHAR(64) COMMENT '操作人IP地址',
    user_agent VARCHAR(512) COMMENT '操作人用户代理',
    opinion TEXT COMMENT '审批意见',
    meta JSON COMMENT '元数据',
    -- Transfer/rollback info
    transfer_to_id VARCHAR(32) COMMENT '转交用户ID',
    transfer_to_name VARCHAR(128) COMMENT '转交用户姓名',
    rollback_to_node_id VARCHAR(32) COMMENT '回退节点ID',
    -- Dynamic assignee info
    add_assignee_type VARCHAR(16) COMMENT '加签方式',
    added_assignee_ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT '加签用户ID',
    removed_assignee_ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT '减签用户ID',
    -- CC info
    cc_user_ids JSON NOT NULL DEFAULT (JSON_ARRAY()) COMMENT '抄送用户ID',
    -- Attachments
    attachments JSON COMMENT '附件列表',
    CONSTRAINT pk_apv_action_log PRIMARY KEY (id),
    CONSTRAINT fk_apv_action_log__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_apv_action_log__node_id FOREIGN KEY (node_id) REFERENCES apv_flow_node(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_action_log__task_id FOREIGN KEY (task_id) REFERENCES apv_task(id) ON DELETE RESTRICT ON UPDATE CASCADE
) COMMENT '操作日志';

CREATE INDEX idx_apv_action_log__operator_id ON apv_action_log(operator_id);
CREATE INDEX idx_apv_action_log__instance_id_created_at ON apv_action_log(instance_id, created_at);

-- Parallel approval record
CREATE TABLE IF NOT EXISTS apv_parallel_record (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人ID',
    instance_id VARCHAR(32) NOT NULL COMMENT '流程实例ID',
    node_id VARCHAR(32) NOT NULL COMMENT '节点ID',
    task_id VARCHAR(32) NOT NULL COMMENT '关联任务ID',
    assignee_id VARCHAR(32) NOT NULL COMMENT '审批人ID',
    decision VARCHAR(16) COMMENT '审批决定',
    opinion TEXT COMMENT '审批意见',
    CONSTRAINT pk_apv_parallel_record PRIMARY KEY (id),
    CONSTRAINT fk_apv_parallel_record__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE
) COMMENT '并行审批记录';

-- For parallel pass-rule evaluation: count results per node
CREATE INDEX idx_apv_parallel_record__instance_id_node_id ON apv_parallel_record(instance_id, node_id);
-- For looking up individual vote by task
CREATE INDEX idx_apv_parallel_record__instance_id_task_id ON apv_parallel_record(instance_id, task_id);

-- CC record
CREATE TABLE IF NOT EXISTS apv_cc_record (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '抄送时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人ID',
    instance_id VARCHAR(32) NOT NULL COMMENT '流程实例ID',
    node_id VARCHAR(32) COMMENT '节点ID',
    task_id VARCHAR(32) COMMENT '关联的任务ID',
    cc_user_id VARCHAR(32) NOT NULL COMMENT '被抄送人ID',
    cc_user_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '被抄送人姓名',
    is_manual BOOLEAN NOT NULL DEFAULT false COMMENT '是否手动指定',
    read_at DATETIME NULL COMMENT '阅读时间',
    -- Generated column for partial unique index: only enforce when node_id IS NOT NULL
    _unique_node_id VARCHAR(32) AS (node_id) STORED,
    CONSTRAINT pk_apv_cc_record PRIMARY KEY (id),
    CONSTRAINT fk_apv_cc_record__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT uk_apv_cc_record__instance_id_node_id_cc_user_id UNIQUE (instance_id, _unique_node_id, cc_user_id)
) COMMENT '抄送记录';

CREATE INDEX idx_apv_cc_record__instance_id ON apv_cc_record(instance_id);
CREATE INDEX idx_apv_cc_record__cc_user_id_read_at ON apv_cc_record(cc_user_id, read_at);

-- --------------------------------------------------------------------------------
-- Extension Tables
-- --------------------------------------------------------------------------------

-- Approval delegation
CREATE TABLE IF NOT EXISTS apv_delegation (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人ID',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '更新人ID',
    delegator_id VARCHAR(32) NOT NULL COMMENT '委托人ID',
    delegatee_id VARCHAR(32) NOT NULL COMMENT '被委托人ID',
    flow_category_id VARCHAR(32) COMMENT '流程分类ID',
    flow_id VARCHAR(32) COMMENT '指定流程ID',
    start_time DATETIME NOT NULL COMMENT '代理开始时间',
    end_time DATETIME NOT NULL COMMENT '代理结束时间',
    is_active BOOLEAN NOT NULL DEFAULT true COMMENT '是否启用',
    reason VARCHAR(256) COMMENT '委托原因',
    CONSTRAINT pk_apv_delegation PRIMARY KEY (id),
    CONSTRAINT fk_apv_delegation__flow_category_id FOREIGN KEY (flow_category_id)
        REFERENCES apv_flow_category(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_apv_delegation__flow_id FOREIGN KEY (flow_id)
        REFERENCES apv_flow(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT ck_apv_delegation__time_range CHECK (start_time < end_time),
    CONSTRAINT ck_apv_delegation__no_self CHECK (delegator_id != delegatee_id)
) COMMENT '审批代理';

-- For "my received delegations" query (reserved for future use)
CREATE INDEX idx_apv_delegation__delegatee_id_is_active_end_time ON apv_delegation(delegatee_id, is_active, end_time);
-- For delegation chain resolution in engine (active use)
CREATE INDEX idx_apv_delegation__delegator_id_is_active ON apv_delegation(delegator_id, is_active);

-- Form snapshot (for rollback strategies: snapshot/merge)
CREATE TABLE IF NOT EXISTS apv_form_snapshot (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '生成时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人ID',
    instance_id VARCHAR(32) NOT NULL COMMENT '实例ID',
    node_id VARCHAR(32) NOT NULL COMMENT '节点ID',
    form_data JSON NOT NULL COMMENT '表单数据快照',
    CONSTRAINT pk_apv_form_snapshot PRIMARY KEY (id),
    CONSTRAINT fk_apv_form_snapshot__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE
) COMMENT '表单快照';

CREATE INDEX idx_apv_form_snapshot__instance_id_node_id ON apv_form_snapshot(instance_id, node_id);

-- --------------------------------------------------------------------------------
-- Auxiliary Tables
-- --------------------------------------------------------------------------------

-- Event outbox (optional, for transactional event publishing)
CREATE TABLE IF NOT EXISTS apv_event_outbox (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人ID',
    event_id VARCHAR(64) NOT NULL COMMENT '事件唯一标识',
    event_type VARCHAR(128) NOT NULL COMMENT '事件类型',
    payload JSON NOT NULL COMMENT '事件载荷',
    status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT '状态',
    retry_count INTEGER NOT NULL DEFAULT 0 COMMENT '重试次数',
    last_error TEXT COMMENT '最后一次错误信息',
    processed_at DATETIME NULL COMMENT '处理时间',
    retry_after DATETIME NULL COMMENT '下次重试时间',
    -- Generated flag for partial-index semantics on relay-eligible statuses
    relay_flag TINYINT AS (CASE WHEN status IN ('pending', 'failed', 'processing') THEN 1 ELSE NULL END) STORED,
    CONSTRAINT pk_apv_event_outbox PRIMARY KEY (id),
    CONSTRAINT uk_apv_event_outbox__event_id UNIQUE (event_id)
) COMMENT '事件发件箱';

CREATE INDEX idx_apv_event_outbox__relay ON apv_event_outbox(relay_flag, retry_after, created_at);

-- Urge record
CREATE TABLE IF NOT EXISTS apv_urge_record (
    id VARCHAR(32) NOT NULL COMMENT '主键',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '催办时间',
    created_by VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人ID',
    instance_id VARCHAR(32) NOT NULL COMMENT '流程实例ID',
    node_id VARCHAR(32) NOT NULL COMMENT '节点ID',
    task_id VARCHAR(32) COMMENT '任务ID',
    urger_id VARCHAR(32) NOT NULL COMMENT '催办人ID',
    urger_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '催办人姓名',
    target_user_id VARCHAR(32) NOT NULL COMMENT '被催办人ID',
    target_user_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '被催办人姓名',
    message TEXT NOT NULL COMMENT '催办消息',
    CONSTRAINT pk_apv_urge_record PRIMARY KEY (id),
    CONSTRAINT fk_apv_urge_record__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE
) COMMENT '催办记录';

CREATE INDEX idx_apv_urge_record__task_id_urger_id_created_at ON apv_urge_record(task_id, urger_id, created_at);
CREATE INDEX idx_apv_urge_record__instance_id ON apv_urge_record(instance_id);

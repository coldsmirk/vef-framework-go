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

COMMENT ON TABLE apv_flow_category IS '流程分类';
COMMENT ON COLUMN apv_flow_category.id IS '主键';
COMMENT ON COLUMN apv_flow_category.created_at IS '创建时间';
COMMENT ON COLUMN apv_flow_category.updated_at IS '更新时间';
COMMENT ON COLUMN apv_flow_category.created_by IS '创建人ID';
COMMENT ON COLUMN apv_flow_category.updated_by IS '更新人ID';
COMMENT ON COLUMN apv_flow_category.tenant_id IS '租户ID';
COMMENT ON COLUMN apv_flow_category.code IS '分类编码';
COMMENT ON COLUMN apv_flow_category.name IS '分类名称';
COMMENT ON COLUMN apv_flow_category.icon IS '分类图标';
COMMENT ON COLUMN apv_flow_category.parent_id IS '父分类ID';
COMMENT ON COLUMN apv_flow_category.sort_order IS '排序';
COMMENT ON COLUMN apv_flow_category.is_active IS '是否启用';
COMMENT ON COLUMN apv_flow_category.remark IS '备注';

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

COMMENT ON TABLE apv_flow IS '流程定义';
COMMENT ON COLUMN apv_flow.id IS '主键';
COMMENT ON COLUMN apv_flow.created_at IS '创建时间';
COMMENT ON COLUMN apv_flow.updated_at IS '更新时间';
COMMENT ON COLUMN apv_flow.created_by IS '创建人ID';
COMMENT ON COLUMN apv_flow.updated_by IS '更新人ID';
COMMENT ON COLUMN apv_flow.tenant_id IS '租户ID';
COMMENT ON COLUMN apv_flow.category_id IS '分类ID';
COMMENT ON COLUMN apv_flow.code IS '流程编码';
COMMENT ON COLUMN apv_flow.name IS '流程名称';
COMMENT ON COLUMN apv_flow.icon IS '流程图标';
COMMENT ON COLUMN apv_flow.description IS '描述';
COMMENT ON COLUMN apv_flow.binding_mode IS '数据绑定模式';
COMMENT ON COLUMN apv_flow.business_table IS '业务表名';
COMMENT ON COLUMN apv_flow.business_pk_field IS '业务表主键字段';
COMMENT ON COLUMN apv_flow.business_title_field IS '标题字段映射';
COMMENT ON COLUMN apv_flow.business_status_field IS '状态字段映射';
COMMENT ON COLUMN apv_flow.admin_user_ids IS '流程管理员ID';
COMMENT ON COLUMN apv_flow.is_all_initiation_allowed IS '是否允许所有人发起';
COMMENT ON COLUMN apv_flow.instance_title_template IS '实例标题模板';
COMMENT ON COLUMN apv_flow.is_active IS '是否启用';
COMMENT ON COLUMN apv_flow.current_version IS '当前版本号';

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

COMMENT ON TABLE apv_flow_initiator IS '流程发起人';
COMMENT ON COLUMN apv_flow_initiator.id IS '主键';
COMMENT ON COLUMN apv_flow_initiator.flow_id IS '流程ID';
COMMENT ON COLUMN apv_flow_initiator.kind IS '发起人类型';
COMMENT ON COLUMN apv_flow_initiator.ids IS '发起人ID';

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

COMMENT ON TABLE apv_flow_version IS '流程版本';
COMMENT ON COLUMN apv_flow_version.id IS '主键';
COMMENT ON COLUMN apv_flow_version.created_at IS '创建时间';
COMMENT ON COLUMN apv_flow_version.updated_at IS '更新时间';
COMMENT ON COLUMN apv_flow_version.created_by IS '创建人ID';
COMMENT ON COLUMN apv_flow_version.updated_by IS '更新人ID';
COMMENT ON COLUMN apv_flow_version.flow_id IS '流程ID';
COMMENT ON COLUMN apv_flow_version.version IS '版本号';
COMMENT ON COLUMN apv_flow_version.status IS '版本状态';
COMMENT ON COLUMN apv_flow_version.description IS '版本描述';
COMMENT ON COLUMN apv_flow_version.storage_mode IS '表单存储模式';
COMMENT ON COLUMN apv_flow_version.flow_schema IS '流程结构定义';
COMMENT ON COLUMN apv_flow_version.form_schema IS '表单结构定义';
COMMENT ON COLUMN apv_flow_version.published_at IS '发布时间';
COMMENT ON COLUMN apv_flow_version.published_by IS '发布人ID';

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

COMMENT ON TABLE apv_flow_node IS '流程节点';
COMMENT ON COLUMN apv_flow_node.id IS '主键';
COMMENT ON COLUMN apv_flow_node.created_at IS '创建时间';
COMMENT ON COLUMN apv_flow_node.updated_at IS '更新时间';
COMMENT ON COLUMN apv_flow_node.created_by IS '创建人ID';
COMMENT ON COLUMN apv_flow_node.updated_by IS '更新人ID';
COMMENT ON COLUMN apv_flow_node.flow_version_id IS '流程版本ID';
COMMENT ON COLUMN apv_flow_node.key IS '节点标识';
COMMENT ON COLUMN apv_flow_node.kind IS '节点类型';
COMMENT ON COLUMN apv_flow_node.name IS '节点名称';
COMMENT ON COLUMN apv_flow_node.description IS '节点描述';
COMMENT ON COLUMN apv_flow_node.execution_type IS '执行类型';
COMMENT ON COLUMN apv_flow_node.approval_method IS '审批方式';
COMMENT ON COLUMN apv_flow_node.pass_rule IS '通过规则';
COMMENT ON COLUMN apv_flow_node.pass_ratio IS '通过比例';
COMMENT ON COLUMN apv_flow_node.empty_assignee_action IS '无处理人时处理方式';
COMMENT ON COLUMN apv_flow_node.fallback_user_ids IS '备选处理人ID';
COMMENT ON COLUMN apv_flow_node.admin_user_ids IS '审批管理员ID';
COMMENT ON COLUMN apv_flow_node.same_applicant_action IS '审批人与提交人同一人时处理方式';
COMMENT ON COLUMN apv_flow_node.is_rollback_allowed IS '是否允许回退';
COMMENT ON COLUMN apv_flow_node.rollback_type IS '回退方式';
COMMENT ON COLUMN apv_flow_node.rollback_data_strategy IS '回退时表单数据策略';
COMMENT ON COLUMN apv_flow_node.rollback_target_keys IS '指定回退目标节点Key列表';
COMMENT ON COLUMN apv_flow_node.is_add_assignee_allowed IS '是否允许动态添加审批人';
COMMENT ON COLUMN apv_flow_node.add_assignee_types IS '动态添加审批人的方式';
COMMENT ON COLUMN apv_flow_node.is_remove_assignee_allowed IS '是否允许移除审批人';
COMMENT ON COLUMN apv_flow_node.field_permissions IS '字段权限配置';
COMMENT ON COLUMN apv_flow_node.is_manual_cc_allowed IS '是否允许处理时手动指定抄送人';
COMMENT ON COLUMN apv_flow_node.is_transfer_allowed IS '是否允许转交';
COMMENT ON COLUMN apv_flow_node.is_opinion_required IS '是否必填审批意见';
COMMENT ON COLUMN apv_flow_node.timeout_hours IS '超时时间';
COMMENT ON COLUMN apv_flow_node.timeout_action IS '超时动作';
COMMENT ON COLUMN apv_flow_node.timeout_notify_before_hours IS '超时通知的提前小时数';
COMMENT ON COLUMN apv_flow_node.urge_cooldown_minutes IS '催办冷却时间';
COMMENT ON COLUMN apv_flow_node.consecutive_approver_action IS '相邻审批节点同一审批人处理方式';
COMMENT ON COLUMN apv_flow_node.is_read_confirm_required IS '是否需要全员已阅后才继续';
COMMENT ON COLUMN apv_flow_node.branches IS '条件分支配置';

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

COMMENT ON TABLE apv_flow_node_assignee IS '节点处理人';
COMMENT ON COLUMN apv_flow_node_assignee.id IS '主键';
COMMENT ON COLUMN apv_flow_node_assignee.node_id IS '节点ID';
COMMENT ON COLUMN apv_flow_node_assignee.kind IS '处理人类型';
COMMENT ON COLUMN apv_flow_node_assignee.ids IS '处理人ID';
COMMENT ON COLUMN apv_flow_node_assignee.form_field IS '表单字段';
COMMENT ON COLUMN apv_flow_node_assignee.sort_order IS '排序';

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

COMMENT ON TABLE apv_flow_node_cc IS '节点抄送人';
COMMENT ON COLUMN apv_flow_node_cc.id IS '主键';
COMMENT ON COLUMN apv_flow_node_cc.node_id IS '节点ID';
COMMENT ON COLUMN apv_flow_node_cc.kind IS '抄送人类型';
COMMENT ON COLUMN apv_flow_node_cc.ids IS '抄送人ID';
COMMENT ON COLUMN apv_flow_node_cc.form_field IS '表单字段';
COMMENT ON COLUMN apv_flow_node_cc.timing IS '抄送时机';

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

COMMENT ON TABLE apv_flow_edge IS '流程连线';
COMMENT ON COLUMN apv_flow_edge.id IS '主键';
COMMENT ON COLUMN apv_flow_edge.flow_version_id IS '流程版本ID';
COMMENT ON COLUMN apv_flow_edge.key IS '连线标识';
COMMENT ON COLUMN apv_flow_edge.source_node_id IS '来源节点ID';
COMMENT ON COLUMN apv_flow_edge.source_node_key IS '来源节点标识';
COMMENT ON COLUMN apv_flow_edge.target_node_id IS '目标节点ID';
COMMENT ON COLUMN apv_flow_edge.target_node_key IS '目标节点标识';
COMMENT ON COLUMN apv_flow_edge.source_handle IS '来源节点句柄';

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

COMMENT ON TABLE apv_flow_form_field IS '流程表单字段';
COMMENT ON COLUMN apv_flow_form_field.id IS '主键';
COMMENT ON COLUMN apv_flow_form_field.flow_version_id IS '流程版本ID';
COMMENT ON COLUMN apv_flow_form_field.name IS '名称';
COMMENT ON COLUMN apv_flow_form_field.kind IS '类型';
COMMENT ON COLUMN apv_flow_form_field.label IS '显示名称';
COMMENT ON COLUMN apv_flow_form_field.placeholder IS '占位提示';
COMMENT ON COLUMN apv_flow_form_field.default_value IS '默认值';
COMMENT ON COLUMN apv_flow_form_field.is_required IS '是否必填';
COMMENT ON COLUMN apv_flow_form_field.is_readonly IS '是否只读';
COMMENT ON COLUMN apv_flow_form_field.validation IS '校验规则';
COMMENT ON COLUMN apv_flow_form_field.sort_order IS '显示顺序';
COMMENT ON COLUMN apv_flow_form_field.meta IS '元信息';

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
    applicant_dept_id VARCHAR(32),
    applicant_dept_name VARCHAR(128),
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

COMMENT ON TABLE apv_instance IS '流程实例';
COMMENT ON COLUMN apv_instance.id IS '主键';
COMMENT ON COLUMN apv_instance.created_at IS '创建时间';
COMMENT ON COLUMN apv_instance.updated_at IS '更新时间';
COMMENT ON COLUMN apv_instance.created_by IS '创建人ID';
COMMENT ON COLUMN apv_instance.updated_by IS '更新人ID';
COMMENT ON COLUMN apv_instance.tenant_id IS '租户ID';
COMMENT ON COLUMN apv_instance.flow_id IS '流程ID';
COMMENT ON COLUMN apv_instance.flow_version_id IS '流程版本ID';
COMMENT ON COLUMN apv_instance.title IS '申请标题';
COMMENT ON COLUMN apv_instance.instance_no IS '实例编号';
COMMENT ON COLUMN apv_instance.applicant_id IS '申请人ID';
COMMENT ON COLUMN apv_instance.applicant_name IS '申请人姓名';
COMMENT ON COLUMN apv_instance.applicant_dept_id IS '申请人部门ID';
COMMENT ON COLUMN apv_instance.applicant_dept_name IS '申请人部门名称';
COMMENT ON COLUMN apv_instance.status IS '实例状态';
COMMENT ON COLUMN apv_instance.current_node_id IS '当前节点ID';
COMMENT ON COLUMN apv_instance.finished_at IS '完成时间';
COMMENT ON COLUMN apv_instance.business_record_id IS '业务记录ID';
COMMENT ON COLUMN apv_instance.form_data IS '表单数据';

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

COMMENT ON TABLE apv_task IS '审批任务';
COMMENT ON COLUMN apv_task.id IS '主键';
COMMENT ON COLUMN apv_task.created_at IS '创建时间';
COMMENT ON COLUMN apv_task.updated_at IS '更新时间';
COMMENT ON COLUMN apv_task.created_by IS '创建人ID';
COMMENT ON COLUMN apv_task.updated_by IS '更新人ID';
COMMENT ON COLUMN apv_task.tenant_id IS '租户ID';
COMMENT ON COLUMN apv_task.instance_id IS '流程实例ID';
COMMENT ON COLUMN apv_task.node_id IS '节点ID';
COMMENT ON COLUMN apv_task.assignee_id IS '审批人ID';
COMMENT ON COLUMN apv_task.assignee_name IS '审批人姓名';
COMMENT ON COLUMN apv_task.delegator_id IS '委托人ID';
COMMENT ON COLUMN apv_task.delegator_name IS '委托人姓名';
COMMENT ON COLUMN apv_task.sort_order IS '审批顺序';
COMMENT ON COLUMN apv_task.status IS '任务状态';
COMMENT ON COLUMN apv_task.read_at IS '阅读时间';
COMMENT ON COLUMN apv_task.parent_task_id IS '来源任务ID';
COMMENT ON COLUMN apv_task.add_assignee_type IS '添加方式';
COMMENT ON COLUMN apv_task.deadline IS '截止时间';
COMMENT ON COLUMN apv_task.is_timeout IS '是否已超时';
COMMENT ON COLUMN apv_task.is_pre_warning_sent IS '是否已发送预警通知';
COMMENT ON COLUMN apv_task.finished_at IS '完成时间';

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
    operator_dept_id VARCHAR(32),
    operator_dept_name VARCHAR(128),
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

COMMENT ON TABLE apv_action_log IS '操作日志';
COMMENT ON COLUMN apv_action_log.id IS '主键';
COMMENT ON COLUMN apv_action_log.created_at IS '操作时间';
COMMENT ON COLUMN apv_action_log.created_by IS '操作人ID';
COMMENT ON COLUMN apv_action_log.instance_id IS '流程实例ID';
COMMENT ON COLUMN apv_action_log.node_id IS '节点ID';
COMMENT ON COLUMN apv_action_log.task_id IS '任务ID';
COMMENT ON COLUMN apv_action_log.action IS '操作类型';
COMMENT ON COLUMN apv_action_log.operator_id IS '操作人ID';
COMMENT ON COLUMN apv_action_log.operator_name IS '操作人姓名';
COMMENT ON COLUMN apv_action_log.operator_dept_id IS '操作人部门ID';
COMMENT ON COLUMN apv_action_log.operator_dept_name IS '操作人部门名称';
COMMENT ON COLUMN apv_action_log.ip_address IS '操作人IP地址';
COMMENT ON COLUMN apv_action_log.user_agent IS '操作人用户代理';
COMMENT ON COLUMN apv_action_log.opinion IS '审批意见';
COMMENT ON COLUMN apv_action_log.meta IS '元数据';
COMMENT ON COLUMN apv_action_log.transfer_to_id IS '转交用户ID';
COMMENT ON COLUMN apv_action_log.transfer_to_name IS '转交用户姓名';
COMMENT ON COLUMN apv_action_log.rollback_to_node_id IS '回退节点ID';
COMMENT ON COLUMN apv_action_log.add_assignee_type IS '加签方式';
COMMENT ON COLUMN apv_action_log.added_assignee_ids IS '加签用户ID';
COMMENT ON COLUMN apv_action_log.removed_assignee_ids IS '减签用户ID';
COMMENT ON COLUMN apv_action_log.cc_user_ids IS '抄送用户ID';
COMMENT ON COLUMN apv_action_log.attachments IS '附件列表';

CREATE INDEX idx_apv_action_log__operator_id ON apv_action_log(operator_id);
CREATE INDEX idx_apv_action_log__instance_id_created_at ON apv_action_log(instance_id, created_at);

-- Parallel approval record
CREATE TABLE IF NOT EXISTS apv_parallel_record (
    id VARCHAR(32) CONSTRAINT pk_apv_parallel_record PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    instance_id VARCHAR(32) NOT NULL,
    node_id VARCHAR(32) NOT NULL,
    task_id VARCHAR(32) NOT NULL,
    assignee_id VARCHAR(32) NOT NULL,
    decision VARCHAR(16),
    opinion TEXT,
    CONSTRAINT fk_apv_parallel_record__instance_id FOREIGN KEY (instance_id) REFERENCES apv_instance(id) ON DELETE CASCADE ON UPDATE CASCADE
);

COMMENT ON TABLE apv_parallel_record IS '并行审批记录';
COMMENT ON COLUMN apv_parallel_record.id IS '主键';
COMMENT ON COLUMN apv_parallel_record.created_at IS '记录时间';
COMMENT ON COLUMN apv_parallel_record.created_by IS '创建人ID';
COMMENT ON COLUMN apv_parallel_record.instance_id IS '流程实例ID';
COMMENT ON COLUMN apv_parallel_record.node_id IS '节点ID';
COMMENT ON COLUMN apv_parallel_record.task_id IS '关联任务ID';
COMMENT ON COLUMN apv_parallel_record.assignee_id IS '审批人ID';
COMMENT ON COLUMN apv_parallel_record.decision IS '审批决定';
COMMENT ON COLUMN apv_parallel_record.opinion IS '审批意见';

-- For parallel pass-rule evaluation: count results per node
CREATE INDEX idx_apv_parallel_record__instance_id_node_id ON apv_parallel_record(instance_id, node_id);
-- For looking up individual vote by task
CREATE INDEX idx_apv_parallel_record__instance_id_task_id ON apv_parallel_record(instance_id, task_id);

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

COMMENT ON TABLE apv_cc_record IS '抄送记录';
COMMENT ON COLUMN apv_cc_record.id IS '主键';
COMMENT ON COLUMN apv_cc_record.created_at IS '抄送时间';
COMMENT ON COLUMN apv_cc_record.created_by IS '创建人ID';
COMMENT ON COLUMN apv_cc_record.instance_id IS '流程实例ID';
COMMENT ON COLUMN apv_cc_record.node_id IS '节点ID';
COMMENT ON COLUMN apv_cc_record.task_id IS '关联的任务ID';
COMMENT ON COLUMN apv_cc_record.cc_user_id IS '被抄送人ID';
COMMENT ON COLUMN apv_cc_record.cc_user_name IS '被抄送人姓名';
COMMENT ON COLUMN apv_cc_record.is_manual IS '是否手动指定';
COMMENT ON COLUMN apv_cc_record.read_at IS '阅读时间';

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

COMMENT ON TABLE apv_delegation IS '审批代理';
COMMENT ON COLUMN apv_delegation.id IS '主键';
COMMENT ON COLUMN apv_delegation.created_at IS '创建时间';
COMMENT ON COLUMN apv_delegation.updated_at IS '更新时间';
COMMENT ON COLUMN apv_delegation.created_by IS '创建人ID';
COMMENT ON COLUMN apv_delegation.updated_by IS '更新人ID';
COMMENT ON COLUMN apv_delegation.delegator_id IS '委托人ID';
COMMENT ON COLUMN apv_delegation.delegatee_id IS '被委托人ID';
COMMENT ON COLUMN apv_delegation.flow_category_id IS '流程分类ID';
COMMENT ON COLUMN apv_delegation.flow_id IS '指定流程ID';
COMMENT ON COLUMN apv_delegation.start_time IS '代理开始时间';
COMMENT ON COLUMN apv_delegation.end_time IS '代理结束时间';
COMMENT ON COLUMN apv_delegation.is_active IS '是否启用';
COMMENT ON COLUMN apv_delegation.reason IS '委托原因';

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

COMMENT ON TABLE apv_form_snapshot IS '表单快照'; -- Use for rollback strategies: snapshot/merge
COMMENT ON COLUMN apv_form_snapshot.id IS '主键';
COMMENT ON COLUMN apv_form_snapshot.created_at IS '生成时间';
COMMENT ON COLUMN apv_form_snapshot.created_by IS '创建人ID';
COMMENT ON COLUMN apv_form_snapshot.instance_id IS '实例ID';
COMMENT ON COLUMN apv_form_snapshot.node_id IS '节点ID';
COMMENT ON COLUMN apv_form_snapshot.form_data IS '表单数据快照';

CREATE INDEX idx_apv_form_snapshot__instance_id_node_id ON apv_form_snapshot(instance_id, node_id);

--------------------------------------------------------------------------------
-- Auxiliary Tables
--------------------------------------------------------------------------------

-- Event outbox (optional, for transactional event publishing)
CREATE TABLE IF NOT EXISTS apv_event_outbox (
    id VARCHAR(32) CONSTRAINT pk_apv_event_outbox PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    event_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    processed_at TIMESTAMP,
    retry_after TIMESTAMP,
    CONSTRAINT uk_apv_event_outbox__event_id UNIQUE (event_id)
);

COMMENT ON TABLE apv_event_outbox IS '事件发件箱';
COMMENT ON COLUMN apv_event_outbox.id IS '主键';
COMMENT ON COLUMN apv_event_outbox.created_at IS '创建时间';
COMMENT ON COLUMN apv_event_outbox.created_by IS '创建人ID';
COMMENT ON COLUMN apv_event_outbox.event_id IS '事件唯一标识';
COMMENT ON COLUMN apv_event_outbox.event_type IS '事件类型';
COMMENT ON COLUMN apv_event_outbox.payload IS '事件载荷';
COMMENT ON COLUMN apv_event_outbox.status IS '状态';
COMMENT ON COLUMN apv_event_outbox.retry_count IS '重试次数';
COMMENT ON COLUMN apv_event_outbox.last_error IS '最后一次错误信息';
COMMENT ON COLUMN apv_event_outbox.processed_at IS '处理时间';
COMMENT ON COLUMN apv_event_outbox.retry_after IS '下次重试时间';

CREATE INDEX idx_apv_event_outbox__relay ON apv_event_outbox(status, retry_after, created_at) WHERE status IN ('pending', 'failed', 'processing');

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

COMMENT ON TABLE apv_urge_record IS '催办记录';
COMMENT ON COLUMN apv_urge_record.id IS '主键';
COMMENT ON COLUMN apv_urge_record.created_at IS '催办时间';
COMMENT ON COLUMN apv_urge_record.created_by IS '创建人ID';
COMMENT ON COLUMN apv_urge_record.instance_id IS '流程实例ID';
COMMENT ON COLUMN apv_urge_record.node_id IS '节点ID';
COMMENT ON COLUMN apv_urge_record.task_id IS '任务ID';
COMMENT ON COLUMN apv_urge_record.urger_id IS '催办人ID';
COMMENT ON COLUMN apv_urge_record.urger_name IS '催办人姓名';
COMMENT ON COLUMN apv_urge_record.target_user_id IS '被催办人ID';
COMMENT ON COLUMN apv_urge_record.target_user_name IS '被催办人姓名';
COMMENT ON COLUMN apv_urge_record.message IS '催办消息';

CREATE INDEX idx_apv_urge_record__task_id_urger_id_created_at ON apv_urge_record(task_id, urger_id, created_at);
CREATE INDEX idx_apv_urge_record__instance_id ON apv_urge_record(instance_id);

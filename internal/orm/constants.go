package orm

const (
	columnAll    = "*"
	sqlNull      = "NULL"
	separatorAnd = " AND "
	separatorOr  = " OR "
)

// Placeholder key for named arguments in database queries.
const PlaceholderKeyOperator = "Operator"

// System operators for audit tracking.
const (
	OperatorSystem    = "system"
	OperatorCronJob   = "cron_job"
	OperatorAnonymous = "anonymous"
)

// SQL expression placeholders for query building.
const (
	ExprOperator     = "?Operator"
	ExprTableColumns = "?TableColumns"
	ExprColumns      = "?Columns"
	ExprTablePKs     = "?TablePKs"
	ExprPKs          = "?PKs"
	ExprTableName    = "?TableName"
	ExprTableAlias   = "?TableAlias"
)

// Database column names for audit fields.
const (
	ColumnID            = "id"
	ColumnCreatedAt     = "created_at"
	ColumnUpdatedAt     = "updated_at"
	ColumnCreatedBy     = "created_by"
	ColumnUpdatedBy     = "updated_by"
	ColumnCreatedByName = "created_by_name"
	ColumnUpdatedByName = "updated_by_name"
)

// Go struct field names corresponding to audit columns.
const (
	FieldID            = "ID"
	FieldCreatedAt     = "CreatedAt"
	FieldUpdatedAt     = "UpdatedAt"
	FieldCreatedBy     = "CreatedBy"
	FieldUpdatedBy     = "UpdatedBy"
	FieldCreatedByName = "CreatedByName"
	FieldUpdatedByName = "UpdatedByName"
)

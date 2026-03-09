package crud

import "github.com/coldsmirk/vef-framework-go/orm"

// TabularFormat represents the format type for import/export operations.
type TabularFormat string

// Tabular format types for import/export.
const (
	FormatExcel TabularFormat = "excel"
	FormatCsv   TabularFormat = "csv"
)

// Error message keys and codes.
const (
	ErrMessageProcessorMustReturnSlice = "processor_must_return_slice"
	ErrCodeProcessorInvalidReturn      = 2400
)

// RPC action names (snake_case identifiers).
const (
	RPCActionCreate          = "create"
	RPCActionUpdate          = "update"
	RPCActionDelete          = "delete"
	RPCActionCreateMany      = "create_many"
	RPCActionUpdateMany      = "update_many"
	RPCActionDeleteMany      = "delete_many"
	RPCActionFindOne         = "find_one"
	RPCActionFindAll         = "find_all"
	RPCActionFindPage        = "find_page"
	RPCActionFindOptions     = "find_options"
	RPCActionFindTree        = "find_tree"
	RPCActionFindTreeOptions = "find_tree_options"
	RPCActionImport          = "import"
	RPCActionExport          = "export"
)

// REST action names in "<method> <path>" format, supporting Fiber route patterns (e.g., /:id).
const (
	RESTActionCreate          = "post /"
	RESTActionUpdate          = "put /:" + IDColumn
	RESTActionDelete          = "delete /:" + IDColumn
	RESTActionCreateMany      = "post /many"
	RESTActionUpdateMany      = "put /many"
	RESTActionDeleteMany      = "delete /many"
	RESTActionFindOne         = "get /:" + IDColumn
	RESTActionFindAll         = "get /"
	RESTActionFindPage        = "get /page"
	RESTActionFindOptions     = "get /options"
	RESTActionFindTree        = "get /tree"
	RESTActionFindTreeOptions = "get /tree/options"
	RESTActionImport          = "post /import"
	RESTActionExport          = "get /export"
)

// Well-known column names.
const (
	IDColumn          = orm.ColumnID
	ParentIDColumn    = "parent_id"
	LabelColumn       = "label"
	ValueColumn       = "value"
	DescriptionColumn = "description"
)

// Internal defaults.
const (
	maxQueryLimit              = 10000
	maxOptionsLimit            = 10000
	defaultAuditUserNameColumn = "name"
	defaultLabelColumn         = "name"
	defaultValueColumn         = orm.ColumnID
)

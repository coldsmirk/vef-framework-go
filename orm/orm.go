package orm

import "github.com/coldsmirk/vef-framework-go/internal/orm"

type (
	DB                          = orm.DB
	SelectQuery                 = orm.SelectQuery
	InsertQuery                 = orm.InsertQuery
	UpdateQuery                 = orm.UpdateQuery
	DeleteQuery                 = orm.DeleteQuery
	MergeQuery                  = orm.MergeQuery
	RawQuery                    = orm.RawQuery
	CreateTableQuery            = orm.CreateTableQuery
	DropTableQuery              = orm.DropTableQuery
	CreateIndexQuery            = orm.CreateIndexQuery
	DropIndexQuery              = orm.DropIndexQuery
	TruncateTableQuery          = orm.TruncateTableQuery
	AddColumnQuery              = orm.AddColumnQuery
	DropColumnQuery             = orm.DropColumnQuery
	Tx                          = orm.Tx
	Executor                    = orm.Executor
	TableTarget[T orm.Executor] = orm.TableTarget[T]
	QueryBuilder                = orm.QueryBuilder
	ConditionBuilder            = orm.ConditionBuilder
	Applier[T any]              = orm.Applier[T]
	ApplyFunc[T any]            = orm.ApplyFunc[T]
	RelationSpec                = orm.RelationSpec
	JoinType                    = orm.JoinType
	FuzzyKind                   = orm.FuzzyKind
	NullsMode                   = orm.NullsMode
	FromDirection               = orm.FromDirection
	FrameType                   = orm.FrameType
	FrameBoundKind              = orm.FrameBoundKind
	StatisticalMode             = orm.StatisticalMode
	ConflictAction              = orm.ConflictAction
	DateTimeUnit                = orm.DateTimeUnit
	ColumnInfo                  = orm.ColumnInfo
	Model                       = orm.Model
	IDModel                     = orm.IDModel
	CreatedModel                = orm.CreatedModel
	AuditedModel                = orm.AuditedModel
	PKField                     = orm.PKField
	ExprBuilder                 = orm.ExprBuilder
	OrderBuilder                = orm.OrderBuilder
	CaseBuilder                 = orm.CaseBuilder
	CaseWhenBuilder             = orm.CaseWhenBuilder
	ConflictBuilder             = orm.ConflictBuilder
	ConflictUpdateBuilder       = orm.ConflictUpdateBuilder
	MergeWhenBuilder            = orm.MergeWhenBuilder
	MergeUpdateBuilder          = orm.MergeUpdateBuilder
	MergeInsertBuilder          = orm.MergeInsertBuilder
	CountBuilder                = orm.CountBuilder
	SumBuilder                  = orm.SumBuilder
	AvgBuilder                  = orm.AvgBuilder
	MinBuilder                  = orm.MinBuilder
	MaxBuilder                  = orm.MaxBuilder
	StringAggBuilder            = orm.StringAggBuilder
	ArrayAggBuilder             = orm.ArrayAggBuilder
	StdDevBuilder               = orm.StdDevBuilder
	VarianceBuilder             = orm.VarianceBuilder
	JSONObjectAggBuilder        = orm.JSONObjectAggBuilder
	JSONArrayAggBuilder         = orm.JSONArrayAggBuilder
	BitOrBuilder                = orm.BitOrBuilder
	BitAndBuilder               = orm.BitAndBuilder
	BoolOrBuilder               = orm.BoolOrBuilder
	BoolAndBuilder              = orm.BoolAndBuilder
	WindowCountBuilder          = orm.WindowCountBuilder
	WindowSumBuilder            = orm.WindowSumBuilder
	WindowAvgBuilder            = orm.WindowAvgBuilder
	WindowMinBuilder            = orm.WindowMinBuilder
	WindowMaxBuilder            = orm.WindowMaxBuilder
	WindowStringAggBuilder      = orm.WindowStringAggBuilder
	WindowArrayAggBuilder       = orm.WindowArrayAggBuilder
	WindowStdDevBuilder         = orm.WindowStdDevBuilder
	WindowVarianceBuilder       = orm.WindowVarianceBuilder
	WindowJSONObjectAggBuilder  = orm.WindowJSONObjectAggBuilder
	WindowJSONArrayAggBuilder   = orm.WindowJSONArrayAggBuilder
	WindowBitOrBuilder          = orm.WindowBitOrBuilder
	WindowBitAndBuilder         = orm.WindowBitAndBuilder
	WindowBoolOrBuilder         = orm.WindowBoolOrBuilder
	WindowBoolAndBuilder        = orm.WindowBoolAndBuilder
	RowNumberBuilder            = orm.RowNumberBuilder
	RankBuilder                 = orm.RankBuilder
	DenseRankBuilder            = orm.DenseRankBuilder
	PercentRankBuilder          = orm.PercentRankBuilder
	CumeDistBuilder             = orm.CumeDistBuilder
	NTileBuilder                = orm.NTileBuilder
	LagBuilder                  = orm.LagBuilder
	LeadBuilder                 = orm.LeadBuilder
	FirstValueBuilder           = orm.FirstValueBuilder
	LastValueBuilder            = orm.LastValueBuilder
	NthValueBuilder             = orm.NthValueBuilder

	// DDL type-safe types.
	DataTypeDef       = orm.DataTypeDef
	ColumnConstraint  = orm.ColumnConstraint
	PrimaryKeyBuilder = orm.PrimaryKeyBuilder
	UniqueBuilder     = orm.UniqueBuilder
	CheckBuilder      = orm.CheckBuilder
	ForeignKeyBuilder = orm.ForeignKeyBuilder
	ReferenceAction   = orm.ReferenceAction
	IndexMethod       = orm.IndexMethod
	PartitionStrategy = orm.PartitionStrategy
)

const (
	// JoinType constants.
	JoinDefault = orm.JoinDefault
	JoinInner   = orm.JoinInner
	JoinLeft    = orm.JoinLeft
	JoinRight   = orm.JoinRight
	JoinFull    = orm.JoinFull
	JoinCross   = orm.JoinCross

	// FuzzyKind constants.
	FuzzyStarts   = orm.FuzzyStarts
	FuzzyEnds     = orm.FuzzyEnds
	FuzzyContains = orm.FuzzyContains

	// NullsMode constants.
	NullsDefault = orm.NullsDefault
	NullsRespect = orm.NullsRespect
	NullsIgnore  = orm.NullsIgnore

	// FromDirection constants.
	FromDefault = orm.FromDefault
	FromFirst   = orm.FromFirst
	FromLast    = orm.FromLast

	// FrameType constants.
	FrameDefault = orm.FrameDefault
	FrameRows    = orm.FrameRows
	FrameRange   = orm.FrameRange
	FrameGroups  = orm.FrameGroups

	// FrameBoundKind constants.
	FrameBoundNone               = orm.FrameBoundNone
	FrameBoundUnboundedPreceding = orm.FrameBoundUnboundedPreceding
	FrameBoundUnboundedFollowing = orm.FrameBoundUnboundedFollowing
	FrameBoundCurrentRow         = orm.FrameBoundCurrentRow
	FrameBoundPreceding          = orm.FrameBoundPreceding
	FrameBoundFollowing          = orm.FrameBoundFollowing

	// StatisticalMode constants.
	StatisticalDefault    = ""
	StatisticalPopulation = orm.StatisticalPopulation
	StatisticalSample     = orm.StatisticalSample

	// ConflictAction constants.
	ConflictDoNothing = orm.ConflictDoNothing
	ConflictDoUpdate  = orm.ConflictDoUpdate

	// DateTimeUnit constants.
	UnitYear   = orm.UnitYear
	UnitMonth  = orm.UnitMonth
	UnitDay    = orm.UnitDay
	UnitHour   = orm.UnitHour
	UnitMinute = orm.UnitMinute
	UnitSecond = orm.UnitSecond

	// Placeholder key for named arguments in database queries.
	PlaceholderKeyOperator = orm.PlaceholderKeyOperator

	// System operators for audit tracking.
	OperatorSystem    = orm.OperatorSystem
	OperatorCronJob   = orm.OperatorCronJob
	OperatorAnonymous = orm.OperatorAnonymous

	// SQL expression placeholders for query building.
	ExprOperator     = orm.ExprOperator
	ExprTableColumns = orm.ExprTableColumns
	ExprColumns      = orm.ExprColumns
	ExprTablePKs     = orm.ExprTablePKs
	ExprPKs          = orm.ExprPKs
	ExprTableName    = orm.ExprTableName
	ExprTableAlias   = orm.ExprTableAlias

	// Database column names for audit fields.
	ColumnID            = orm.ColumnID
	ColumnCreatedAt     = orm.ColumnCreatedAt
	ColumnUpdatedAt     = orm.ColumnUpdatedAt
	ColumnCreatedBy     = orm.ColumnCreatedBy
	ColumnUpdatedBy     = orm.ColumnUpdatedBy
	ColumnCreatedByName = orm.ColumnCreatedByName
	ColumnUpdatedByName = orm.ColumnUpdatedByName

	// Go struct field names corresponding to audit columns.
	FieldID            = orm.FieldID
	FieldCreatedAt     = orm.FieldCreatedAt
	FieldUpdatedAt     = orm.FieldUpdatedAt
	FieldCreatedBy     = orm.FieldCreatedBy
	FieldUpdatedBy     = orm.FieldUpdatedBy
	FieldCreatedByName = orm.FieldCreatedByName
	FieldUpdatedByName = orm.FieldUpdatedByName

	// ReferenceAction constants.
	ReferenceCascade    = orm.ReferenceCascade
	ReferenceRestrict   = orm.ReferenceRestrict
	ReferenceSetNull    = orm.ReferenceSetNull
	ReferenceSetDefault = orm.ReferenceSetDefault
	ReferenceNoAction   = orm.ReferenceNoAction

	// IndexMethod constants.
	IndexBTree  = orm.IndexBTree
	IndexHash   = orm.IndexHash
	IndexGIN    = orm.IndexGIN
	IndexGiST   = orm.IndexGiST
	IndexSPGiST = orm.IndexSPGiST
	IndexBRIN   = orm.IndexBRIN

	// PartitionStrategy constants.
	PartitionRange = orm.PartitionRange
	PartitionList  = orm.PartitionList
	PartitionHash  = orm.PartitionHash
)

var (
	ApplySort = orm.ApplySort

	// DataType is the factory for creating type-safe SQL data type definitions.
	DataType = orm.DataType
)

// DDL constraint constructors.
var (
	NotNull       = orm.NotNull
	Nullable      = orm.Nullable
	Default       = orm.Default
	PrimaryKey    = orm.PrimaryKey
	Unique        = orm.Unique
	AutoIncrement = orm.AutoIncrement
	Check         = orm.Check
	References    = orm.References
)

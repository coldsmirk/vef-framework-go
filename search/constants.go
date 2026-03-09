package search

type Operator string

const (
	Equals             Operator = "eq"
	NotEquals          Operator = "neq"
	GreaterThan        Operator = "gt"
	GreaterThanOrEqual Operator = "gte"
	LessThan           Operator = "lt"
	LessThanOrEqual    Operator = "lte"

	Between    Operator = "between"
	NotBetween Operator = "notBetween"

	In    Operator = "in"
	NotIn Operator = "notIn"

	IsNull    Operator = "isNull"
	IsNotNull Operator = "isNotNull"

	Contains      Operator = "contains"
	NotContains   Operator = "notContains"
	StartsWith    Operator = "startsWith"
	NotStartsWith Operator = "notStartsWith"
	EndsWith      Operator = "endsWith"
	NotEndsWith   Operator = "notEndsWith"

	ContainsIgnoreCase      Operator = "iContains"
	NotContainsIgnoreCase   Operator = "iNotContains"
	StartsWithIgnoreCase    Operator = "iStartsWith"
	NotStartsWithIgnoreCase Operator = "iNotStartsWith"
	EndsWithIgnoreCase      Operator = "iEndsWith"
	NotEndsWithIgnoreCase   Operator = "iNotEndsWith"

	TagSearch = "search"

	AttrDive     = "dive"
	AttrAlias    = "alias"
	AttrColumn   = "column"
	AttrOperator = "operator"
	AttrParams   = "params"

	ParamDelimiter = "delimiter"
	ParamType      = "type"

	IgnoreField = "-"

	// Type tokens for schema field type identification.
	TypeInt      = "int"
	TypeString   = "str"
	TypeBool     = "bool"
	TypeDecimal  = "dec"
	TypeDate     = "date"
	TypeDateTime = "datetime"
	TypeTime     = "time"
)

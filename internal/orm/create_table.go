package orm

import (
	"context"
	"database/sql"
	"strings"

	"github.com/uptrace/bun"
)

// columnDef stores a rendered column or table-level constraint fragment with its bound args.
type columnDef struct {
	sql  string
	args []any
}

// BunCreateTableQuery implements the CreateTableQuery interface with type-safe DDL operations.
//
// Bun's CreateTableQuery can only render a statement when a model is bound (it derives the
// column list from the model's schema.Table and errors otherwise), so model-less CREATE TABLE
// must be rendered as raw SQL. To keep both rendering strategies coherent, every builder option
// is recorded once into this struct, which is the single source of truth: the bun query is
// populated from that state lazily in applyToBun (model path), and buildRawSQL renders it
// directly (model-less path). Setters never mutate the bun query directly, so the two renderers
// cannot drift.
type BunCreateTableQuery struct {
	*BaseQueryBuilder

	query *bun.CreateTableQuery

	// hasModel tracks whether Model() was called. It selects the rendering strategy at
	// execution time (Model() may be called after column/option setters).
	hasModel bool

	// applied guards applyToBun so repeated Exec/String calls do not append options twice.
	applied bool

	tableName     string
	columnDefs    []columnDef
	isTemp        bool
	ifNotExists   bool
	defaultVarLen int
	partitionExpr string
	tableSpace    string
	withFKs       bool
}

// NewCreateTableQuery creates a new CreateTableQuery with BaseQueryBuilder for expression support.
func NewCreateTableQuery(db *BunDB) *BunCreateTableQuery {
	eb := &QueryExprBuilder{}
	bunQuery := db.db.NewCreateTable()
	q := &BunCreateTableQuery{
		query: bunQuery,
	}
	q.BaseQueryBuilder = newQueryBuilder(db, db.db.Dialect(), bunQuery, eb)
	eb.qb = q

	return q
}

func (q *BunCreateTableQuery) Model(model any) CreateTableQuery {
	q.query.Model(model)
	q.hasModel = true

	return q
}

func (q *BunCreateTableQuery) Table(tables ...string) CreateTableQuery {
	q.query.Table(tables...)

	if len(tables) > 0 {
		q.tableName = tables[0]
	}

	return q
}

func (q *BunCreateTableQuery) Column(name string, dataType DataTypeDef, constraints ...ColumnConstraint) CreateTableQuery {
	queryStr, args := renderColumnDef(q.Dialect(), name, dataType, constraints, q)
	q.columnDefs = append(q.columnDefs, columnDef{sql: queryStr, args: args})

	return q
}

func (q *BunCreateTableQuery) Temp() CreateTableQuery {
	q.isTemp = true

	return q
}

func (q *BunCreateTableQuery) IfNotExists() CreateTableQuery {
	q.ifNotExists = true

	return q
}

func (q *BunCreateTableQuery) DefaultVarChar(n int) CreateTableQuery {
	q.defaultVarLen = n

	return q
}

func (q *BunCreateTableQuery) PrimaryKey(builder func(PrimaryKeyBuilder)) CreateTableQuery {
	pk := new(PrimaryKeyDef)
	builder(pk)

	rendered := renderTableKeyConstraint(q.Dialect().IdentQuote(), "PRIMARY KEY", pk.name, pk.columns)
	q.columnDefs = append(q.columnDefs, columnDef{sql: rendered})

	return q
}

func (q *BunCreateTableQuery) Unique(builder func(UniqueBuilder)) CreateTableQuery {
	u := new(UniqueDef)
	builder(u)

	rendered := renderTableKeyConstraint(q.Dialect().IdentQuote(), "UNIQUE", u.name, u.columns)
	q.columnDefs = append(q.columnDefs, columnDef{sql: rendered})

	return q
}

func (q *BunCreateTableQuery) Check(builder func(CheckBuilder)) CreateTableQuery {
	ck := new(CheckDef)
	builder(ck)

	if ck.conditionBuilder == nil {
		return q
	}

	condition := q.BuildCondition(ck.conditionBuilder)

	var queryStr string
	if ck.name != "" {
		queryStr = "CONSTRAINT " + quoteIdent(q.Dialect().IdentQuote(), ck.name) + " CHECK (?)"
	} else {
		queryStr = "CHECK (?)"
	}

	q.columnDefs = append(q.columnDefs, columnDef{sql: queryStr, args: []any{condition}})

	return q
}

func (q *BunCreateTableQuery) ForeignKey(builder func(ForeignKeyBuilder)) CreateTableQuery {
	fk := new(ForeignKeyDef)
	builder(fk)

	rendered := renderTableForeignKey(q.Dialect().IdentQuote(), fk)
	q.columnDefs = append(q.columnDefs, columnDef{sql: rendered})

	return q
}

func (q *BunCreateTableQuery) PartitionBy(strategy PartitionStrategy, columns ...string) CreateTableQuery {
	q.partitionExpr = renderPartitionBy(q.Dialect().IdentQuote(), strategy, columns)

	return q
}

func (q *BunCreateTableQuery) TableSpace(tableSpace string) CreateTableQuery {
	q.tableSpace = tableSpace

	return q
}

func (q *BunCreateTableQuery) WithForeignKeys() CreateTableQuery {
	q.withFKs = true

	return q
}

func (q *BunCreateTableQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	if q.hasModel {
		q.applyToBun()

		return q.query.Exec(ctx, dest...)
	}

	rawSQL, rawArgs := q.buildRawSQL()

	return q.BaseQueryBuilder.db.db.NewRaw(rawSQL, rawArgs...).Exec(ctx)
}

func (q *BunCreateTableQuery) String() string {
	if q.hasModel {
		q.applyToBun()

		return q.query.String()
	}

	rawSQL, _ := q.buildRawSQL()

	return rawSQL
}

// applyToBun projects the recorded option state onto the bun query for model-based rendering.
// It runs at most once so repeated Exec/String calls remain idempotent.
func (q *BunCreateTableQuery) applyToBun() {
	if q.applied {
		return
	}

	q.applied = true

	if q.isTemp {
		q.query.Temp()
	}

	if q.ifNotExists {
		q.query.IfNotExists()
	}

	if q.defaultVarLen > 0 {
		q.query.Varchar(q.defaultVarLen)
	}

	for _, col := range q.columnDefs {
		q.query.ColumnExpr(col.sql, col.args...)
	}

	if q.partitionExpr != "" {
		q.query.PartitionBy(q.partitionExpr)
	}

	if q.tableSpace != "" {
		// bun quotes the tablespace identifier itself, so pass the raw name to
		// avoid double-quoting (the model-less buildRawSQL path quotes manually).
		q.query.TableSpace(q.tableSpace)
	}

	if q.withFKs {
		q.query.WithForeignKeys()
	}
}

// buildRawSQL renders the CREATE TABLE statement from the recorded option state for the
// model-less path. Identifiers (table name, tablespace) are quoted via quoteIdent so the
// output matches the model path.
func (q *BunCreateTableQuery) buildRawSQL() (string, []any) {
	quote := q.Dialect().IdentQuote()

	var (
		sb      strings.Builder
		allArgs []any
	)

	sb.WriteString("CREATE ")

	if q.isTemp {
		sb.WriteString("TEMPORARY ")
	}

	sb.WriteString("TABLE ")

	if q.ifNotExists {
		sb.WriteString("IF NOT EXISTS ")
	}

	sb.WriteString(quoteIdent(quote, q.tableName))
	sb.WriteString(" (")

	for i, col := range q.columnDefs {
		if i > 0 {
			sb.WriteString(", ")
		}

		sb.WriteString(col.sql)
		allArgs = append(allArgs, col.args...)
	}

	sb.WriteString(")")

	if q.partitionExpr != "" {
		sb.WriteString(" PARTITION BY ")
		sb.WriteString(q.partitionExpr)
	}

	if q.tableSpace != "" {
		sb.WriteString(" TABLESPACE ")
		sb.WriteString(quoteIdent(quote, q.tableSpace))
	}

	return sb.String(), allArgs
}

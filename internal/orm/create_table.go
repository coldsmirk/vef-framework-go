package orm

import (
	"context"
	"database/sql"
	"strings"

	"github.com/uptrace/bun"
)

// RawColumnDef stores a column definition for model-less table creation.
type RawColumnDef struct {
	sql  string
	args []any
}

// BunCreateTableQuery implements the CreateTableQuery interface with type-safe DDL operations.
type BunCreateTableQuery struct {
	*BaseQueryBuilder

	query *bun.CreateTableQuery

	// hasModel tracks whether Model() was called. When false, Exec/String
	// build raw SQL to avoid Bun panics on model-less queries.
	hasModel bool

	// State for model-less raw SQL building.
	tableName     string
	columnDefs    []RawColumnDef
	isTemp        bool
	ifNotExists   bool
	partitionExpr string
	tableSpace    string
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
	q.query.ColumnExpr(queryStr, args...)
	q.columnDefs = append(q.columnDefs, RawColumnDef{sql: queryStr, args: args})

	return q
}

func (q *BunCreateTableQuery) Temp() CreateTableQuery {
	q.query.Temp()
	q.isTemp = true

	return q
}

func (q *BunCreateTableQuery) IfNotExists() CreateTableQuery {
	q.query.IfNotExists()
	q.ifNotExists = true

	return q
}

func (q *BunCreateTableQuery) DefaultVarChar(n int) CreateTableQuery {
	q.query.Varchar(n)

	return q
}

func (q *BunCreateTableQuery) PrimaryKey(builder func(PrimaryKeyBuilder)) CreateTableQuery {
	pk := &PrimaryKeyDef{}
	builder(pk)

	rendered := renderTableKeyConstraint(q.Dialect().IdentQuote(), "PRIMARY KEY", pk.name, pk.columns)
	q.query.ColumnExpr(rendered)
	q.columnDefs = append(q.columnDefs, RawColumnDef{sql: rendered})

	return q
}

func (q *BunCreateTableQuery) Unique(builder func(UniqueBuilder)) CreateTableQuery {
	u := &UniqueDef{}
	builder(u)

	rendered := renderTableKeyConstraint(q.Dialect().IdentQuote(), "UNIQUE", u.name, u.columns)
	q.query.ColumnExpr(rendered)
	q.columnDefs = append(q.columnDefs, RawColumnDef{sql: rendered})

	return q
}

func (q *BunCreateTableQuery) Check(builder func(CheckBuilder)) CreateTableQuery {
	ck := &CheckDef{}
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

	q.query.ColumnExpr(queryStr, condition)
	q.columnDefs = append(q.columnDefs, RawColumnDef{sql: queryStr, args: []any{condition}})

	return q
}

func (q *BunCreateTableQuery) ForeignKey(builder func(ForeignKeyBuilder)) CreateTableQuery {
	fk := &ForeignKeyDef{}
	builder(fk)

	rendered := renderTableForeignKey(q.Dialect().IdentQuote(), fk)
	q.query.ColumnExpr(rendered)
	q.columnDefs = append(q.columnDefs, RawColumnDef{sql: rendered})

	return q
}

func (q *BunCreateTableQuery) PartitionBy(strategy PartitionStrategy, columns ...string) CreateTableQuery {
	rendered := renderPartitionBy(q.Dialect().IdentQuote(), strategy, columns)
	q.query.PartitionBy(rendered)
	q.partitionExpr = rendered

	return q
}

func (q *BunCreateTableQuery) TableSpace(tableSpace string) CreateTableQuery {
	q.query.TableSpace(tableSpace)
	q.tableSpace = tableSpace

	return q
}

func (q *BunCreateTableQuery) WithForeignKeys() CreateTableQuery {
	q.query.WithForeignKeys()

	return q
}

func (q *BunCreateTableQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	if q.hasModel {
		return q.query.Exec(ctx, dest...)
	}

	rawSQL, rawArgs := q.buildRawCreateSQL()

	return q.BaseQueryBuilder.db.db.NewRaw(rawSQL, rawArgs...).Exec(ctx)
}

func (q *BunCreateTableQuery) String() string {
	if q.hasModel {
		return q.query.String()
	}

	rawSQL, _ := q.buildRawCreateSQL()

	return rawSQL
}

// buildRawCreateSQL generates CREATE TABLE SQL from tracked state (for model-less queries).
func (q *BunCreateTableQuery) buildRawCreateSQL() (string, []any) {
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

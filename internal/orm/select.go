package orm

import (
	"context"
	"database/sql"
	"errors"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/page"
	"github.com/coldsmirk/vef-framework-go/result"
)

// NewSelectQuery creates a new SelectQuery instance with the provided database instance.
// It initializes the query builders and sets up the table schema context for proper query building.
func NewSelectQuery(db *BunDB) *BunSelectQuery {
	eb := &QueryExprBuilder{}
	sq := db.db.NewSelect()
	dialect := db.db.Dialect()
	query := &BunSelectQuery{
		QueryBuilder: newQueryBuilder(db, dialect, sq, eb),

		db:      db,
		dialect: dialect,
		eb:      eb,
		query:   sq,
	}
	eb.qb = query

	return query
}

// BunSelectQuery is the concrete implementation of SelectQuery interface.
// It wraps bun.SelectQuery and provides additional functionality for expression building.
type BunSelectQuery struct {
	QueryBuilder

	db         *BunDB
	dialect    schema.Dialect
	eb         ExprBuilder
	query      *bun.SelectQuery
	isSubQuery bool

	// State tracking for deferred select operations
	hasSelectAll          bool
	hasSelectModelColumns bool
	hasSelectModelPKs     bool
	explicitSelects       []func()
	exprSelects           []func()
	selectStateApplied    bool
}

func (q *BunSelectQuery) DB() DB {
	return q.db
}

func (q *BunSelectQuery) With(name string, builder func(query SelectQuery)) SelectQuery {
	q.query.With(name, q.BuildSubQuery(builder))

	return q
}

func (q *BunSelectQuery) WithValues(name string, model any, withOrder ...bool) SelectQuery {
	values := q.query.NewValues(model)
	if len(withOrder) > 0 && withOrder[0] {
		values.WithOrder()
	}

	q.query.With(name, values)

	return q
}

func (q *BunSelectQuery) WithRecursive(name string, builder func(query SelectQuery)) SelectQuery {
	q.query.WithRecursive(name, q.BuildSubQuery(builder))

	return q
}

func (q *BunSelectQuery) SelectAll() SelectQuery {
	q.clearSelectState()
	q.hasSelectAll = true

	return q
}

func (q *BunSelectQuery) Select(columns ...string) SelectQuery {
	q.hasSelectAll = false

	for _, column := range columns {
		q.explicitSelects = append(q.explicitSelects, func() {
			q.query.ColumnExpr("?", q.eb.Column(column))
		})
	}

	return q
}

func (q *BunSelectQuery) SelectAs(column, alias string) SelectQuery {
	q.hasSelectAll = false

	q.explicitSelects = append(q.explicitSelects, func() {
		q.query.ColumnExpr("? AS ?", q.eb.Column(column), bun.Name(alias))
	})

	return q
}

func (q *BunSelectQuery) SelectExpr(builder func(ExprBuilder) any, alias ...string) SelectQuery {
	expr := builder(q.eb)

	q.exprSelects = append(q.exprSelects, func() {
		if len(alias) > 0 && alias[0] != "" {
			q.query.ColumnExpr("? AS ?", expr, bun.Name(alias[0]))
		} else {
			q.query.ColumnExpr("?", expr)
		}
	})

	return q
}

func (q *BunSelectQuery) SelectModelColumns() SelectQuery {
	q.hasSelectAll = false
	q.hasSelectModelPKs = false
	q.hasSelectModelColumns = true

	return q
}

func (q *BunSelectQuery) SelectModelPKs() SelectQuery {
	q.hasSelectAll = false
	q.hasSelectModelColumns = false
	q.hasSelectModelPKs = true

	return q
}

func (q *BunSelectQuery) Exclude(columns ...string) SelectQuery {
	q.query.ExcludeColumn(columns...)

	return q
}

func (q *BunSelectQuery) ExcludeAll() SelectQuery {
	q.query.ExcludeColumn(columnAll)

	return q
}

func (q *BunSelectQuery) Distinct() SelectQuery {
	q.query.Distinct()

	return q
}

func (q *BunSelectQuery) DistinctOnColumns(columns ...string) SelectQuery {
	for _, column := range columns {
		q.query.DistinctOn("?", q.eb.Column(column))
	}

	return q
}

func (q *BunSelectQuery) DistinctOnExpr(builder func(ExprBuilder) any) SelectQuery {
	expr := builder(q.eb)
	q.query.DistinctOn("?", expr)

	return q
}

func (q *BunSelectQuery) Model(model any) SelectQuery {
	q.query.Model(model)

	return q
}

func (q *BunSelectQuery) ModelTable(name string, alias ...string) SelectQuery {
	if len(alias) > 0 && alias[0] != "" {
		q.query.ModelTableExpr("? AS ?", bun.Name(name), bun.Name(alias[0]))
	} else {
		q.query.ModelTableExpr("? AS ?TableAlias", bun.Name(name))
	}

	return q
}

func (q *BunSelectQuery) Table(name string, alias ...string) SelectQuery {
	if len(alias) > 0 && alias[0] != "" {
		q.query.TableExpr("? AS ?", bun.Name(name), bun.Name(alias[0]))
	} else {
		q.query.Table(name)
	}

	return q
}

func (q *BunSelectQuery) TableFrom(model any, alias ...string) SelectQuery {
	table := q.db.TableOf(model)

	aliasToUse := table.Alias
	if len(alias) > 0 && alias[0] != "" {
		aliasToUse = alias[0]
	}

	q.query.TableExpr("? AS ?", bun.Name(table.Name), bun.Name(aliasToUse))

	return q
}

func (q *BunSelectQuery) TableExpr(builder func(ExprBuilder) any, alias ...string) SelectQuery {
	if len(alias) > 0 && alias[0] != "" {
		q.query.TableExpr("? AS ?", builder(q.eb), bun.Name(alias[0]))
	} else {
		q.query.TableExpr("?", builder(q.eb))
	}

	return q
}

func (q *BunSelectQuery) TableSubQuery(builder func(query SelectQuery), alias ...string) SelectQuery {
	if len(alias) > 0 && alias[0] != "" {
		q.query.TableExpr("(?) AS ?", q.BuildSubQuery(builder), bun.Name(alias[0]))
	} else {
		q.query.TableExpr("(?)", q.BuildSubQuery(builder))
	}

	return q
}

func (q *BunSelectQuery) Join(model any, builder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinModel(JoinInner, model, alias...)
	q.query.JoinOn("?", q.BuildCondition(builder))

	return q
}

func (q *BunSelectQuery) JoinTable(name string, builder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinTable(JoinInner, name, alias...)
	q.query.JoinOn("?", q.BuildCondition(builder))

	return q
}

func (q *BunSelectQuery) JoinSubQuery(sqBuilder func(query SelectQuery), cBuilder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinSource(JoinInner, q.BuildSubQuery(sqBuilder), alias...)
	q.query.JoinOn("?", q.BuildCondition(cBuilder))

	return q
}

func (q *BunSelectQuery) JoinExpr(eBuilder func(ExprBuilder) any, cBuilder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinSource(JoinInner, eBuilder(q.eb), alias...)
	q.query.JoinOn("?", q.BuildCondition(cBuilder))

	return q
}

func (q *BunSelectQuery) LeftJoin(model any, builder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinModel(JoinLeft, model, alias...)
	q.query.JoinOn("?", q.BuildCondition(builder))

	return q
}

func (q *BunSelectQuery) LeftJoinTable(name string, builder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinTable(JoinLeft, name, alias...)
	q.query.JoinOn("?", q.BuildCondition(builder))

	return q
}

func (q *BunSelectQuery) LeftJoinSubQuery(sqBuilder func(query SelectQuery), cBuilder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinSource(JoinLeft, q.BuildSubQuery(sqBuilder), alias...)
	q.query.JoinOn("?", q.BuildCondition(cBuilder))

	return q
}

func (q *BunSelectQuery) LeftJoinExpr(eBuilder func(ExprBuilder) any, cBuilder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinSource(JoinLeft, eBuilder(q.eb), alias...)
	q.query.JoinOn("?", q.BuildCondition(cBuilder))

	return q
}

func (q *BunSelectQuery) RightJoin(model any, builder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinModel(JoinRight, model, alias...)
	q.query.JoinOn("?", q.BuildCondition(builder))

	return q
}

func (q *BunSelectQuery) RightJoinTable(name string, builder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinTable(JoinRight, name, alias...)
	q.query.JoinOn("?", q.BuildCondition(builder))

	return q
}

func (q *BunSelectQuery) RightJoinSubQuery(sqBuilder func(query SelectQuery), cBuilder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinSource(JoinRight, q.BuildSubQuery(sqBuilder), alias...)
	q.query.JoinOn("?", q.BuildCondition(cBuilder))

	return q
}

func (q *BunSelectQuery) RightJoinExpr(eBuilder func(ExprBuilder) any, cBuilder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinSource(JoinRight, eBuilder(q.eb), alias...)
	q.query.JoinOn("?", q.BuildCondition(cBuilder))

	return q
}

func (q *BunSelectQuery) FullJoin(model any, builder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinModel(JoinFull, model, alias...)
	q.query.JoinOn("?", q.BuildCondition(builder))

	return q
}

func (q *BunSelectQuery) FullJoinTable(name string, builder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinTable(JoinFull, name, alias...)
	q.query.JoinOn("?", q.BuildCondition(builder))

	return q
}

func (q *BunSelectQuery) FullJoinSubQuery(sqBuilder func(query SelectQuery), cBuilder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinSource(JoinFull, q.BuildSubQuery(sqBuilder), alias...)
	q.query.JoinOn("?", q.BuildCondition(cBuilder))

	return q
}

func (q *BunSelectQuery) FullJoinExpr(eBuilder func(ExprBuilder) any, cBuilder func(ConditionBuilder), alias ...string) SelectQuery {
	q.joinSource(JoinFull, eBuilder(q.eb), alias...)
	q.query.JoinOn("?", q.BuildCondition(cBuilder))

	return q
}

func (q *BunSelectQuery) CrossJoin(model any, alias ...string) SelectQuery {
	q.joinModel(JoinCross, model, alias...)

	return q
}

func (q *BunSelectQuery) CrossJoinTable(name string, alias ...string) SelectQuery {
	q.joinTable(JoinCross, name, alias...)

	return q
}

func (q *BunSelectQuery) CrossJoinSubQuery(sqBuilder func(query SelectQuery), alias ...string) SelectQuery {
	q.joinSource(JoinCross, q.BuildSubQuery(sqBuilder), alias...)

	return q
}

func (q *BunSelectQuery) CrossJoinExpr(eBuilder func(ExprBuilder) any, alias ...string) SelectQuery {
	q.joinSource(JoinCross, eBuilder(q.eb), alias...)

	return q
}

// joinModel adds a JOIN clause using a model's table schema.
func (q *BunSelectQuery) joinModel(joinType JoinType, model any, alias ...string) {
	table := q.db.TableOf(model)

	aliasToUse := table.Alias
	if len(alias) > 0 && alias[0] != "" {
		aliasToUse = alias[0]
	}

	q.query.Join("? ? AS ?", bun.Safe(joinType.String()), bun.Name(table.Name), bun.Name(aliasToUse))
}

// joinTable adds a JOIN clause using a table name string.
func (q *BunSelectQuery) joinTable(joinType JoinType, name string, alias ...string) {
	if len(alias) > 0 && alias[0] != "" {
		q.query.Join("? ? AS ?", bun.Safe(joinType.String()), bun.Name(name), bun.Name(alias[0]))
	} else {
		q.query.Join("? ?", bun.Safe(joinType.String()), bun.Name(name))
	}
}

// joinSource adds a JOIN clause using a subquery or expression source.
func (q *BunSelectQuery) joinSource(joinType JoinType, source any, alias ...string) {
	if len(alias) > 0 && alias[0] != "" {
		q.query.Join("? (?) AS ?", bun.Safe(joinType.String()), source, bun.Name(alias[0]))
	} else {
		q.query.Join("? (?)", bun.Safe(joinType.String()), source)
	}
}

func (q *BunSelectQuery) JoinRelations(specs ...*RelationSpec) SelectQuery {
	for _, spec := range specs {
		applyRelationSpec(spec, q)
	}

	return q
}

func (q *BunSelectQuery) Relation(name string, apply ...func(query SelectQuery)) SelectQuery {
	if len(apply) == 0 {
		q.query.Relation(name)
	} else {
		q.query.Relation(name, func(query *bun.SelectQuery) *bun.SelectQuery {
			subQuery := q.CreateSubQuery(query)
			for _, apply := range apply {
				apply(subQuery)
			}

			return query
		})
	}

	return q
}

func (q *BunSelectQuery) Where(builder func(ConditionBuilder)) SelectQuery {
	cb := newQueryConditionBuilder(q.query.QueryBuilder(), q)
	builder(cb)

	return q
}

func (q *BunSelectQuery) WherePK(columns ...string) SelectQuery {
	q.query.WherePK(columns...)

	return q
}

func (q *BunSelectQuery) WhereDeleted() SelectQuery {
	q.query.WhereDeleted()

	return q
}

func (q *BunSelectQuery) IncludeDeleted() SelectQuery {
	q.query.WhereAllWithDeleted()

	return q
}

func (q *BunSelectQuery) GroupBy(columns ...string) SelectQuery {
	for _, column := range columns {
		q.query.GroupExpr("?", q.eb.Column(column))
	}

	return q
}

func (q *BunSelectQuery) GroupByExpr(builder func(ExprBuilder) any) SelectQuery {
	expr := builder(q.eb)
	q.query.GroupExpr("?", expr)

	return q
}

func (q *BunSelectQuery) Having(builder func(ConditionBuilder)) SelectQuery {
	q.query.Having("?", q.BuildCondition(builder))

	return q
}

func (q *BunSelectQuery) OrderBy(columns ...string) SelectQuery {
	for _, column := range columns {
		q.query.OrderExpr("? ASC", q.eb.Column(column))
	}

	return q
}

func (q *BunSelectQuery) OrderByDesc(columns ...string) SelectQuery {
	for _, column := range columns {
		q.query.OrderExpr("? DESC", q.eb.Column(column))
	}

	return q
}

func (q *BunSelectQuery) OrderByExpr(builder func(ExprBuilder) any) SelectQuery {
	expr := builder(q.eb)
	q.query.OrderExpr("?", expr)

	return q
}

func (q *BunSelectQuery) Limit(limit int) SelectQuery {
	q.query.Limit(limit)

	return q
}

func (q *BunSelectQuery) Offset(offset int) SelectQuery {
	q.query.Offset(offset)

	return q
}

func (q *BunSelectQuery) Paginate(pageable page.Pageable) SelectQuery {
	pageable.Normalize()

	return q.Offset(pageable.Offset()).Limit(pageable.Size)
}

func (q *BunSelectQuery) ForShare(tables ...any) SelectQuery {
	return q.forLock("SHARE", "", tables...)
}

func (q *BunSelectQuery) ForShareNoWait(tables ...any) SelectQuery {
	return q.forLock("SHARE", "NOWAIT", tables...)
}

func (q *BunSelectQuery) ForShareSkipLocked(tables ...any) SelectQuery {
	return q.forLock("SHARE", "SKIP LOCKED", tables...)
}

func (q *BunSelectQuery) ForKeyShare(tables ...any) SelectQuery {
	return q.forLock("KEY SHARE", "", tables...)
}

func (q *BunSelectQuery) ForKeyShareNoWait(tables ...any) SelectQuery {
	return q.forLock("KEY SHARE", "NOWAIT", tables...)
}

func (q *BunSelectQuery) ForKeyShareSkipLocked(tables ...any) SelectQuery {
	return q.forLock("KEY SHARE", "SKIP LOCKED", tables...)
}

func (q *BunSelectQuery) ForUpdate(tables ...any) SelectQuery {
	return q.forLock("UPDATE", "", tables...)
}

func (q *BunSelectQuery) ForUpdateNoWait(tables ...any) SelectQuery {
	return q.forLock("UPDATE", "NOWAIT", tables...)
}

func (q *BunSelectQuery) ForUpdateSkipLocked(tables ...any) SelectQuery {
	return q.forLock("UPDATE", "SKIP LOCKED", tables...)
}

func (q *BunSelectQuery) ForNoKeyUpdate(tables ...any) SelectQuery {
	return q.forLock("NO KEY UPDATE", "", tables...)
}

func (q *BunSelectQuery) ForNoKeyUpdateNoWait(tables ...any) SelectQuery {
	return q.forLock("NO KEY UPDATE", "NOWAIT", tables...)
}

func (q *BunSelectQuery) ForNoKeyUpdateSkipLocked(tables ...any) SelectQuery {
	return q.forLock("NO KEY UPDATE", "SKIP LOCKED", tables...)
}

// postgresOnlyLockModes contains lock modes that are only supported by PostgreSQL.
var postgresOnlyLockModes = map[string]bool{
	"NO KEY UPDATE": true,
	"KEY SHARE":     true,
}

// forLock builds a FOR lock clause with the given lock mode, optional suffix, and optional table references.
// Each table can be a string (alias/name) or a model pointer (resolved to its table alias via TableOf).
// SQLite does not support row-level locking; calls are silently ignored with a warning log.
// FOR NO KEY UPDATE and FOR KEY SHARE are PostgreSQL-only; on MySQL they are silently ignored with a warning log.
func (q *BunSelectQuery) forLock(mode, suffix string, tables ...any) SelectQuery {
	dialectName := q.dialect.Name()

	if dialectName == dialect.SQLite {
		logger.Warnf("Row-level locking is not supported by SQLite, FOR %q clause will be ignored", mode)

		return q
	}

	if dialectName == dialect.MySQL && postgresOnlyLockModes[mode] {
		logger.Warnf("FOR %q is only supported by PostgreSQL, locking clause will be ignored", mode)

		return q
	}

	clause := mode

	hasTables := len(tables) > 0
	if hasTables {
		clause += " OF ?"
	}

	if suffix != "" {
		clause += " " + suffix
	}

	if hasTables {
		q.query.For(clause, Names(q.resolveTableAliases(tables)...))
	} else {
		q.query.For(clause)
	}

	return q
}

// resolveTableAliases converts a mix of string aliases and model pointers into string aliases.
func (q *BunSelectQuery) resolveTableAliases(tables []any) []string {
	aliases := make([]string, len(tables))

	for i, t := range tables {
		if s, ok := t.(string); ok {
			aliases[i] = s
		} else {
			aliases[i] = q.db.TableOf(t).Alias
		}
	}

	return aliases
}

func (q *BunSelectQuery) Union(builder func(query SelectQuery)) SelectQuery {
	q.query.Union(q.BuildSubQuery(builder))

	return q
}

func (q *BunSelectQuery) UnionAll(builder func(query SelectQuery)) SelectQuery {
	q.query.UnionAll(q.BuildSubQuery(builder))

	return q
}

func (q *BunSelectQuery) Intersect(builder func(query SelectQuery)) SelectQuery {
	q.query.Intersect(q.BuildSubQuery(builder))

	return q
}

func (q *BunSelectQuery) IntersectAll(builder func(query SelectQuery)) SelectQuery {
	q.query.IntersectAll(q.BuildSubQuery(builder))

	return q
}

func (q *BunSelectQuery) Except(builder func(query SelectQuery)) SelectQuery {
	q.query.Except(q.BuildSubQuery(builder))

	return q
}

func (q *BunSelectQuery) ExceptAll(builder func(query SelectQuery)) SelectQuery {
	q.query.ExceptAll(q.BuildSubQuery(builder))

	return q
}

func (q *BunSelectQuery) Apply(fns ...ApplyFunc[SelectQuery]) SelectQuery {
	for _, fn := range fns {
		if fn != nil {
			fn(q)
		}
	}

	return q
}

func (q *BunSelectQuery) ApplyIf(condition bool, fns ...ApplyFunc[SelectQuery]) SelectQuery {
	if condition {
		return q.Apply(fns...)
	}

	return q
}

// clearSelectState clears base column selection state flags.
// Note: exprSelects are NOT cleared as SelectExpr is cumulative and can combine with any mode.
func (q *BunSelectQuery) clearSelectState() {
	q.hasSelectAll = false
	q.hasSelectModelColumns = false
	q.hasSelectModelPKs = false
	q.explicitSelects = nil
}

// applySelectState applies deferred select state before query execution.
func (q *BunSelectQuery) applySelectState() {
	if q.selectStateApplied {
		return
	}

	if q.hasSelectAll {
		q.query.Column(columnAll)
	} else {
		if q.hasSelectModelColumns {
			q.query.ColumnExpr(ExprTableColumns)
		} else if q.hasSelectModelPKs {
			q.query.ColumnExpr(ExprTablePKs)
		}

		for _, selectFn := range q.explicitSelects {
			selectFn()
		}
	}

	for _, exprFn := range q.exprSelects {
		exprFn()
	}

	q.selectStateApplied = true
}

func (q *BunSelectQuery) Exec(ctx context.Context, dest ...any) (res sql.Result, err error) {
	if q.isSubQuery {
		return nil, ErrSubQuery
	}

	q.applySelectState()

	if res, err = q.query.Exec(ctx, dest...); err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil, result.ErrRecordNotFound
	}

	return res, err
}

func (q *BunSelectQuery) Scan(ctx context.Context, dest ...any) (err error) {
	if q.isSubQuery {
		return ErrSubQuery
	}

	q.applySelectState()

	if err = q.query.Scan(ctx, dest...); err != nil && errors.Is(err, sql.ErrNoRows) {
		return result.ErrRecordNotFound
	}

	return err
}

func (q *BunSelectQuery) Rows(ctx context.Context) (rows *sql.Rows, err error) {
	if q.isSubQuery {
		return nil, ErrSubQuery
	}

	q.applySelectState()

	if rows, err = q.query.Rows(ctx); err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil, result.ErrRecordNotFound
	}

	return rows, err
}

func (q *BunSelectQuery) ScanAndCount(ctx context.Context, dest ...any) (int64, error) {
	if q.isSubQuery {
		return 0, ErrSubQuery
	}

	q.applySelectState()

	total, err := q.query.ScanAndCount(ctx, dest...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, result.ErrRecordNotFound
		}

		return 0, err
	}

	return int64(total), nil
}

func (q *BunSelectQuery) Count(ctx context.Context) (int64, error) {
	if q.isSubQuery {
		return 0, ErrSubQuery
	}

	q.applySelectState()

	total, err := q.query.Count(ctx)

	return int64(total), err
}

func (q *BunSelectQuery) Exists(ctx context.Context) (bool, error) {
	if q.isSubQuery {
		return false, ErrSubQuery
	}

	q.applySelectState()

	return q.query.Exists(ctx)
}

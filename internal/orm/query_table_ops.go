package orm

import "github.com/uptrace/bun"

func applyModelTable[T any](name string, alias []string, modelTableExpr func(string, ...any) T) {
	if len(alias) > 0 && alias[0] != "" {
		modelTableExpr("? AS ?", bun.Name(name), bun.Name(alias[0]))
	} else {
		modelTableExpr("? AS ?TableAlias", bun.Name(name))
	}
}

func applyTable[TExpr, TTable any](
	name string,
	alias []string,
	tableExpr func(string, ...any) TExpr,
	table func(...string) TTable,
) {
	if len(alias) > 0 && alias[0] != "" {
		tableExpr("? AS ?", bun.Name(name), bun.Name(alias[0]))
	} else {
		table(name)
	}
}

func applyTableFrom[T any](tableExpr func(string, ...any) T, db *BunDB, model any, alias []string) {
	table := db.TableOf(model)

	aliasToUse := table.Alias
	if len(alias) > 0 && alias[0] != "" {
		aliasToUse = alias[0]
	}

	tableExpr("? AS ?", bun.Name(table.Name), bun.Name(aliasToUse))
}

func applyTableExpr[T any](tableExpr func(string, ...any) T, eb ExprBuilder, builder func(ExprBuilder) any, alias []string) {
	if len(alias) > 0 && alias[0] != "" {
		tableExpr("(?) AS ?", builder(eb), bun.Name(alias[0]))
	} else {
		tableExpr("(?)", builder(eb))
	}
}

func applyTableSubQuery[T any](tableExpr func(string, ...any) T, subQuery *bun.SelectQuery, alias []string) {
	if len(alias) > 0 && alias[0] != "" {
		tableExpr("(?) AS ?", subQuery, bun.Name(alias[0]))
	} else {
		tableExpr("(?)", subQuery)
	}
}

package orm

import "github.com/uptrace/bun/schema"

// DialectExprBuilder is a callback that returns a QueryAppender for dialect-specific expressions.
type DialectExprBuilder func() schema.QueryAppender

// DialectExprs maps database dialects to expression builders for cross-database compatibility.
// The Default builder is used as fallback when no dialect-specific builder is set.
type DialectExprs struct {
	Oracle    DialectExprBuilder
	SQLServer DialectExprBuilder
	Postgres  DialectExprBuilder
	MySQL     DialectExprBuilder
	SQLite    DialectExprBuilder
	Default   DialectExprBuilder
}

// DialectAction is a zero-argument callback for dialect-specific side effects.
type DialectAction func()

// DialectExecs maps database dialects to side-effect callbacks.
type DialectExecs struct {
	Oracle    DialectAction
	SQLServer DialectAction
	Postgres  DialectAction
	MySQL     DialectAction
	SQLite    DialectAction
	Default   DialectAction
}

// DialectActionErr is a callback that can return an error.
type DialectActionErr func() error

// DialectExecsWithErr maps database dialects to callbacks that may return an error.
type DialectExecsWithErr struct {
	Oracle    DialectActionErr
	SQLServer DialectActionErr
	Postgres  DialectActionErr
	MySQL     DialectActionErr
	SQLite    DialectActionErr
	Default   DialectActionErr
}

// DialectFragmentBuilder is a callback that returns a query fragment buffer.
type DialectFragmentBuilder func() ([]byte, error)

// DialectFragments maps database dialects to query fragment builders.
type DialectFragments struct {
	Oracle    DialectFragmentBuilder
	SQLServer DialectFragmentBuilder
	Postgres  DialectFragmentBuilder
	MySQL     DialectFragmentBuilder
	SQLite    DialectFragmentBuilder
	Default   DialectFragmentBuilder
}

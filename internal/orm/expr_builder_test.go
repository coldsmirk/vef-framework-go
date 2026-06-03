package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

// renderAppender renders a QueryAppender to its SQL string using the sqlite test
// generator. The methods exercised here never touch the QueryBuilder, so a
// zero-value QueryExprBuilder is sufficient.
func renderAppender(t *testing.T, appender schema.QueryAppender) string {
	t.Helper()

	out, err := appender.AppendQuery(newTestQueryGen(), nil)
	require.NoError(t, err, "should render the expression to SQL")

	return string(out)
}

// TestEscapeLikeLiteral verifies that LIKE metacharacters in a literal value are
// neutralized so the value matches literally rather than as a wildcard.
func TestEscapeLikeLiteral(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{name: "Percent", value: "50%", want: `50\%`},
		{name: "Underscore", value: "a_b", want: `a\_b`},
		{name: "Backslash", value: `a\b`, want: `a\\b`},
		{name: "BackslashBeforeWildcard", value: `\%`, want: `\\\%`},
		{name: "Mixed", value: "100%_x", want: `100\%\_x`},
		{name: "Plain", value: "Post", want: "Post"},
		{name: "Empty", value: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, escapeLikeLiteral(tc.value), "should escape LIKE metacharacters and the escape character")
		})
	}
}

// TestFuzzyMatchLiteralEscaping verifies the literal fast-path emits an escaped
// pattern plus an ESCAPE clause for the case-sensitive matchers.
func TestFuzzyMatchLiteralEscaping(t *testing.T) {
	b := new(QueryExprBuilder)

	cases := []struct {
		name  string
		build func() schema.QueryAppender
	}{
		{name: "Contains", build: func() schema.QueryAppender { return b.Contains(bun.Ident("name"), "50%") }},
		{name: "StartsWith", build: func() schema.QueryAppender { return b.StartsWith(bun.Ident("name"), "50%") }},
		{name: "EndsWith", build: func() schema.QueryAppender { return b.EndsWith(bun.Ident("name"), "50%") }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sql := renderAppender(t, tc.build())

			assert.Contains(t, sql, "LIKE", "should emit a LIKE comparison")
			assert.Contains(t, sql, `ESCAPE '\'`, "should append an ESCAPE clause so the escape character is honored")
			assert.Contains(t, sql, `\%`, "the literal percent should be escaped in the bound pattern")
		})
	}
}

// TestEscapeLikeExprDynamic verifies that a dynamic (non-literal) pattern is
// escaped SQL-side via nested REPLACE calls, escaping the escape character first
// so the escapes added for % and _ are not themselves doubled.
func TestEscapeLikeExprDynamic(t *testing.T) {
	b := new(QueryExprBuilder)

	sql := renderAppender(t, b.escapeLikeExpr(bun.Ident("keyword")))

	assert.Equal(t,
		`REPLACE(REPLACE(REPLACE("keyword", '\', '\\'), '%', '\%'), '_', '\_')`,
		sql,
		"should escape backslash, then percent, then underscore in order",
	)
}

// TestErrExprAppendQuery verifies the error-yielding appender surfaces its error
// instead of emitting SQL, which is how Reverse on SQLite and a missing dialect
// handler fail loudly rather than producing silently-wrong SQL.
func TestErrExprAppendQuery(t *testing.T) {
	out, err := errExpr{err: ErrDialectUnsupportedOperation}.AppendQuery(newTestQueryGen(), nil)

	require.ErrorIs(t, err, ErrDialectUnsupportedOperation, "should propagate the wrapped error")
	assert.Empty(t, out, "should not emit any SQL bytes")
}

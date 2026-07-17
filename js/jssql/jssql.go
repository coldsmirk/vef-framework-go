package jssql

import (
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm/sqlguard"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Name is the library identifier and the global binding installed into the
// runtime.
const Name = "sql"

// lib exposes parameterized SQL access to scripts as the global "sql" object:
//
//	sql.queryList('SELECT name FROM users WHERE age > ?', 18)  // → [{...}, ...]
//	sql.queryOne('SELECT ... WHERE id = ?', id)                // → {...} | null
//	sql.execute('UPDATE ...', args)                            // → { rowsAffected }
//
// Only placeholder binding is offered — there is deliberately no string
// interpolation helper. The library is read-only unless built with WithExecute;
// failures are thrown as catchable exceptions.
type lib struct {
	db           orm.DB
	kind         config.DBKind
	maxRows      int
	allowExecute bool
}

// New builds the sql library over db. The caller picks the data source —
// typically the primary orm.DB, or a specific one taken from
// datasource.Registry — and thereby decides what the scripts can reach.
// kind is the dialect of db, used by the read-only guard to select the
// dialect's side-effecting-function denylist.
func New(db orm.DB, kind config.DBKind, opts ...Option) js.Lib {
	cfg := libConfig{maxRows: DefaultMaxRows}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &lib{db: db, kind: kind, maxRows: cfg.maxRows, allowExecute: cfg.allowExecute}
}

func (*lib) Name() string {
	return Name
}

func (l *lib) Install(rt *js.Runtime) error {
	return rt.Set(Name, map[string]any{
		"queryList": func(query string, args ...any) ([]map[string]any, error) {
			return l.query(rt, query, args)
		},
		"queryOne": func(query string, args ...any) (any, error) {
			rows, err := l.query(rt, query, args)
			if err != nil {
				return nil, err
			}

			if len(rows) == 0 {
				return nil, nil
			}

			return rows[0], nil
		},
		"execute": func(query string, args ...any) (map[string]any, error) {
			return l.execute(rt, query, args)
		},
	})
}

// query runs a read-only statement and returns its rows as plain objects.
// Read-onlyness is enforced fail-closed through sqlguard.EnsureReadOnly:
// AST-based, so writing CTEs, stacked statements, and dialect-specific
// side-effecting functions are rejected and unparseable SQL is refused.
func (l *lib) query(rt *js.Runtime, query string, args []any) ([]map[string]any, error) {
	// The guard's parser does not understand bun's ? placeholders, so it sees
	// a structurally identical statement with each ? substituted by a literal
	// — the substitution can never turn a write into a read.
	if err := sqlguard.EnsureReadOnly(l.kind, strings.ReplaceAll(query, "?", "1")); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrQueryNotReadOnly, err)
	}

	var rows []map[string]any
	if err := l.db.NewRaw(query, args...).Scan(rt.Context(), &rows); err != nil {
		return nil, err
	}

	if len(rows) > l.maxRows {
		return nil, fmt.Errorf("%w: %d rows over limit %d, constrain the query with LIMIT", ErrTooManyRows, len(rows), l.maxRows)
	}

	if rows == nil {
		// Scan leaves the slice nil on no match, which would surface in the
		// script (and its JSON output) as null instead of the documented [].
		rows = []map[string]any{}
	}

	return rows, nil
}

// execute runs a mutating statement, guarded by the WithExecute grant.
func (l *lib) execute(rt *js.Runtime, query string, args []any) (map[string]any, error) {
	if !l.allowExecute {
		return nil, ErrExecuteDisabled
	}

	result, err := l.db.NewRaw(query, args...).Exec(rt.Context())
	if err != nil {
		return nil, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	return map[string]any{"rowsAffected": affected}, nil
}

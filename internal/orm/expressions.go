package orm

import (
	"reflect"

	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/schema"
)

var (
	bytesType         = reflect.TypeFor[[]byte]()
	queryAppenderType = reflect.TypeFor[schema.QueryAppender]()
)

type Expressions struct {
	exprs []any
	sep   any
}

func (e *Expressions) AppendQuery(gen schema.QueryGen, b []byte) ([]byte, error) {
	var appendExprs func(b []byte, slice reflect.Value) ([]byte, error)

	appendExprs = func(b []byte, slice reflect.Value) (_ []byte, err error) {
		sliceLen := slice.Len()
		if sliceLen == 0 {
			return dialect.AppendNull(b), nil
		}

		for i := range sliceLen {
			if i > 0 {
				if b, err = e.appendSep(gen, b); err != nil {
					return
				}
			}

			expr := slice.Index(i)

			switch {
			case expr.Type().Implements(queryAppenderType):
				appender := expr.Interface().(schema.QueryAppender)
				if b, err = appender.AppendQuery(gen, b); err != nil {
					return
				}

			case expr.Kind() == reflect.Slice && expr.Type() != bytesType:
				b = append(b, '(')
				if b, err = appendExprs(b, expr); err != nil {
					return
				}

				b = append(b, ')')

			default:
				b = gen.AppendValue(b, expr)
			}
		}

		return b, nil
	}

	return appendExprs(b, reflect.ValueOf(e.exprs))
}

// appendSep appends the separator between expression elements.
func (e *Expressions) appendSep(gen schema.QueryGen, b []byte) ([]byte, error) {
	switch sep := e.sep.(type) {
	case string:
		return append(b, sep...), nil
	case schema.QueryAppender:
		return sep.AppendQuery(gen, b)
	default:
		return gen.AppendValue(b, reflect.ValueOf(sep)), nil
	}
}

func newExpressions(sep any, exprs ...any) *Expressions {
	return &Expressions{
		exprs: exprs,
		sep:   sep,
	}
}

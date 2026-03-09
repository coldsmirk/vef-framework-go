package orm

import (
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/schema"
)

// Names returns a query appender that appends a list of names to the query.
func Names(ns ...string) schema.QueryAppender {
	return &names{
		ns: ns,
	}
}

type names struct {
	ns []string
}

func (n *names) AppendQuery(gen schema.QueryGen, b []byte) ([]byte, error) {
	nsLen := len(n.ns)

	if nsLen == 0 {
		return dialect.AppendNull(b), nil
	}

	for i := range nsLen {
		if i > 0 {
			b = append(b, ", "...)
		}

		b = gen.AppendName(b, n.ns[i])
	}

	return b, nil
}

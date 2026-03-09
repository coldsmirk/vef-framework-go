package orm

import collections "github.com/coldsmirk/go-collections"

// returningColumns keeps RETURNING columns unique while preserving append order.
type returningColumns struct {
	seen  collections.Set[string]
	order []string
}

func newReturningColumns() *returningColumns {
	return &returningColumns{
		seen: collections.NewHashSet[string](),
	}
}

func (r *returningColumns) AddAll(columns ...string) {
	for _, column := range columns {
		if r.seen.Add(column) {
			r.order = append(r.order, column)
		}
	}
}

func (r *returningColumns) Clear() {
	r.seen.Clear()
	r.order = nil
}

func (r *returningColumns) IsEmpty() bool {
	return len(r.order) == 0
}

func (r *returningColumns) Values() []string {
	values := make([]string, len(r.order))
	copy(values, r.order)

	return values
}

package search

import "github.com/coldsmirk/vef-framework-go/orm"

func Applier[T any]() func(T) orm.ApplyFunc[orm.ConditionBuilder] {
	f := NewFor[T]()

	return func(value T) orm.ApplyFunc[orm.ConditionBuilder] {
		return func(cb orm.ConditionBuilder) {
			f.Apply(cb, value)
		}
	}
}

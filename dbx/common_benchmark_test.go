package dbx

import "testing"

// benchResult prevents compiler optimizations from eliminating the result.
var benchResult string

func BenchmarkColumnWithAlias(b *testing.B) {
	alias := "su"
	column := "username"

	b.Run("WithAlias", func(b *testing.B) {
		b.ReportAllocs()

		var r string
		for b.Loop() {
			r = ColumnWithAlias(column, alias)
		}

		benchResult = r
	})

	b.Run("NoAlias", func(b *testing.B) {
		b.ReportAllocs()

		var r string
		for b.Loop() {
			r = ColumnWithAlias(column)
		}

		benchResult = r
	})
}

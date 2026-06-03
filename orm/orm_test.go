package orm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// Compile-time guards: the typed default-enum re-exports must keep their
// internal enum type. A drift to an untyped literal (the StatisticalDefault
// regression) fails to build here before any test runs.
var (
	_ orm.JoinType        = orm.JoinDefault
	_ orm.FuzzyKind       = orm.FuzzyStarts
	_ orm.NullsMode       = orm.NullsDefault
	_ orm.FromDirection   = orm.FromDefault
	_ orm.FrameType       = orm.FrameDefault
	_ orm.FrameBoundKind  = orm.FrameBoundNone
	_ orm.StatisticalMode = orm.StatisticalDefault
	_ orm.ConflictAction  = orm.ConflictDoNothing
	_ orm.DateTimeUnit    = orm.UnitYear
)

// TestEnumDefaults asserts the re-exported default enum constants keep both the
// correct type and the zero value of their underlying enum, catching silent
// facade drift between the public alias file and internal/orm.
func TestEnumDefaults(t *testing.T) {
	t.Run("StatisticalDefault", func(t *testing.T) {
		assert.Equal(t, orm.StatisticalMode(0), orm.StatisticalDefault, "StatisticalDefault must alias the zero-value StatisticalMode, not an untyped string")
		assert.Equal(t, "", orm.StatisticalDefault.String(), "zero-value StatisticalMode renders as the empty SQL modifier")
	})
	t.Run("JoinDefault", func(t *testing.T) {
		assert.Equal(t, orm.JoinType(0), orm.JoinDefault, "JoinDefault must alias the zero-value JoinType")
	})
	t.Run("NullsDefault", func(t *testing.T) {
		assert.Equal(t, orm.NullsMode(0), orm.NullsDefault, "NullsDefault must alias the zero-value NullsMode")
	})
	t.Run("FromDefault", func(t *testing.T) {
		assert.Equal(t, orm.FromDirection(0), orm.FromDefault, "FromDefault must alias the zero-value FromDirection")
	})
	t.Run("FrameDefault", func(t *testing.T) {
		assert.Equal(t, orm.FrameType(0), orm.FrameDefault, "FrameDefault must alias the zero-value FrameType")
	})
}

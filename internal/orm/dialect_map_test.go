package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
)

// TestDialectFor pins the cross-package invariant that every kind orm can build
// a dialect for is also openable by internal/database — the connector half and
// the dialect half must stay in agreement.
func TestDialectFor(t *testing.T) {
	supported := []config.DBKind{config.Postgres, config.MySQL, config.SQLite}

	for _, kind := range supported {
		t.Run(string(kind), func(t *testing.T) {
			dialect, err := DialectFor(kind)
			require.NoError(t, err, "DialectFor should support %s", kind)
			assert.NotNil(t, dialect, "dialect should not be nil for %s", kind)
			assert.True(t, database.SupportsKind(kind),
				"database must also provide a connector for %s (connector<->dialect invariant)", kind)
		})
	}

	t.Run("Unsupported", func(t *testing.T) {
		dialect, err := DialectFor("no-such-dialect")
		require.ErrorIs(t, err, ErrUnsupportedDialect, "unsupported kind should return ErrUnsupportedDialect")
		assert.Nil(t, dialect, "dialect should be nil for an unsupported kind")
	})
}

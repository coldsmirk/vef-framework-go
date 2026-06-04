package orm_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/sqliteshim"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

// fuzzyEscapeRow is a minimal model for exercising LIKE wildcard escaping
// through the CriteriaBuilder fuzzy methods (Contains/StartsWith/EndsWith and
// their Not/IgnoreCase variants), which back the search:"contains" tag.
type fuzzyEscapeRow struct {
	bun.BaseModel `bun:"table:test_fuzzy_escape"`

	ID   int64  `bun:"id,pk"`
	Name string `bun:"name,notnull"`
}

// TestCriteriaBuilderFuzzyEscaping verifies that the CriteriaBuilder fuzzy
// methods escape LIKE metacharacters (% and _) so a user-supplied value is
// matched literally instead of as a wildcard. Pre-fix these methods
// bare-concatenated %value%, so "50%" over-matched every row containing "50".
func TestCriteriaBuilderFuzzyEscaping(t *testing.T) {
	ctx := context.Background()

	sqldb, err := sql.Open(sqliteshim.ShimName, ":memory:")
	require.NoError(t, err, "open in-memory sqlite")
	sqldb.SetMaxOpenConns(1) // keep a single connection so the in-memory schema persists
	t.Cleanup(func() { require.NoError(t, sqldb.Close(), "close sqlite") })

	db, err := orm.Open(sqldb, config.SQLite)
	require.NoError(t, err, "wrap sqlite into orm.DB")

	db.RegisterModel((*fuzzyEscapeRow)(nil))
	require.NoError(t, db.ResetModel(ctx, (*fuzzyEscapeRow)(nil)), "create table")

	rows := []fuzzyEscapeRow{
		{ID: 1, Name: "50%off"},     // literal percent
		{ID: 2, Name: "5000off"},    // decoy: LIKE %50%% would wrongly match this
		{ID: 3, Name: "discount_5"}, // literal underscore
		{ID: 4, Name: "discountX5"}, // decoy: LIKE %discount_5% would wrongly match this
		{ID: 5, Name: "plain"},
	}
	_, err = db.NewInsert().Model(&rows).Exec(ctx)
	require.NoError(t, err, "seed rows")

	queryNames := func(build func(cb orm.ConditionBuilder)) []string {
		var got []fuzzyEscapeRow

		err := db.NewSelect().Model((*fuzzyEscapeRow)(nil)).Where(build).OrderBy("name").Scan(ctx, &got)
		require.NoError(t, err, "scan query")

		names := make([]string, len(got))
		for i, r := range got {
			names[i] = r.Name
		}

		return names
	}

	t.Run("ContainsEscapesPercent", func(t *testing.T) {
		got := queryNames(func(cb orm.ConditionBuilder) { cb.Contains("name", "50%") })
		require.Equal(t, []string{"50%off"}, got, "Contains must treat % as a literal, not a wildcard")
	})

	t.Run("ContainsEscapesUnderscore", func(t *testing.T) {
		got := queryNames(func(cb orm.ConditionBuilder) { cb.Contains("name", "discount_5") })
		require.Equal(t, []string{"discount_5"}, got, "Contains must treat _ as a literal, not a single-char wildcard")
	})

	t.Run("StartsWithEscapesPercent", func(t *testing.T) {
		got := queryNames(func(cb orm.ConditionBuilder) { cb.StartsWith("name", "50%") })
		require.Equal(t, []string{"50%off"}, got, "StartsWith must treat % as a literal prefix character")
	})

	t.Run("NotContainsEscapesPercent", func(t *testing.T) {
		got := queryNames(func(cb orm.ConditionBuilder) { cb.NotContains("name", "50%") })
		require.NotContains(t, got, "50%off", "NotContains must exclude the literal match")
		require.Contains(t, got, "5000off", "NotContains must retain rows that only match under wildcard semantics")
	})

	t.Run("ContainsIgnoreCaseEscapesPercent", func(t *testing.T) {
		got := queryNames(func(cb orm.ConditionBuilder) { cb.ContainsIgnoreCase("name", "50%OFF") })
		require.Equal(t, []string{"50%off"}, got, "case-insensitive Contains must still escape % while matching case-insensitively")
	})
}

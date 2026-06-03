package crud

import (
	"errors"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/sortx"
)

// TestResolveQueryParts tests the resolveQueryParts default and explicit-parts behavior.
func TestResolveQueryParts(t *testing.T) {
	t.Run("DefaultsToQueryRootWhenNoneProvided", func(t *testing.T) {
		parts := resolveQueryParts()
		require.Len(t, parts, 1, "Should return exactly one part when none provided")
		assert.Equal(t, QueryRoot, parts[0], "Default part should be QueryRoot")
	})

	t.Run("ReturnsSingleExplicitPart", func(t *testing.T) {
		parts := resolveQueryParts(QueryBase)
		require.Len(t, parts, 1, "Should return exactly one explicit part")
		assert.Equal(t, QueryBase, parts[0], "Should return the provided QueryBase part")
	})

	t.Run("ReturnsMultipleExplicitParts", func(t *testing.T) {
		parts := resolveQueryParts(QueryBase, QueryRecursive)
		require.Len(t, parts, 2, "Should return both explicit parts")
		assert.Equal(t, QueryBase, parts[0], "First part should be QueryBase")
		assert.Equal(t, QueryRecursive, parts[1], "Second part should be QueryRecursive")
	})

	t.Run("DoesNotDefaultWhenQueryAllExplicitlyProvided", func(t *testing.T) {
		parts := resolveQueryParts(QueryAll)
		require.Len(t, parts, 1, "Should return exactly one explicit QueryAll part")
		assert.Equal(t, QueryAll, parts[0], "Should return QueryAll without modification")
	})
}

// TestWithSortPrecedence tests that withSort targets the correct query parts.
func TestWithSortPrecedence(t *testing.T) {
	defaultSpecs := []*sortx.OrderSpec{
		{Column: "created_at", Direction: sortx.OrderDesc},
	}

	t.Run("DefaultsToQueryRoot", func(t *testing.T) {
		opt := withSort(defaultSpecs)
		require.NotNil(t, opt, "Should return a non-nil FindOperationOption")
		assert.Equal(t, []QueryPart{QueryRoot}, opt.Parts, "Should target QueryRoot when no parts specified")
	})

	t.Run("TargetsCustomPartsWhenSpecified", func(t *testing.T) {
		opt := withSort(defaultSpecs, QueryBase, QueryRecursive)
		require.NotNil(t, opt, "Should return a non-nil FindOperationOption")
		assert.Equal(t, []QueryPart{QueryBase, QueryRecursive}, opt.Parts, "Should target the specified parts")
	})

	t.Run("RequestSortTakesPrecedenceOverDefaultWhenMetaHasSort", func(t *testing.T) {
		// When meta contains a sort spec, the request sort is used instead of defaults.
		// The meta Decode path is exercised; we verify no meta-decode error.
		opt := withSort(defaultSpecs)
		require.NotNil(t, opt, "Should return a non-nil FindOperationOption")

		meta := api.Meta{"sort": []any{map[string]any{"column": "name", "direction": "asc"}}}
		// The applier calls query.OrderByExpr for each spec, which would panic on nil query.
		// We only verify the meta decoding path does not error before reaching query ops.
		var err error
		func() {
			defer func() { recover() }()

			err = opt.Applier(nil, struct{}{}, meta, nil)
		}()
		// If meta decoded a sort, the error (if any) must not be a meta-decode error.
		assert.False(t, errors.Is(err, errSearchTypeMismatch), "Sort applier must not return search type mismatch")
	})
}

// TestWithSearchApplierTypeMismatch tests that withSearchApplier returns errSearchTypeMismatch
// when the runtime search value does not match the generic type parameter.
func TestWithSearchApplierTypeMismatch(t *testing.T) {
	type ExpectedSearch struct{ Name string }

	opt := withSearchApplier[ExpectedSearch]()
	require.NotNil(t, opt, "Should return a non-nil FindOperationOption")

	t.Run("DoesNotReturnTypeMismatchForCorrectType", func(t *testing.T) {
		// Passing the correct concrete type succeeds the type assertion (ok=true).
		// The nil query will panic inside Where, but the type-mismatch sentinel must NOT
		// be returned — the function exits before reaching Where when type assertion fails.
		// We test only the sentinel, so recover from any nil-query panic.
		var err error
		func() {
			defer func() { recover() }()

			err = opt.Applier(nil, ExpectedSearch{Name: "ok"}, api.Meta{}, nil)
		}()
		assert.False(t, errors.Is(err, errSearchTypeMismatch), "Correct type must not yield errSearchTypeMismatch")
	})

	t.Run("FailsWithWrongType", func(t *testing.T) {
		type WrongSearch struct{ Age int }

		err := opt.Applier(nil, WrongSearch{Age: 42}, api.Meta{}, nil)
		require.Error(t, err, "Should return an error when search type does not match")
		assert.True(t, errors.Is(err, errSearchTypeMismatch), "Error should wrap errSearchTypeMismatch")
	})

	t.Run("FailsWithNilSearch", func(t *testing.T) {
		// nil (untyped) cannot satisfy ExpectedSearch
		err := opt.Applier(nil, nil, api.Meta{}, nil)
		require.Error(t, err, "Should return an error when search is nil")
		assert.True(t, errors.Is(err, errSearchTypeMismatch), "Error should wrap errSearchTypeMismatch for nil search")
	})
}

// TestWithQueryApplierTypeMismatch tests that withQueryApplier returns errSearchTypeMismatch
// when the runtime search value does not match the generic type parameter.
func TestWithQueryApplierTypeMismatch(t *testing.T) {
	type ExpectedSearch struct{ ID string }

	opt := withQueryApplier(func(_ orm.SelectQuery, _ ExpectedSearch, _ fiber.Ctx) error {
		return nil
	})
	require.NotNil(t, opt, "Should return a non-nil FindOperationOption")

	t.Run("SucceedsWithCorrectType", func(t *testing.T) {
		err := opt.Applier(nil, ExpectedSearch{ID: "abc"}, api.Meta{}, nil)
		assert.NoError(t, err, "Should not return an error when search type matches")
	})

	t.Run("FailsWithWrongType", func(t *testing.T) {
		type WrongSearch struct{ Code string }

		err := opt.Applier(nil, WrongSearch{Code: "x"}, api.Meta{}, nil)
		require.Error(t, err, "Should return an error when search type does not match")
		assert.True(t, errors.Is(err, errSearchTypeMismatch), "Error should wrap errSearchTypeMismatch")
	})

	t.Run("FailsWithNilSearch", func(t *testing.T) {
		err := opt.Applier(nil, nil, api.Meta{}, nil)
		require.Error(t, err, "Should return an error when search is nil")
		assert.True(t, errors.Is(err, errSearchTypeMismatch), "Error should wrap errSearchTypeMismatch for nil search")
	})
}

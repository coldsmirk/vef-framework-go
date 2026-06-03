package security

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// mockDataScope is a test double for DataScope, used by request_scoped_data_perm_applier_test.
type mockDataScope struct {
	mock.Mock
}

func (m *mockDataScope) Key() string {
	return m.Called().String(0)
}

func (m *mockDataScope) Priority() int {
	return m.Called().Int(0)
}

func (m *mockDataScope) Supports(principal *Principal, table *orm.Table) bool {
	return m.Called(principal, table).Bool(0)
}

func (m *mockDataScope) Apply(principal *Principal, query orm.SelectQuery) error {
	return m.Called(principal, query).Error(0)
}

// TestRequestScopedDataPermApplier covers the branching logic in Apply.
func TestRequestScopedDataPermApplier(t *testing.T) {
	principal := NewUser("user-1", "Alice", "admin")

	t.Run("NilDataScopeSkipsFiltering", func(t *testing.T) {
		// Apply short-circuits before touching the query when dataScope is nil.
		applier := NewRequestScopedDataPermApplier(principal, nil, logger)
		db := testx.NewTestDB(t)
		query := db.NewSelect().Model((*selfScopeModel)(nil))

		err := applier.Apply(query)

		require.NoError(t, err, "nil dataScope should be a no-op")
	})

	t.Run("QueryWithNilTableReturnsError", func(t *testing.T) {
		scope := new(mockDataScope)
		scope.On("Key").Return("test-scope").Maybe()
		applier := NewRequestScopedDataPermApplier(principal, scope, logger)

		db := testx.NewTestDB(t)
		// NewSelect without Model() leaves GetTable() returning nil.
		query := db.NewSelect()

		err := applier.Apply(query)

		require.Error(t, err, "query without model should return error")
		assert.ErrorIs(t, err, ErrQueryModelNotSet, "Should return ErrQueryModelNotSet")
	})

	t.Run("ScopeNotApplicableSkipsFiltering", func(t *testing.T) {
		scope := new(mockDataScope)
		scope.On("Key").Return("test-scope").Maybe()
		scope.On("Supports", principal, mock.Anything).Return(false)
		applier := NewRequestScopedDataPermApplier(principal, scope, logger)

		db := testx.NewTestDB(t)
		query := db.NewSelect().Model((*selfScopeModel)(nil))

		err := applier.Apply(query)

		require.NoError(t, err, "Supports returning false should skip and return nil")
		scope.AssertNotCalled(t, "Apply", mock.Anything, mock.Anything)
	})

	t.Run("ScopeAppliedSuccessfully", func(t *testing.T) {
		scope := new(mockDataScope)
		scope.On("Key").Return("test-scope").Maybe()
		scope.On("Supports", principal, mock.Anything).Return(true)
		scope.On("Apply", principal, mock.Anything).Return(nil)
		applier := NewRequestScopedDataPermApplier(principal, scope, logger)

		db := testx.NewTestDB(t)
		query := db.NewSelect().Model((*selfScopeModel)(nil))

		err := applier.Apply(query)

		require.NoError(t, err, "Scope.Apply returning nil should propagate as nil")
		scope.AssertCalled(t, "Apply", principal, mock.Anything)
	})

	t.Run("ScopeApplyErrorIsWrapped", func(t *testing.T) {
		scopeErr := errors.New("scope failure")
		scope := new(mockDataScope)
		scope.On("Key").Return("test-scope")
		scope.On("Supports", principal, mock.Anything).Return(true)
		scope.On("Apply", principal, mock.Anything).Return(scopeErr)
		applier := NewRequestScopedDataPermApplier(principal, scope, logger)

		db := testx.NewTestDB(t)
		query := db.NewSelect().Model((*selfScopeModel)(nil))

		err := applier.Apply(query)

		require.Error(t, err, "Scope.Apply error should be propagated")
		assert.ErrorIs(t, err, scopeErr, "Wrapped error should contain original scope error")
	})
}

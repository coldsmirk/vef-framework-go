package security

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/result"
)

// ─── Mock implementations ───

type MockDepartmentLoader struct {
	LoadDepartmentsFn func(ctx context.Context, principal *Principal) ([]DepartmentOption, error)
}

func (m *MockDepartmentLoader) LoadDepartments(ctx context.Context, principal *Principal) ([]DepartmentOption, error) {
	return m.LoadDepartmentsFn(ctx, principal)
}

type MockDepartmentSelector struct {
	SelectDepartmentFn func(ctx context.Context, principal *Principal, departmentID string) (*Principal, error)
}

func (m *MockDepartmentSelector) SelectDepartment(ctx context.Context, principal *Principal, departmentID string) (*Principal, error) {
	return m.SelectDepartmentFn(ctx, principal, departmentID)
}

// ─── Constructor validation ───

func TestNewDepartmentSelectionChallengeProvider(t *testing.T) {
	validLoader := &MockDepartmentLoader{LoadDepartmentsFn: func(context.Context, *Principal) ([]DepartmentOption, error) { return nil, nil }}
	validSelector := &MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, _ string) (*Principal, error) { return p, nil }}

	t.Run("MissingLoader", func(t *testing.T) {
		assert.PanicsWithValue(t, "security: DepartmentLoader is required", func() {
			NewDepartmentSelectionChallengeProvider(nil, validSelector)
		}, "Should panic when loader is nil")
	})

	t.Run("MissingSelector", func(t *testing.T) {
		assert.PanicsWithValue(t, "security: DepartmentSelector is required", func() {
			NewDepartmentSelectionChallengeProvider(validLoader, nil)
		}, "Should panic when selector is nil")
	})

	t.Run("ValidConfig", func(t *testing.T) {
		assert.NotPanics(t, func() {
			NewDepartmentSelectionChallengeProvider(validLoader, validSelector)
		}, "Should not panic with valid config")
	})
}

// ─── Type and Order ───

func TestDepartmentSelectionChallengeProviderTypeAndOrder(t *testing.T) {
	provider := NewDepartmentSelectionChallengeProvider(
		&MockDepartmentLoader{LoadDepartmentsFn: func(context.Context, *Principal) ([]DepartmentOption, error) { return nil, nil }},
		&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, _ string) (*Principal, error) { return p, nil }},
	)

	t.Run("Type", func(t *testing.T) {
		assert.Equal(t, "department_selection", provider.Type(), "Should return department_selection type")
	})

	t.Run("Order", func(t *testing.T) {
		assert.Equal(t, 500, provider.Order(), "Should return default order 500")
	})
}

// ─── Evaluate ───

func TestDepartmentSelectionChallengeProviderEvaluate(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")

	t.Run("NoDepartments", func(t *testing.T) {
		provider := NewDepartmentSelectionChallengeProvider(
			&MockDepartmentLoader{LoadDepartmentsFn: func(context.Context, *Principal) ([]DepartmentOption, error) {
				return []DepartmentOption{}, nil
			}},
			&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, _ string) (*Principal, error) { return p, nil }},
		)

		challenge, err := provider.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error for empty departments")
		assert.Nil(t, challenge, "Should return nil challenge for empty departments")
	})

	t.Run("NilDepartments", func(t *testing.T) {
		provider := NewDepartmentSelectionChallengeProvider(
			&MockDepartmentLoader{LoadDepartmentsFn: func(context.Context, *Principal) ([]DepartmentOption, error) { return nil, nil }},
			&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, _ string) (*Principal, error) { return p, nil }},
		)

		challenge, err := provider.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error for nil departments")
		assert.Nil(t, challenge, "Should return nil challenge for nil departments")
	})

	t.Run("MultipleDepartments", func(t *testing.T) {
		departments := []DepartmentOption{
			{ID: "d1", Name: "Engineering"},
			{ID: "d2", Name: "Marketing"},
		}
		provider := NewDepartmentSelectionChallengeProvider(
			&MockDepartmentLoader{LoadDepartmentsFn: func(context.Context, *Principal) ([]DepartmentOption, error) {
				return departments, nil
			}},
			&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, _ string) (*Principal, error) { return p, nil }},
		)

		challenge, err := provider.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error for multiple departments")
		require.NotNil(t, challenge, "Should return challenge for multiple departments")
		assert.Equal(t, ChallengeTypeDepartmentSelection, challenge.Type, "Challenge type should be department_selection")
		assert.True(t, challenge.Required, "Challenge should be marked as required")
		challengeData, ok := challenge.Data.(*DepartmentSelectionChallengeData)
		require.True(t, ok, "Challenge data should be *DepartmentSelectionChallengeData")
		assert.Equal(t, departments, challengeData.Departments, "Challenge data should contain departments")
	})

	t.Run("SingleDepartment", func(t *testing.T) {
		departments := []DepartmentOption{{ID: "d1", Name: "Engineering"}}
		provider := NewDepartmentSelectionChallengeProvider(
			&MockDepartmentLoader{LoadDepartmentsFn: func(context.Context, *Principal) ([]DepartmentOption, error) {
				return departments, nil
			}},
			&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, _ string) (*Principal, error) { return p, nil }},
		)

		challenge, err := provider.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error for single department")
		require.NotNil(t, challenge, "Should return challenge even for single department")
		challengeData, ok := challenge.Data.(*DepartmentSelectionChallengeData)
		require.True(t, ok, "Challenge data should be *DepartmentSelectionChallengeData")
		assert.Len(t, challengeData.Departments, 1, "Should contain one department")
	})

	t.Run("LoaderError", func(t *testing.T) {
		loadErr := errors.New("load failed")
		provider := NewDepartmentSelectionChallengeProvider(
			&MockDepartmentLoader{LoadDepartmentsFn: func(context.Context, *Principal) ([]DepartmentOption, error) { return nil, loadErr }},
			&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, _ string) (*Principal, error) { return p, nil }},
		)

		challenge, err := provider.Evaluate(ctx, principal)

		require.ErrorIs(t, err, loadErr, "Should propagate loader error")
		assert.Nil(t, challenge, "Should return nil challenge on loader error")
	})

	t.Run("LoaderReturnsDataAndError", func(t *testing.T) {
		loadErr := errors.New("partial failure")
		departments := []DepartmentOption{{ID: "d1", Name: "Engineering"}}
		provider := NewDepartmentSelectionChallengeProvider(
			&MockDepartmentLoader{LoadDepartmentsFn: func(context.Context, *Principal) ([]DepartmentOption, error) {
				return departments, loadErr
			}},
			&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, _ string) (*Principal, error) { return p, nil }},
		)

		challenge, err := provider.Evaluate(ctx, principal)

		require.ErrorIs(t, err, loadErr, "Should propagate error even when departments are non-empty")
		assert.Nil(t, challenge, "Should discard departments when error is present")
	})
}

// ─── Resolve ───

func TestDepartmentSelectionChallengeProviderResolve(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")
	noopLoader := &MockDepartmentLoader{LoadDepartmentsFn: func(context.Context, *Principal) ([]DepartmentOption, error) { return nil, nil }}

	t.Run("ValidSelection", func(t *testing.T) {
		var receivedDeptID string

		provider := NewDepartmentSelectionChallengeProvider(
			noopLoader,
			&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, deptID string) (*Principal, error) {
				receivedDeptID = deptID

				return p, nil
			}},
		)

		resolved, err := provider.Resolve(ctx, principal, "d1")

		require.NoError(t, err, "Should not return error for valid selection")
		assert.Same(t, principal, resolved, "Should return principal from selector")
		assert.Equal(t, "d1", receivedDeptID, "Should pass department ID to selector")
	})

	t.Run("ResponseNotString", func(t *testing.T) {
		provider := NewDepartmentSelectionChallengeProvider(
			noopLoader,
			&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, _ string) (*Principal, error) { return p, nil }},
		)

		_, err := provider.Resolve(ctx, principal, 12345)

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error for non-string response")
		assert.Equal(t, result.ErrCodeDepartmentRequired, resErr.Code, "Should return department required error")
	})

	t.Run("ResponseNil", func(t *testing.T) {
		provider := NewDepartmentSelectionChallengeProvider(
			noopLoader,
			&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, _ string) (*Principal, error) { return p, nil }},
		)

		_, err := provider.Resolve(ctx, principal, nil)

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error for nil response")
		assert.Equal(t, result.ErrCodeDepartmentRequired, resErr.Code, "Should return department required error")
	})

	t.Run("ResponseEmpty", func(t *testing.T) {
		provider := NewDepartmentSelectionChallengeProvider(
			noopLoader,
			&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, _ string) (*Principal, error) { return p, nil }},
		)

		_, err := provider.Resolve(ctx, principal, "")

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error for empty response")
		assert.Equal(t, result.ErrCodeDepartmentRequired, resErr.Code, "Should return department required error")
	})

	t.Run("WhitespaceDepartmentID", func(t *testing.T) {
		var receivedDeptID string

		provider := NewDepartmentSelectionChallengeProvider(
			noopLoader,
			&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, deptID string) (*Principal, error) {
				receivedDeptID = deptID

				return p, nil
			}},
		)

		resolved, err := provider.Resolve(ctx, principal, "  ")

		require.NoError(t, err, "Should not return error for whitespace-only department ID")
		assert.Same(t, principal, resolved, "Should return principal when selector succeeds")
		assert.Equal(t, "  ", receivedDeptID, "Should pass whitespace department ID as-is to selector")
	})

	t.Run("SelectorError", func(t *testing.T) {
		selectErr := errors.New("invalid department")
		provider := NewDepartmentSelectionChallengeProvider(
			noopLoader,
			&MockDepartmentSelector{SelectDepartmentFn: func(context.Context, *Principal, string) (*Principal, error) { return nil, selectErr }},
		)

		_, err := provider.Resolve(ctx, principal, "invalid")

		require.ErrorIs(t, err, selectErr, "Should propagate selector error")
	})

	t.Run("PrincipalPassthrough", func(t *testing.T) {
		var loaderPrincipal, selectorPrincipal *Principal

		provider := NewDepartmentSelectionChallengeProvider(
			&MockDepartmentLoader{LoadDepartmentsFn: func(_ context.Context, p *Principal) ([]DepartmentOption, error) {
				loaderPrincipal = p

				return []DepartmentOption{{ID: "d1", Name: "Engineering"}}, nil
			}},
			&MockDepartmentSelector{SelectDepartmentFn: func(_ context.Context, p *Principal, _ string) (*Principal, error) {
				selectorPrincipal = p

				return p, nil
			}},
		)

		_, _ = provider.Evaluate(ctx, principal)
		_, _ = provider.Resolve(ctx, principal, "d1")

		assert.Same(t, principal, loaderPrincipal, "Should pass the same principal to loader")
		assert.Same(t, principal, selectorPrincipal, "Should pass the same principal to selector")
	})

	t.Run("PrincipalEnriched", func(t *testing.T) {
		enrichedPrincipal := NewUser("u1", "Alice")
		enrichedPrincipal.Details = map[string]any{"department": "Engineering"}
		provider := NewDepartmentSelectionChallengeProvider(
			noopLoader,
			&MockDepartmentSelector{SelectDepartmentFn: func(context.Context, *Principal, string) (*Principal, error) {
				return enrichedPrincipal, nil
			}},
		)

		resolved, err := provider.Resolve(ctx, principal, "d1")

		require.NoError(t, err, "Should not return error for valid selection")
		assert.Same(t, enrichedPrincipal, resolved, "Should return the enriched principal from selector")
		assert.NotSame(t, principal, resolved, "Enriched principal should differ from input principal")
	})
}

// ─── Interface compliance ───

func TestDepartmentSelectionInterfaceCompliance(t *testing.T) {
	t.Run("ImplementsChallengeProvider", func(*testing.T) {
		var _ ChallengeProvider = (*DepartmentSelectionChallengeProvider)(nil)
	})
}

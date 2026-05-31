package crud_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

func TestFindSetupAuditUserNames(t *testing.T) {
	db := testx.NewTestDB(t)

	tests := []struct {
		name      string
		specs     func() []api.OperationSpec
		expectErr error
	}{
		{
			name: "Model without primary key",
			specs: func() []api.OperationSpec {
				fa := crud.NewFindAll[NoPKModel, struct{}]()
				fa.WithAuditUserNames((*struct{})(nil))

				return fa.Public().Provide()
			},
			expectErr: crud.ErrModelNoPrimaryKey,
		},
		{
			name: "Composite primary key model",
			specs: func() []api.OperationSpec {
				fa := crud.NewFindAll[CompositePKModel, struct{}]()
				fa.WithAuditUserNames((*struct{})(nil))

				return fa.Public().Provide()
			},
			expectErr: crud.ErrAuditUserCompositePK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := tt.specs()
			require.Len(t, specs, 1, "Find operation should expose exactly one operation spec")

			err := callHandlerFactory(t, specs[0].Handler, db)
			assert.ErrorIs(t, err, tt.expectErr, "Find setup should return the expected audit user configuration error")
		})
	}
}

func TestFindWithOptions(t *testing.T) {
	fa := crud.NewFindAll[orm.FullAuditedModel, struct{}]().
		WithOptions(&crud.FindOperationOption{
			Parts: []crud.QueryPart{crud.QueryRoot},
		}).
		Public()

	specs := fa.Provide()
	assert.Len(t, specs, 1, "Find operation with options should expose exactly one operation spec")
}

package command

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

func TestValidateFlowEnums(t *testing.T) {
	t.Parallel()

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()

		err := validateFlowEnums(approval.BindingStandalone, nil)
		assert.NoError(t, err, "Known binding mode should pass")
	})

	t.Run("BindingMode", func(t *testing.T) {
		t.Parallel()

		err := validateFlowEnums("bogus", nil)
		assert.ErrorIs(t, err, shared.ErrInvalidBindingMode, "Unknown binding mode should be rejected")
	})

	t.Run("InitiatorKind", func(t *testing.T) {
		t.Parallel()

		err := validateFlowEnums(approval.BindingBusiness, []shared.CreateFlowInitiatorCmd{{Kind: "bogus"}})
		assert.ErrorIs(t, err, shared.ErrInvalidInitiatorKind, "Unknown initiator kind should be rejected")
	})
}

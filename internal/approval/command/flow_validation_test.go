package command

import (
	"strings"
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

func TestValidateFlowLabels(t *testing.T) {
	t.Parallel()

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()

		valid := map[string]map[string]string{
			"NilLabels":       nil,
			"EmptyLabels":     {},
			"TypicalLabels":   {"app": "crm", "mobile": "true"},
			"SingleCharKey":   {"a": "x"},
			"MixedSeparators": {"env-2_stage": "x"},
			"EmptyValue":      {"mobile": ""},
			"BoundaryLengths": {strings.Repeat("k", 63): strings.Repeat("v", 256)},
			"MultibyteValue":  {"app": strings.Repeat("值", 256)},
		}
		for name, labels := range valid {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				assert.NoError(t, validateFlowLabels(labels), "Labels %v should pass validation", labels)
			})
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()

		invalid := map[string]map[string]string{
			"EmptyKey":      {"": "x"},
			"DottedKey":     {"app.id": "x"},
			"LeadingDash":   {"-app": "x"},
			"TrailingDash":  {"app-": "x"},
			"WhitespaceKey": {"app id": "x"},
			"KeyTooLong":    {strings.Repeat("k", 64): "x"},
			"ValueTooLong":  {"app": strings.Repeat("v", 257)},
		}
		for name, labels := range invalid {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				err := validateFlowLabels(labels)
				assert.ErrorIs(t, err, shared.ErrInvalidFlowLabel, "Labels %v should be rejected", labels)
			})
		}
	})
}

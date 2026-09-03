package command

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

// builtinInitiatorKinds is the selection index save-time validation runs
// against in production: the three built-in kinds, all of which select IDs.
var builtinInitiatorKinds = map[approval.InitiatorKind]approval.SelectionMode{
	approval.InitiatorUser:       approval.SelectionUser,
	approval.InitiatorRole:       approval.SelectionRole,
	approval.InitiatorDepartment: approval.SelectionDepartment,
}

func TestValidateBindingMode(t *testing.T) {
	t.Parallel()

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, validateBindingMode(approval.BindingStandalone), "Known binding mode should pass")
	})

	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()

		assert.ErrorIs(t, validateBindingMode("bogus"), approval.ErrInvalidBindingMode, "Unknown binding mode should be rejected")
	})
}

func TestValidateInitiatorRules(t *testing.T) {
	t.Parallel()

	t.Run("OpenToEveryoneCarriesNoRules", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, validateInitiatorRules(true, nil, builtinInitiatorKinds),
			"An open flow with no rules is the only coherent open state")
	})

	t.Run("OpenToEveryoneRejectsRules", func(t *testing.T) {
		t.Parallel()

		err := validateInitiatorRules(true, []shared.CreateFlowInitiatorCmd{
			{Kind: approval.InitiatorUser, IDs: []string{"u1"}},
		}, builtinInitiatorKinds)
		assert.ErrorIs(t, err, approval.ErrInitiatorsNotAllowed, "Rules alongside open initiation would display a restriction that does not hold")
	})

	t.Run("RestrictedRequiresRules", func(t *testing.T) {
		t.Parallel()

		assert.ErrorIs(t, validateInitiatorRules(false, nil, builtinInitiatorKinds), approval.ErrInitiatorsRequired,
			"A restricted flow with no rules could be started by nobody")
	})

	t.Run("UnknownKindIsRejected", func(t *testing.T) {
		t.Parallel()

		err := validateInitiatorRules(false, []shared.CreateFlowInitiatorCmd{{Kind: "bogus", IDs: []string{"x"}}}, builtinInitiatorKinds)
		assert.ErrorIs(t, err, approval.ErrInvalidInitiatorKind, "A kind nothing resolves should be rejected")
	})

	t.Run("SelectingKindRequiresIDs", func(t *testing.T) {
		t.Parallel()

		err := validateInitiatorRules(false, []shared.CreateFlowInitiatorCmd{{Kind: approval.InitiatorUser}}, builtinInitiatorKinds)
		assert.ErrorIs(t, err, approval.ErrInitiatorsRequired, "A rule that selects nobody matches nobody")
	})

	t.Run("BlankIDsDoNotCount", func(t *testing.T) {
		t.Parallel()

		err := validateInitiatorRules(false, []shared.CreateFlowInitiatorCmd{
			{Kind: approval.InitiatorUser, IDs: []string{"  ", ""}},
		}, builtinInitiatorKinds)
		assert.ErrorIs(t, err, approval.ErrInitiatorsRequired, "Blank IDs select nobody just as an empty list does")
	})

	// A host kind that resolves from the applicant alone — "any ward
	// supervisor" — carries no IDs by design, and must not be held to the
	// selecting kinds' requirement.
	t.Run("HostKindWithoutSelectionNeedsNoIDs", func(t *testing.T) {
		t.Parallel()

		kinds := map[approval.InitiatorKind]approval.SelectionMode{"ward_supervisor": approval.SelectionNone}

		assert.NoError(t, validateInitiatorRules(false, []shared.CreateFlowInitiatorCmd{{Kind: "ward_supervisor"}}, kinds),
			"A parameterless host kind is a complete rule on its own")
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
				assert.ErrorIs(t, err, approval.ErrInvalidFlowLabel, "Labels %v should be rejected", labels)
			})
		}
	})
}

package command

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

func TestValidateBusinessIdentifiers(t *testing.T) {
	t.Parallel()

	ptr := func(s string) *string { return &s }

	t.Run("StandaloneSkipped", func(t *testing.T) {
		t.Parallel()

		err := validateBusinessIdentifiers(approval.BindingStandalone, ptr("orders; DROP TABLE--"), nil, nil)
		assert.NoError(t, err, "Standalone mode should bypass identifier checks")
	})

	t.Run("ValidIdentifiers", func(t *testing.T) {
		t.Parallel()

		err := validateBusinessIdentifiers(approval.BindingBusiness, ptr("orders"), ptr("id"), ptr("status"))
		assert.NoError(t, err, "Plain identifiers should pass")
	})

	t.Run("EmptyTolerated", func(t *testing.T) {
		t.Parallel()
		// DefaultHook handles blank table/pk/status separately; the
		// validator only rejects non-empty values that aren't SQL-safe.
		err := validateBusinessIdentifiers(approval.BindingBusiness, nil, nil, nil)
		assert.NoError(t, err, "Nil identifiers should not trigger rejection")
	})

	t.Run("RejectInjection", func(t *testing.T) {
		t.Parallel()

		err := validateBusinessIdentifiers(approval.BindingBusiness, ptr("orders; DROP TABLE apv_instance --"), ptr("id"), ptr("status"))
		assert.ErrorIs(t, err, shared.ErrInvalidBusinessIdentifier, "Identifier with semicolon should be rejected")
	})

	t.Run("RejectQuotedIdentifier", func(t *testing.T) {
		t.Parallel()

		err := validateBusinessIdentifiers(approval.BindingBusiness, ptr(`"orders"`), ptr("id"), ptr("status"))
		assert.ErrorIs(t, err, shared.ErrInvalidBusinessIdentifier, "Quoted identifier should be rejected")
	})

	t.Run("RejectOverlongIdentifier", func(t *testing.T) {
		t.Parallel()

		long := make([]byte, 100)
		for i := range long {
			long[i] = 'a'
		}

		err := validateBusinessIdentifiers(approval.BindingBusiness, ptr(string(long)), ptr("id"), ptr("status"))
		assert.ErrorIs(t, err, shared.ErrInvalidBusinessIdentifier, "Identifier over 63 chars should be rejected")
	})
}

func TestValidateBusinessBindingComplete(t *testing.T) {
	t.Parallel()

	ptr := func(s string) *string { return &s }

	t.Run("StandaloneSkipped", func(t *testing.T) {
		t.Parallel()

		err := validateBusinessBindingComplete(approval.BindingStandalone, nil, nil, nil)
		assert.NoError(t, err, "Standalone mode skips the completeness check")
	})

	t.Run("BusinessComplete", func(t *testing.T) {
		t.Parallel()

		err := validateBusinessBindingComplete(approval.BindingBusiness, ptr("orders"), ptr("id"), ptr("status"))
		assert.NoError(t, err, "All three binding fields present should pass")
	})

	t.Run("MissingTable", func(t *testing.T) {
		t.Parallel()

		err := validateBusinessBindingComplete(approval.BindingBusiness, nil, ptr("id"), ptr("status"))
		assert.ErrorIs(t, err, shared.ErrBindingIncomplete, "A missing table must be rejected")
	})

	t.Run("MissingPkField", func(t *testing.T) {
		t.Parallel()

		err := validateBusinessBindingComplete(approval.BindingBusiness, ptr("orders"), nil, ptr("status"))
		assert.ErrorIs(t, err, shared.ErrBindingIncomplete, "A missing pk field must be rejected")
	})

	t.Run("MissingStatusField", func(t *testing.T) {
		t.Parallel()

		err := validateBusinessBindingComplete(approval.BindingBusiness, ptr("orders"), ptr("id"), nil)
		assert.ErrorIs(t, err, shared.ErrBindingIncomplete, "A missing status field must be rejected")
	})

	t.Run("WhitespaceOnlyRejected", func(t *testing.T) {
		t.Parallel()

		err := validateBusinessBindingComplete(approval.BindingBusiness, ptr("orders"), ptr("   "), ptr("status"))
		assert.ErrorIs(t, err, shared.ErrBindingIncomplete, "A whitespace-only value is treated as missing")
	})
}

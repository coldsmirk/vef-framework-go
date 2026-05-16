package command

import (
	"errors"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

// validateBusinessIdentifiers enforces the SQL-identifier whitelist on
// every business binding field whenever BindingMode == BindingBusiness.
// Empty values are tolerated here (the runtime check in DefaultHook rejects
// flows with blank table/pk/status separately). Returns the domain-level
// shared.ErrInvalidBusinessIdentifier so the API surface emits a stable
// error code; the regex itself lives in approval.ValidateBusinessIdentifier
// so binding.DefaultHook can reuse it for defense-in-depth.
func validateBusinessIdentifiers(mode approval.BindingMode, table, pkField, statusField, titleField *string) error {
	if mode != approval.BindingBusiness {
		return nil
	}

	for _, v := range []*string{table, pkField, statusField, titleField} {
		if v == nil {
			continue
		}

		if err := approval.ValidateBusinessIdentifier(*v); err != nil {
			if errors.Is(err, approval.ErrInvalidBusinessIdentifier) {
				return shared.ErrInvalidBusinessIdentifier
			}

			return err
		}
	}

	return nil
}

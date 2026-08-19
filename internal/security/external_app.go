package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/security"
)

// validateExternalAppPolicy enforces the access policy an ExternalAppLoader
// attaches to an external app principal: whether the app is still enabled, and
// whether the request's source address falls inside its whitelist. Every
// framework entry point that authenticates an external app — the API signature
// authenticator and the trust-login gateway — runs it, so an app disabled in
// one place is disabled everywhere.
//
// Applications may replace the details type wholesale (see
// security.SetExternalAppDetailsType), in which case there is no policy to
// enforce and the app is admitted: the caller has already proven possession of
// the app's secret, and the alternative would break every deployment carrying
// its own details struct.
func validateExternalAppPolicy(ctx context.Context, principal *security.Principal) error {
	details, ok := principal.Details.(*security.ExternalAppConfig)
	if !ok || details == nil {
		return nil
	}

	if !details.Enabled {
		return security.ErrExternalAppDisabled
	}

	if details.IPWhitelist == "" {
		return nil
	}

	requestIP := contextx.RequestIP(ctx)
	if requestIP == "" {
		// Fail closed: an IP whitelist is configured but the request IP cannot be
		// determined. Allowing the request would silently bypass the control, so
		// deny instead.
		return security.ErrIPNotAllowed
	}

	if validator := security.NewIPWhitelistValidator(details.IPWhitelist); !validator.IsAllowed(requestIP) {
		return security.ErrIPNotAllowed
	}

	return nil
}

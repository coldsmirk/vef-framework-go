package definition

import (
	"context"
	"fmt"
	"net/url"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// OutboundAuthSchemeResolver is the validator's view of the outbound scheme registry;
// keeping it consumer-side avoids an import cycle with the auth package.
type OutboundAuthSchemeResolver interface {
	// Resolve returns the scheme for cfg, ok=false for an unknown name.
	Resolve(cfg *integration.OutboundAuthConfig) (integration.OutboundAuthScheme, bool)
}

// ValidateContract rejects a contract whose input or output schema does not
// compile, so a broken schema fails at save time instead of on the first
// invocation.
func ValidateContract(contract *integration.Contract) error {
	if len(contract.InputSchema) > 0 {
		if _, err := CompileSchema(contract.InputSchema); err != nil {
			return integration.ErrInvalidSchema(err.Error())
		}
	}

	if len(contract.OutputSchema) > 0 {
		if _, err := CompileSchema(contract.OutputSchema); err != nil {
			return integration.ErrInvalidSchema(err.Error())
		}
	}

	return nil
}

// ValidateSystem rejects a system whose base URL is not absolute or whose
// auth config references an unknown scheme or fails the scheme's own
// parameter validation. Auth params must already be in their persisted form
// (EncryptOutboundAuth applied) so masked placeholders have been resolved.
func ValidateSystem(registry OutboundAuthSchemeResolver, codec *SecretCodec, system *integration.System) error {
	if system.BaseURL != "" {
		parsed, err := url.Parse(system.BaseURL)
		if err != nil || !parsed.IsAbs() {
			return integration.ErrInvalidBaseURL
		}
	}

	if system.DataSource != nil {
		if system.DataSource.Kind == "" {
			return integration.ErrInvalidDataSource("kind is required")
		}

		if !system.DataSource.Mode.IsValid() {
			return integration.ErrInvalidDataSource(fmt.Sprintf("unknown mode %q", system.DataSource.Mode))
		}
	}

	if err := validateOutboundEnvelope(system); err != nil {
		return err
	}

	if system.OutboundAuth == nil {
		return nil
	}

	scheme, ok := registry.Resolve(system.OutboundAuth)
	if !ok {
		return integration.ErrUnknownAuthScheme(system.OutboundAuth.Scheme)
	}

	decrypted, err := codec.DecryptOutboundAuth(scheme, system.OutboundAuth)
	if err != nil {
		return integration.ErrInvalidAuthParams(err.Error())
	}

	if _, err := scheme.Apply(decrypted); err != nil {
		return integration.ErrInvalidAuthParams(err.Error())
	}

	return nil
}

// validateOutboundEnvelope rejects an envelope on a system without an HTTP
// transport, an envelope defining no script, and scripts that do not compile.
func validateOutboundEnvelope(system *integration.System) error {
	envelope := system.OutboundEnvelope
	if envelope == nil {
		return nil
	}

	if system.BaseURL == "" {
		return integration.ErrInvalidEnvelope("an outbound envelope requires a base URL")
	}

	if envelope.Request == "" && envelope.Response == "" {
		return integration.ErrInvalidEnvelope("at least one script is required")
	}

	if envelope.Request != "" {
		if _, err := CompileEnvelopeRequestScript(envelope.Request); err != nil {
			return integration.ErrInvalidEnvelope(err.Error())
		}
	}

	if envelope.Response != "" {
		if _, err := CompileEnvelopeResponseScript(envelope.Response); err != nil {
			return integration.ErrInvalidEnvelope(err.Error())
		}
	}

	return nil
}

// ValidateAdapterScript rejects an adapter whose script does not compile.
func ValidateAdapterScript(script string) error {
	if _, err := CompileScript(script); err != nil {
		return integration.ErrInvalidScript(err.Error())
	}

	return nil
}

// ValidateRouteRefs rejects a route referencing a missing contract or
// system. The contract reference is checked here because the column carries
// the empty-string wildcard sentinel and therefore has no foreign key.
func ValidateRouteRefs(ctx context.Context, db orm.DB, route *integration.Route) error {
	if route.ContractID != "" {
		exists, err := db.NewSelect().
			Model((*integration.Contract)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", route.ContractID)
			}).
			Exists(ctx)
		if err != nil {
			return err
		}

		if !exists {
			return integration.ErrInvalidRouteRef
		}
	}

	exists, err := db.NewSelect().
		Model((*integration.System)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", route.SystemID)
		}).
		Exists(ctx)
	if err != nil {
		return err
	}

	if !exists {
		return integration.ErrInvalidRouteRef
	}

	return nil
}

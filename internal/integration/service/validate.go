package service

import (
	"context"
	"net/url"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/auth"
	"github.com/coldsmirk/vef-framework-go/orm"
)

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
// (EncryptAuth applied) so masked placeholders have been resolved.
func ValidateSystem(registry *auth.Registry, codec *SecretCodec, system *integration.System) error {
	if system.BaseURL != "" {
		parsed, err := url.Parse(system.BaseURL)
		if err != nil || !parsed.IsAbs() {
			return integration.ErrInvalidBaseURL
		}
	}

	scheme, ok := registry.Resolve(system.Auth)
	if !ok {
		return integration.ErrUnknownAuthScheme(system.Auth.Scheme)
	}

	if system.Auth == nil {
		return nil
	}

	params, err := codec.DecryptAuth(scheme, system.Auth)
	if err != nil {
		return integration.ErrInvalidAuthParams(err.Error())
	}

	if _, err := scheme.Apply(params); err != nil {
		return integration.ErrInvalidAuthParams(err.Error())
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
// the ” wildcard sentinel and therefore has no foreign key.
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

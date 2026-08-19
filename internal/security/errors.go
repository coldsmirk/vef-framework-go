package security

import "errors"

// errReservedPrincipalRejected marks a reserved-identity rejection raised by the
// framework's own gate rather than by a caller's credential. The outward error
// stays security.ErrReservedPrincipal, whose business code is shared with the
// legitimately countable ErrPrincipalInvalid failures — and result.Error.Is
// compares codes alone — so the login path unwraps this sentinel to keep a
// buggy authenticator from consuming the caller's brute-force budget.
var errReservedPrincipalRejected = errors.New("authenticator resolved a framework-reserved principal")

// Configuration faults raised while building the config-backed IP whitelist
// loader; they surface as fx start-up errors, never through the API.
var (
	// ErrIPWhitelistNameBlank rejects a whitelist declared under a blank name.
	ErrIPWhitelistNameBlank = errors.New("ip whitelist name must not be blank")
	// ErrIPWhitelistEmpty rejects a whitelist with no usable entries, which
	// could never authenticate a request.
	ErrIPWhitelistEmpty = errors.New("ip whitelist must contain at least one IP or CIDR entry")
	// ErrIPWhitelistEntryInvalid rejects an entry that is neither an IP
	// address nor a CIDR range.
	ErrIPWhitelistEntryInvalid = errors.New("ip whitelist entry is neither an IP address nor a CIDR range")
)

// Configuration faults raised while building the config-backed API key and
// basic account loaders; they surface as fx start-up errors, never through
// the API.
var (
	// ErrAPIKeyNameBlank rejects an API key declared under a blank name.
	ErrAPIKeyNameBlank = errors.New("api key name must not be blank")
	// ErrAPIKeyValueBlank rejects an API key with a blank key value, which
	// could never authenticate a request.
	ErrAPIKeyValueBlank = errors.New("api key value must not be blank")
	// ErrBasicAccountUsernameBlank rejects a basic account declared under a
	// blank username.
	ErrBasicAccountUsernameBlank = errors.New("basic account username must not be blank")
	// ErrBasicAccountPasswordBlank rejects a basic account with a blank
	// password, which could never authenticate a request.
	ErrBasicAccountPasswordBlank = errors.New("basic account password must not be blank")
)

// Configuration faults raised while building the trust-login gateway; they
// surface as fx start-up errors, never through the API.
var (
	// ErrTrustLoginExternalAppLoaderMissing rejects an enabled gateway with no
	// security.ExternalAppLoader, which supplies the per-app signing secret
	// every handoff is verified against.
	ErrTrustLoginExternalAppLoaderMissing = errors.New("vef.security.trust_login is enabled but no security.ExternalAppLoader is registered to supply app signing secrets")
	// ErrTrustLoginUserResolutionMissing rejects an enabled gateway that can
	// map no external identifier onto a local user: it needs either a
	// security.TrustUserResolver or a security.UserLoader.
	ErrTrustLoginUserResolutionMissing = errors.New(
		"vef.security.trust_login is enabled but neither a security.TrustUserResolver nor a security.UserLoader is registered to resolve external users",
	)
)

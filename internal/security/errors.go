package security

import "errors"

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

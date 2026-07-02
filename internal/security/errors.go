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

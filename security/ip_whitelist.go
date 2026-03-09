package security

import (
	"net"
	"strings"

	"github.com/coldsmirk/go-collections"
)

// IPWhitelistValidator validates IP addresses against a whitelist.
// It supports both individual IP addresses and CIDR notation.
type IPWhitelistValidator struct {
	// networks contains parsed CIDR networks for range matching.
	networks []*net.IPNet
	// ips contains individual IP addresses for O(1) exact matching.
	ips collections.Set[string]
	// isEmpty indicates whether the whitelist is empty (allow all).
	isEmpty bool
	// invalid indicates whitelist parsing failed; invalid configuration denies all requests.
	invalid bool
}

// NewIPWhitelistValidator creates a new IP whitelist validator from a comma-separated string.
// Supports individual IP addresses (e.g., "192.168.1.1") and CIDR notation (e.g., "192.168.1.0/24").
// An empty whitelist means all IPs are allowed.
func NewIPWhitelistValidator(whitelist string) *IPWhitelistValidator {
	validator := &IPWhitelistValidator{
		ips: collections.NewHashSet[string](),
	}

	whitelist = strings.TrimSpace(whitelist)
	if whitelist == "" {
		validator.isEmpty = true

		return validator
	}

	hasValidEntry := false

	for entry := range strings.SplitSeq(whitelist, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Handle CIDR notation
		if strings.Contains(entry, "/") {
			if _, network, err := net.ParseCIDR(entry); err != nil {
				logger.Warnf("Failed to parse CIDR %s: %v", entry, err)

				validator.invalid = true
			} else {
				validator.networks = append(validator.networks, network)
				hasValidEntry = true
			}

			continue
		}

		// Handle individual IP address
		if ip := net.ParseIP(entry); ip != nil {
			validator.ips.Add(ip.String())

			hasValidEntry = true
		} else {
			logger.Warnf("Failed to parse IP %s", entry)

			validator.invalid = true
		}
	}

	validator.isEmpty = !validator.invalid && !hasValidEntry

	return validator
}

// IsAllowed checks if the given IP address is in the whitelist.
func (v *IPWhitelistValidator) IsAllowed(ipStr string) bool {
	if v.isEmpty {
		return true
	}

	if v.invalid {
		return false
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	if v.ips.Contains(ip.String()) {
		return true
	}

	for _, network := range v.networks {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// IsEmpty returns true if the whitelist is empty (no restrictions).
func (v *IPWhitelistValidator) IsEmpty() bool {
	return v.isEmpty
}

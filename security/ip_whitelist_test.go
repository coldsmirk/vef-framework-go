package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewIPWhitelistValidator tests new IP whitelist validator functionality.
func TestNewIPWhitelistValidator(t *testing.T) {
	t.Run("EmptyWhitelist", func(t *testing.T) {
		validator := NewIPWhitelistValidator("")

		assert.True(t, validator.IsEmpty(), "Empty whitelist should be empty")
		assert.True(t, validator.IsAllowed("192.168.1.1"), "Empty whitelist should allow any IP")
		assert.True(t, validator.IsAllowed("10.0.0.1"), "Empty whitelist should allow any IP")
		assert.True(t, validator.IsAllowed("::1"), "Empty whitelist should allow IPv6 localhost")
	})

	t.Run("WhitespaceOnlyWhitelist", func(t *testing.T) {
		validator := NewIPWhitelistValidator("   ")

		assert.True(t, validator.IsEmpty(), "Whitespace-only whitelist should be empty")
		assert.True(t, validator.IsAllowed("192.168.1.1"), "Whitespace-only whitelist should allow any IP")
	})

	t.Run("SingleIP", func(t *testing.T) {
		validator := NewIPWhitelistValidator("192.168.1.100")

		assert.False(t, validator.IsEmpty(), "Single IP whitelist should not be empty")
	})

	t.Run("AllInvalidEntries", func(t *testing.T) {
		validator := NewIPWhitelistValidator("invalid, also-invalid, not-an-ip")

		assert.False(t, validator.IsEmpty(), "Invalid whitelist should not be treated as empty")
		assert.False(t, validator.IsAllowed("192.168.1.1"), "Invalid whitelist should deny all IPs")
	})
}

// TestIPWhitelistValidatorIsEmpty tests IPWhitelistValidator IsEmpty scenarios.
func TestIPWhitelistValidatorIsEmpty(t *testing.T) {
	tests := []struct {
		name      string
		whitelist string
		expected  bool
	}{
		{"EmptyString", "", true},
		{"WhitespaceOnly", "   ", true},
		{"SingleValidIP", "192.168.1.100", false},
		{"MultipleValidIPs", "192.168.1.100, 10.0.0.1", false},
		{"ValidCIDR", "192.168.1.0/24", false},
		{"AllInvalidEntries", "invalid, not-an-ip", false},
		{"MixedValidAndInvalid", "invalid, 192.168.1.100", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewIPWhitelistValidator(tt.whitelist)

			assert.Equal(t, tt.expected, validator.IsEmpty(), "IsEmpty should return %v for whitelist %q", tt.expected, tt.whitelist)
		})
	}
}

// TestIPWhitelistValidatorIsAllowedSingleIP tests IPWhitelistValidator IsAllowed single IP scenarios.
func TestIPWhitelistValidatorIsAllowedSingleIP(t *testing.T) {
	validator := NewIPWhitelistValidator("192.168.1.100")

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"ExactMatch", "192.168.1.100", true},
		{"DifferentLastOctet", "192.168.1.101", false},
		{"CompletelyDifferentIP", "10.0.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, validator.IsAllowed(tt.ip), "IP %s should be allowed=%v", tt.ip, tt.expected)
		})
	}
}

// TestIPWhitelistValidatorIsAllowedMultipleIPs tests IPWhitelistValidator IsAllowed multiple IPs scenarios.
func TestIPWhitelistValidatorIsAllowedMultipleIPs(t *testing.T) {
	validator := NewIPWhitelistValidator("192.168.1.100, 192.168.1.101, 10.0.0.1")

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"FirstIP", "192.168.1.100", true},
		{"SecondIP", "192.168.1.101", true},
		{"ThirdIP", "10.0.0.1", true},
		{"UnlistedIP", "192.168.1.102", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, validator.IsAllowed(tt.ip), "IP %s should be allowed=%v", tt.ip, tt.expected)
		})
	}
}

// TestIPWhitelistValidatorIsAllowedCIDR tests IPWhitelistValidator IsAllowed CIDR scenarios.
func TestIPWhitelistValidatorIsAllowedCIDR(t *testing.T) {
	t.Run("ClassCNetwork", func(t *testing.T) {
		validator := NewIPWhitelistValidator("192.168.1.0/24")

		tests := []struct {
			name     string
			ip       string
			expected bool
		}{
			{"FirstUsableIP", "192.168.1.1", true},
			{"MiddleIP", "192.168.1.100", true},
			{"LastUsableIP", "192.168.1.254", true},
			{"NetworkAddress", "192.168.1.0", true},
			{"BroadcastAddress", "192.168.1.255", true},
			{"AdjacentNetwork", "192.168.2.1", false},
			{"DifferentNetwork", "10.0.0.1", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.expected, validator.IsAllowed(tt.ip), "IP %s should be allowed=%v in /24 network", tt.ip, tt.expected)
			})
		}
	})

	t.Run("SingleHost32", func(t *testing.T) {
		validator := NewIPWhitelistValidator("192.168.1.100/32")

		assert.True(t, validator.IsAllowed("192.168.1.100"), "Exact IP should be allowed in /32 CIDR")
		assert.False(t, validator.IsAllowed("192.168.1.101"), "Adjacent IP should not be allowed in /32 CIDR")
	})

	t.Run("LargeNetwork8", func(t *testing.T) {
		validator := NewIPWhitelistValidator("10.0.0.0/8")

		assert.True(t, validator.IsAllowed("10.0.0.1"), "First IP should be allowed in /8 network")
		assert.True(t, validator.IsAllowed("10.255.255.254"), "Last usable IP should be allowed in /8 network")
		assert.False(t, validator.IsAllowed("11.0.0.1"), "IP outside /8 network should not be allowed")
	})

	t.Run("ClassBNetwork16", func(t *testing.T) {
		validator := NewIPWhitelistValidator("172.16.0.0/16")

		assert.True(t, validator.IsAllowed("172.16.0.1"), "First IP should be allowed in /16 network")
		assert.True(t, validator.IsAllowed("172.16.255.255"), "Last IP should be allowed in /16 network")
		assert.False(t, validator.IsAllowed("172.17.0.1"), "IP outside /16 network should not be allowed")
	})
}

// TestIPWhitelistValidatorIsAllowedMixedIPsAndCIDR tests IPWhitelistValidator IsAllowed mixed IPs and CIDR scenarios.
func TestIPWhitelistValidatorIsAllowedMixedIPsAndCIDR(t *testing.T) {
	validator := NewIPWhitelistValidator("192.168.1.0/24, 10.0.0.100, 172.16.0.0/16")

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"IPInFirstCIDR", "192.168.1.50", true},
		{"SingleWhitelistedIP", "10.0.0.100", true},
		{"AdjacentToSingleIP", "10.0.0.101", false},
		{"IPInSecondCIDR", "172.16.1.1", true},
		{"IPAtEndOfSecondCIDR", "172.16.255.255", true},
		{"IPOutsideSecondCIDR", "172.17.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, validator.IsAllowed(tt.ip), "IP %s should be allowed=%v in mixed whitelist", tt.ip, tt.expected)
		})
	}
}

// TestIPWhitelistValidatorIsAllowedIPv6 tests IPWhitelistValidator IsAllowed IPv6 scenarios.
func TestIPWhitelistValidatorIsAllowedIPv6(t *testing.T) {
	t.Run("SingleIPv6AndCIDR", func(t *testing.T) {
		validator := NewIPWhitelistValidator("::1, 2001:db8::/32")

		tests := []struct {
			name     string
			ip       string
			expected bool
		}{
			{"IPv6Localhost", "::1", true},
			{"IPv6InCIDR", "2001:db8::1", true},
			{"IPv6InCIDRDifferentSubnet", "2001:db8:ffff::1", true},
			{"IPv6OutsideCIDR", "2001:db9::1", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.expected, validator.IsAllowed(tt.ip), "IPv6 %s should be allowed=%v", tt.ip, tt.expected)
			})
		}
	})

	t.Run("SingleHost128", func(t *testing.T) {
		validator := NewIPWhitelistValidator("2001:db8::1/128")

		assert.True(t, validator.IsAllowed("2001:db8::1"), "Exact IPv6 should be allowed in /128 CIDR")
		assert.False(t, validator.IsAllowed("2001:db8::2"), "Adjacent IPv6 should not be allowed in /128 CIDR")
	})

	t.Run("FullIPv6AddressFormat", func(t *testing.T) {
		validator := NewIPWhitelistValidator("2001:0db8:0000:0000:0000:0000:0000:0001")

		assert.True(t, validator.IsAllowed("2001:db8::1"), "Compressed IPv6 should match full format")
	})

	t.Run("IPv6LinkLocal", func(t *testing.T) {
		validator := NewIPWhitelistValidator("fe80::/10")

		assert.True(t, validator.IsAllowed("fe80::1"), "Link-local IPv6 should be allowed")
		assert.True(t, validator.IsAllowed("fe80::abcd:1234"), "Link-local IPv6 with suffix should be allowed")
		assert.False(t, validator.IsAllowed("2001:db8::1"), "Non-link-local IPv6 should not be allowed")
	})
}

// TestIPWhitelistValidatorIsAllowedInvalidEntries tests IPWhitelistValidator IsAllowed invalid entries scenarios.
func TestIPWhitelistValidatorIsAllowedInvalidEntries(t *testing.T) {
	t.Run("MixedValidAndInvalid", func(t *testing.T) {
		validator := NewIPWhitelistValidator("invalid, 192.168.1.100, also-invalid")

		assert.False(t, validator.IsEmpty(), "Whitelist should not be treated as empty")
		assert.False(t, validator.IsAllowed("192.168.1.100"), "Any parse error should deny all IPs")
		assert.False(t, validator.IsAllowed("192.168.1.101"), "Any parse error should deny all IPs")
	})

	t.Run("AllInvalidEntries", func(t *testing.T) {
		validator := NewIPWhitelistValidator("invalid, also-invalid, not-an-ip")

		assert.False(t, validator.IsEmpty(), "Invalid whitelist should not be treated as empty")
		assert.False(t, validator.IsAllowed("192.168.1.1"), "Invalid whitelist should deny all IPs")
	})

	t.Run("InvalidCIDRNotation", func(t *testing.T) {
		validator := NewIPWhitelistValidator("192.168.1.0/33, 192.168.1.100")

		assert.False(t, validator.IsEmpty(), "Invalid whitelist should not be treated as empty")
		assert.False(t, validator.IsAllowed("192.168.1.100"), "Any parse error should deny all IPs")
		assert.False(t, validator.IsAllowed("192.168.1.50"), "Any parse error should deny all IPs")
	})

	t.Run("NegativeCIDRPrefix", func(t *testing.T) {
		validator := NewIPWhitelistValidator("192.168.1.0/-1, 192.168.1.100")

		assert.False(t, validator.IsAllowed("192.168.1.100"), "Any parse error should deny all IPs")
	})
}

// TestIPWhitelistValidatorIsAllowedWhitespaceHandling tests IPWhitelistValidator IsAllowed whitespace handling scenarios.
func TestIPWhitelistValidatorIsAllowedWhitespaceHandling(t *testing.T) {
	tests := []struct {
		name      string
		whitelist string
		testIP    string
		expected  bool
	}{
		{"LeadingAndTrailingSpaces", "  192.168.1.100  ", "192.168.1.100", true},
		{"SpacesAroundComma", "192.168.1.100 , 10.0.0.1", "10.0.0.1", true},
		{"MultipleSpaces", "  192.168.1.100  ,  10.0.0.1  ", "192.168.1.100", true},
		{"TabsAndSpaces", "\t192.168.1.100\t", "192.168.1.100", true},
		{"NewlineCharacters", "192.168.1.100\n", "192.168.1.100", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewIPWhitelistValidator(tt.whitelist)

			assert.Equal(t, tt.expected, validator.IsAllowed(tt.testIP), "IP %s should be allowed=%v for whitelist %q", tt.testIP, tt.expected, tt.whitelist)
		})
	}
}

// TestIPWhitelistValidatorIsAllowedInvalidIPInput tests IPWhitelistValidator IsAllowed invalid IP input scenarios.
func TestIPWhitelistValidatorIsAllowedInvalidIPInput(t *testing.T) {
	validator := NewIPWhitelistValidator("192.168.1.100")

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"InvalidString", "not-an-ip", false},
		{"EmptyString", "", false},
		{"PartialIP", "192.168.1", false},
		{"IPWithPort", "192.168.1.100:8080", false},
		{"NegativeOctet", "192.168.1.-1", false},
		{"OctetOutOfRange", "192.168.1.256", false},
		{"TooManyOctets", "192.168.1.100.1", false},
		{"SpecialCharacters", "192.168.1.100!", false},
		{"LeadingZeros", "192.168.001.100", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, validator.IsAllowed(tt.ip), "Invalid IP %q should be allowed=%v", tt.ip, tt.expected)
		})
	}
}

// TestIPWhitelistValidatorIsAllowedEdgeCases tests IPWhitelistValidator IsAllowed edge cases scenarios.
func TestIPWhitelistValidatorIsAllowedEdgeCases(t *testing.T) {
	t.Run("EmptyEntriesBetweenCommas", func(t *testing.T) {
		validator := NewIPWhitelistValidator("192.168.1.100,,10.0.0.1")

		assert.True(t, validator.IsAllowed("192.168.1.100"), "First IP should be allowed")
		assert.True(t, validator.IsAllowed("10.0.0.1"), "Second IP should be allowed")
	})

	t.Run("DuplicateIPs", func(t *testing.T) {
		validator := NewIPWhitelistValidator("192.168.1.100, 192.168.1.100, 192.168.1.100")

		assert.True(t, validator.IsAllowed("192.168.1.100"), "Duplicate IP should be allowed")
		assert.False(t, validator.IsAllowed("192.168.1.101"), "Unlisted IP should not be allowed")
	})

	t.Run("OverlappingCIDRAndSingleIP", func(t *testing.T) {
		validator := NewIPWhitelistValidator("192.168.1.0/24, 192.168.1.100")

		assert.True(t, validator.IsAllowed("192.168.1.100"), "IP in both CIDR and single should be allowed")
		assert.True(t, validator.IsAllowed("192.168.1.50"), "IP only in CIDR should be allowed")
	})

	t.Run("LoopbackAddresses", func(t *testing.T) {
		validator := NewIPWhitelistValidator("127.0.0.1, ::1")

		assert.True(t, validator.IsAllowed("127.0.0.1"), "IPv4 loopback should be allowed")
		assert.True(t, validator.IsAllowed("::1"), "IPv6 loopback should be allowed")
		assert.False(t, validator.IsAllowed("127.0.0.2"), "Non-loopback IP should not be allowed")
	})

	t.Run("PrivateNetworkRanges", func(t *testing.T) {
		validator := NewIPWhitelistValidator("10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16")

		assert.True(t, validator.IsAllowed("10.1.2.3"), "Class A private IP should be allowed")
		assert.True(t, validator.IsAllowed("172.16.1.1"), "Class B private IP start should be allowed")
		assert.True(t, validator.IsAllowed("172.31.255.255"), "Class B private IP end should be allowed")
		assert.True(t, validator.IsAllowed("192.168.100.50"), "Class C private IP should be allowed")
		assert.False(t, validator.IsAllowed("172.32.0.1"), "IP outside Class B private range should not be allowed")
		assert.False(t, validator.IsAllowed("8.8.8.8"), "Public IP should not be allowed")
	})

	t.Run("ZeroAddress", func(t *testing.T) {
		validator := NewIPWhitelistValidator("0.0.0.0/0")

		assert.True(t, validator.IsAllowed("192.168.1.1"), "Any IP should be allowed with 0.0.0.0/0")
		assert.True(t, validator.IsAllowed("8.8.8.8"), "Public IP should be allowed with 0.0.0.0/0")
		assert.True(t, validator.IsAllowed("10.0.0.1"), "Private IP should be allowed with 0.0.0.0/0")
	})

	t.Run("BroadcastAddress", func(t *testing.T) {
		validator := NewIPWhitelistValidator("255.255.255.255")

		assert.True(t, validator.IsAllowed("255.255.255.255"), "Broadcast address should be allowed")
		assert.False(t, validator.IsAllowed("255.255.255.254"), "Adjacent to broadcast should not be allowed")
	})
}

// TestIPWhitelistValidatorIsAllowedSpecialAddresses tests IPWhitelistValidator IsAllowed special addresses scenarios.
func TestIPWhitelistValidatorIsAllowedSpecialAddresses(t *testing.T) {
	t.Run("IPv4MappedIPv6", func(t *testing.T) {
		validator := NewIPWhitelistValidator("::ffff:192.168.1.100")

		assert.True(t, validator.IsAllowed("::ffff:192.168.1.100"), "IPv4-mapped IPv6 should be allowed")
	})

	t.Run("MulticastAddresses", func(t *testing.T) {
		validator := NewIPWhitelistValidator("224.0.0.0/4")

		assert.True(t, validator.IsAllowed("224.0.0.1"), "First multicast IP should be allowed")
		assert.True(t, validator.IsAllowed("239.255.255.255"), "Last multicast IP should be allowed")
		assert.False(t, validator.IsAllowed("240.0.0.1"), "IP outside multicast range should not be allowed")
	})

	t.Run("DocumentationAddresses", func(t *testing.T) {
		validator := NewIPWhitelistValidator("192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24")

		assert.True(t, validator.IsAllowed("192.0.2.1"), "TEST-NET-1 IP should be allowed")
		assert.True(t, validator.IsAllowed("198.51.100.50"), "TEST-NET-2 IP should be allowed")
		assert.True(t, validator.IsAllowed("203.0.113.100"), "TEST-NET-3 IP should be allowed")
		assert.False(t, validator.IsAllowed("192.0.3.1"), "IP outside TEST-NET-1 should not be allowed")
	})
}

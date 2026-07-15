package jsevents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMatchType tests the allowlist pattern semantics.
func TestMatchType(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		eventType string
		want      bool
	}{
		{name: "StarMatchesEverything", pattern: "*", eventType: "a.b.c", want: true},
		{name: "ExactMatch", pattern: "a.b", eventType: "a.b", want: true},
		{name: "ExactMismatch", pattern: "a.b", eventType: "a.c", want: false},
		{name: "WildcardMatchesChild", pattern: "a.*", eventType: "a.b", want: true},
		{name: "WildcardMatchesDeepDescendant", pattern: "a.*", eventType: "a.b.c", want: true},
		{name: "WildcardExcludesRoot", pattern: "a.*", eventType: "a", want: false},
		{name: "WildcardExcludesSiblingPrefix", pattern: "a.*", eventType: "ab.c", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchType(tt.pattern, tt.eventType), "Pattern match result should be correct for %s vs %s", tt.pattern, tt.eventType)
		})
	}
}

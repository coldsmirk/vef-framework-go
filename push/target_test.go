package push

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTargetConstructors(t *testing.T) {
	tests := []struct {
		name       string
		target     Target
		wantKind   TargetKind
		wantValues []string
	}{
		{"ToUsers", ToUsers("u1", "u2"), TargetUsers, []string{"u1", "u2"}},
		{"ToRoles", ToRoles("admin"), TargetRoles, []string{"admin"}},
		{"Broadcast", Broadcast(), TargetBroadcast, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantKind, tt.target.Kind, "Constructor should set the kind")
			assert.Equal(t, tt.wantValues, tt.target.Values, "Constructor should carry the values")
		})
	}
}

package exec

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
)

func TestLogRecorderShouldRecord(t *testing.T) {
	tests := []struct {
		name string
		mode config.IntegrationLogMode
		kind integration.FailureKind
		want bool
	}{
		{name: "OffDropsSuccesses", mode: config.IntegrationLogOff, kind: "", want: false},
		{name: "OffDropsFailures", mode: config.IntegrationLogOff, kind: integration.FailureScript, want: false},
		{name: "ErrorsDropsSuccesses", mode: config.IntegrationLogErrors, kind: "", want: false},
		{name: "ErrorsKeepsFailures", mode: config.IntegrationLogErrors, kind: integration.FailureTimeout, want: true},
		{name: "AllKeepsSuccesses", mode: config.IntegrationLogAll, kind: "", want: true},
		{name: "AllKeepsFailures", mode: config.IntegrationLogAll, kind: integration.FailureUpstream, want: true},
		{name: "DefaultModeDropsSuccesses", mode: "", kind: "", want: false},
		{name: "DefaultModeKeepsFailures", mode: "", kind: integration.FailureConfig, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := newLogRecorder(nil, &config.IntegrationConfig{Log: config.IntegrationLogConfig{Mode: tt.mode}})

			assert.Equal(t, tt.want, recorder.ShouldRecord(tt.kind), "Mode %q with kind %q should select %v", tt.mode, tt.kind, tt.want)
		})
	}
}

package logx

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/logx"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  logx.Level
	}{
		{"Debug", "debug", logx.LevelDebug},
		{"Info", "info", logx.LevelInfo},
		{"Warn", "warn", logx.LevelWarn},
		{"Error", "error", logx.LevelError},
		{"Panic", "panic", logx.LevelPanic},
		{"MixedCase", "Debug", logx.LevelDebug},
		{"Empty", "", logx.LevelInfo},
		{"Unknown", "verbose", logx.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseLevel(tt.input), "parseLevel should map %q to the matching level", tt.input)
		})
	}
}

func TestSetLevel(t *testing.T) {
	original := rootLevel.Level()
	t.Cleanup(func() { rootLevel.SetLevel(original) })

	logger := Named("set-level-test")

	SetLevel(logx.LevelError)
	assert.False(t, logger.Enabled(logx.LevelInfo), "Info should be disabled once the root level is raised to Error")
	assert.True(t, logger.Enabled(logx.LevelError), "Error should remain enabled at the Error root level")

	SetLevel(logx.LevelDebug)
	assert.True(t, logger.Enabled(logx.LevelDebug), "Debug should be enabled after lowering the root level to Debug")
}

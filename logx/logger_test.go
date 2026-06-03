package logx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		name  string
		level Level
		want  string
	}{
		{"Debug", LevelDebug, "debug"},
		{"Info", LevelInfo, "info"},
		{"Warn", LevelWarn, "warn"},
		{"Error", LevelError, "error"},
		{"Panic", LevelPanic, "panic"},
		{"Unknown", Level(0), "unknown"},
		{"OutOfRange", Level(127), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.level.String(), "Level.String should map %d to its canonical name", tt.level)
		})
	}
}

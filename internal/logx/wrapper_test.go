package logx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/coldsmirk/vef-framework-go/logx"
)

func TestToZapLevel(t *testing.T) {
	tests := []struct {
		name  string
		level logx.Level
		want  zapcore.Level
	}{
		{"Debug", logx.LevelDebug, zap.DebugLevel},
		{"Info", logx.LevelInfo, zap.InfoLevel},
		{"Warn", logx.LevelWarn, zap.WarnLevel},
		{"Error", logx.LevelError, zap.ErrorLevel},
		{"Panic", logx.LevelPanic, zap.PanicLevel},
		{"UnknownDefaultsToInfo", logx.Level(0), zap.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, toZapLevel(tt.level), "toZapLevel should map %s to the matching zap level", tt.level)
		})
	}
}

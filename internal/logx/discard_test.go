package logx

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/logx"
)

func TestDiscard(t *testing.T) {
	logger := Discard()

	t.Run("EnabledAlwaysFalse", func(t *testing.T) {
		assert.False(t, logger.Enabled(logx.LevelError), "discard logger should report every level disabled")
	})

	t.Run("DerivationsReturnDiscard", func(t *testing.T) {
		assert.NotPanics(t, func() {
			logger.Named("child").WithCallerSkip(1).Info("ignored")
		}, "discard logger derivations should silently drop messages")
	})

	t.Run("Panic", func(t *testing.T) {
		assert.PanicsWithValue(t, "boom", func() {
			logger.Panic("boom")
		}, "discard logger Panic should still panic with the message")
	})

	t.Run("Panicf", func(t *testing.T) {
		assert.PanicsWithValue(t, "boom 42", func() {
			logger.Panicf("boom %d", 42)
		}, "discard logger Panicf should still panic with the formatted message")
	})
}

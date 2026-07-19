package push

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessage(t *testing.T) {
	message := NewMessage("order.status_changed", map[string]any{"orderId": "o-1"})

	assert.Len(t, message.ID, 20, "NewMessage should generate a 20-char XID")
	assert.Equal(t, "order.status_changed", message.Type, "Type should pass through")
	assert.False(t, message.Time.IsZero(), "Time should be stamped")
	assert.Equal(t, map[string]any{"orderId": "o-1"}, message.Payload, "Payload should pass through")
}

func TestMessageEnvelopeJSON(t *testing.T) {
	t.Run("WithPayload", func(t *testing.T) {
		payload, err := json.Marshal(NewMessage("t", "v"))
		require.NoError(t, err, "Envelope should marshal")

		var envelope map[string]any
		require.NoError(t, json.Unmarshal(payload, &envelope), "Envelope should unmarshal as an object")

		assert.ElementsMatch(t, []string{"id", "type", "payload", "time"},
			slices.Collect(maps.Keys(envelope)), "Envelope should carry exactly the contract fields")
	})

	t.Run("NilPayloadOmitted", func(t *testing.T) {
		payload, err := json.Marshal(NewMessage("t", nil))
		require.NoError(t, err, "Envelope should marshal")

		assert.NotContains(t, string(payload), "payload", "A nil payload should be omitted from the wire")
	})
}

package event

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewBaseEvent tests new base event functionality.
func TestNewBaseEvent(t *testing.T) {
	t.Run("MinimalCreationWithJustType", func(t *testing.T) {
		event := NewBaseEvent("test.event")

		assert.Equal(t, "test.event", event.Type(), "Should equal expected value")
		assert.Empty(t, event.Source(), "Source should be empty")
		assert.NotEmpty(t, event.ID(), "ID should not be empty")
		assert.False(t, event.Time().IsZero(), "Time should not be zero")
		assert.Empty(t, event.Meta(), "Meta should be empty")
	})

	t.Run("CreationWithSourceOption", func(t *testing.T) {
		event := NewBaseEvent("user.created", WithSource("user-service"))

		assert.Equal(t, "user.created", event.Type(), "Should equal expected value")
		assert.Equal(t, "user-service", event.Source(), "Should equal expected value")
		assert.NotEmpty(t, event.ID(), "ID should not be empty")
		assert.False(t, event.Time().IsZero(), "Time should not be zero")
		assert.Empty(t, event.Meta(), "Meta should be empty")
	})

	t.Run("CreationWithSingleMetadataOption", func(t *testing.T) {
		event := NewBaseEvent("order.placed", WithMeta("version", "1.0"))

		assert.Equal(t, "order.placed", event.Type(), "Should equal expected value")
		assert.Empty(t, event.Source(), "Source should be empty")
		assert.NotEmpty(t, event.ID(), "ID should not be empty")
		assert.False(t, event.Time().IsZero(), "Time should not be zero")
		assert.Len(t, event.Meta(), 1, "Length should be 1")
		assert.Equal(t, "1.0", event.Meta()["version"], "Should equal expected value")
	})

	t.Run("CreationWithMultipleOptions", func(t *testing.T) {
		event := NewBaseEvent("payment.processed",
			WithSource("payment-service"),
			WithMeta("amount", "100.00"),
			WithMeta("currency", "USD"),
			WithMeta("gateway", "stripe"),
		)

		assert.Equal(t, "payment.processed", event.Type(), "Should equal expected value")
		assert.Equal(t, "payment-service", event.Source(), "Should equal expected value")
		assert.NotEmpty(t, event.ID(), "ID should not be empty")
		assert.False(t, event.Time().IsZero(), "Time should not be zero")

		meta := event.Meta()
		assert.Len(t, meta, 3, "Length should be 3")
		assert.Equal(t, "100.00", meta["amount"], "Should equal expected value")
		assert.Equal(t, "USD", meta["currency"], "Should equal expected value")
		assert.Equal(t, "stripe", meta["gateway"], "Should equal expected value")
	})

	t.Run("EachEventHasUniqueIDAndTime", func(t *testing.T) {
		event1 := NewBaseEvent("test.event")

		time.Sleep(1 * time.Millisecond)

		event2 := NewBaseEvent("test.event")

		assert.NotEqual(t, event1.ID(), event2.ID(), "Should not equal")
		assert.True(t, event2.Time().After(event1.Time()) || event2.Time().Equal(event1.Time()), "Second event time should be after or equal to first")
	})
}

// TestBaseEventMetadata tests Base Event metadata scenarios.
func TestBaseEventMetadata(t *testing.T) {
	t.Run("MetaReturnsAllMetadata", func(t *testing.T) {
		event := NewBaseEvent("test.event",
			WithMeta("key1", "value1"),
			WithMeta("key2", "value2"),
			WithMeta("key3", "value3"),
		)

		meta := event.Meta()
		assert.Len(t, meta, 3, "Length should be 3")
		assert.Equal(t, "value1", meta["key1"], "Should equal expected value")
		assert.Equal(t, "value2", meta["key2"], "Should equal expected value")
		assert.Equal(t, "value3", meta["key3"], "Should equal expected value")
	})

	t.Run("MetaReturnsCopyToPreventExternalModification", func(t *testing.T) {
		event := NewBaseEvent("test.event", WithMeta("key", "value"))

		meta := event.Meta()
		meta["key"] = "modified"
		meta["new"] = "added"

		freshMeta := event.Meta()
		assert.Equal(t, "value", freshMeta["key"], "Should equal expected value")
		assert.NotContains(t, freshMeta, "new", "External modification should not affect original meta")
	})
}

// TestBaseEventJSONSerialization tests Base Event JSON serialization scenarios.
func TestBaseEventJSONSerialization(t *testing.T) {
	t.Run("MarshalMinimalEvent", func(t *testing.T) {
		event := NewBaseEvent("test.event")

		jsonData, err := json.Marshal(event)
		require.NoError(t, err, "Should not return error")

		var jsonMap map[string]any

		err = json.Unmarshal(jsonData, &jsonMap)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, "test.event", jsonMap["type"], "Should equal expected value")
		assert.Equal(t, "", jsonMap["source"], "Should equal expected value")
		assert.NotEmpty(t, jsonMap["id"], "Should not be empty")
		assert.NotEmpty(t, jsonMap["time"], "Should not be empty")

		_, hasMetadata := jsonMap["metadata"]
		assert.False(t, hasMetadata, "Should be false")
	})

	t.Run("MarshalEventWithAllFields", func(t *testing.T) {
		event := NewBaseEvent("user.registered",
			WithSource("user-service"),
			WithMeta("version", "1.0"),
			WithMeta("region", "us-east-1"),
		)

		jsonData, err := json.Marshal(event)
		require.NoError(t, err, "Should not return error")

		var jsonMap map[string]any

		err = json.Unmarshal(jsonData, &jsonMap)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, "user.registered", jsonMap["type"], "Should equal expected value")
		assert.Equal(t, "user-service", jsonMap["source"], "Should equal expected value")
		assert.NotEmpty(t, jsonMap["id"], "Should not be empty")
		assert.NotEmpty(t, jsonMap["time"], "Should not be empty")

		metadata, ok := jsonMap["metadata"].(map[string]any)
		require.True(t, ok, "Should be ok")
		assert.Equal(t, "1.0", metadata["version"], "Should equal expected value")
		assert.Equal(t, "us-east-1", metadata["region"], "Should equal expected value")
	})

	t.Run("UnmarshalMinimalEvent", func(t *testing.T) {
		jsonData := `{
			"type": "test.unmarshal",
			"id": "test-id-123",
			"source": "test-source",
			"time": "2023-01-01T12:00:00Z"
		}`

		var event BaseEvent

		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, "test.unmarshal", event.Type(), "Should equal expected value")
		assert.Equal(t, "test-id-123", event.ID(), "Should equal expected value")
		assert.Equal(t, "test-source", event.Source(), "Should equal expected value")

		expectedTime, _ := time.Parse(time.RFC3339, "2023-01-01T12:00:00Z")
		assert.Equal(t, expectedTime, event.Time(), "Should equal expected value")
		assert.Empty(t, event.Meta(), "Meta should be empty")
	})

	t.Run("UnmarshalEventWithMetadata", func(t *testing.T) {
		jsonData := `{
			"type": "order.created",
			"id": "order-456",
			"source": "order-service",
			"time": "2023-06-15T10:30:00Z",
			"metadata": {
				"customer_id": "123",
				"total": "99.99"
			}
		}`

		var event BaseEvent

		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, "order.created", event.Type(), "Should equal expected value")
		assert.Equal(t, "order-456", event.ID(), "Should equal expected value")
		assert.Equal(t, "order-service", event.Source(), "Should equal expected value")

		meta := event.Meta()
		assert.Len(t, meta, 2, "Length should be 2")
		assert.Equal(t, "123", meta["customer_id"], "Should equal expected value")
		assert.Equal(t, "99.99", meta["total"], "Should equal expected value")
	})

	t.Run("RoundtripSerializationPreservesData", func(t *testing.T) {
		original := NewBaseEvent("roundtrip.test",
			WithSource("test-service"),
			WithMeta("key1", "value1"),
			WithMeta("key2", "value2"),
		)

		jsonData, err := json.Marshal(original)
		require.NoError(t, err, "Should not return error")

		var restored BaseEvent

		err = json.Unmarshal(jsonData, &restored)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, original.Type(), restored.Type(), "Should equal expected value")
		assert.Equal(t, original.ID(), restored.ID(), "Should equal expected value")
		assert.Equal(t, original.Source(), restored.Source(), "Should equal expected value")
		assert.Equal(t, original.Time().Unix(), restored.Time().Unix(), "Should equal expected value")
		assert.Equal(t, original.Meta(), restored.Meta(), "Should equal expected value")
	})

	t.Run("UnmarshalHandlesMissingMetadataGracefully", func(t *testing.T) {
		jsonData := `{
			"type": "simple.event",
			"id": "simple-123",
			"source": "simple-service",
			"time": "2023-01-01T00:00:00Z"
		}`

		var event BaseEvent

		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err, "Should not return error")

		assert.NotNil(t, event.Meta(), "Meta should not be nil")
		assert.Empty(t, event.Meta(), "Meta should be empty")
	})

	t.Run("UnmarshalInvalidJSONReturnsError", func(t *testing.T) {
		invalidJSON := `{invalid json`

		var event BaseEvent

		err := json.Unmarshal([]byte(invalidJSON), &event)
		assert.Error(t, err, "Should return error")
	})
}

// TestBaseEventImmutability tests Base Event immutability scenarios.
func TestBaseEventImmutability(t *testing.T) {
	t.Run("CoreFieldsAreImmutableAfterCreation", func(t *testing.T) {
		event := NewBaseEvent("test.event", WithSource("test-source"))

		originalType := event.Type()
		originalID := event.ID()
		originalSource := event.Source()
		originalTime := event.Time()

		time.Sleep(1 * time.Millisecond)

		assert.Equal(t, originalType, event.Type(), "Should equal expected value")
		assert.Equal(t, originalID, event.ID(), "Should equal expected value")
		assert.Equal(t, originalSource, event.Source(), "Should equal expected value")
		assert.Equal(t, originalTime, event.Time(), "Should equal expected value")
	})

	t.Run("MetadataIsImmutableAfterCreation", func(t *testing.T) {
		event := NewBaseEvent("test.event", WithMeta("initial", "value"))

		meta := event.Meta()
		assert.Len(t, meta, 1, "Length should be 1")
		assert.Equal(t, "value", meta["initial"], "Should equal expected value")
	})
}

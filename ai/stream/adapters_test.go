package stream

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChannelSource tests channel source functionality.
func TestChannelSource(t *testing.T) {
	t.Run("ReceivesMessagesUntilChannelClosed", func(t *testing.T) {
		ch := make(chan Message, 3)
		ch <- Message{Role: RoleUser, Content: "Hello"}

		ch <- Message{Role: RoleAssistant, Content: "Hi"}

		ch <- Message{Role: RoleAssistant, Content: "there"}

		close(ch)

		source := NewChannelSource(ch)
		defer source.Close()

		msg1, err := source.Recv()
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, RoleUser, msg1.Role, "Should equal expected value")
		assert.Equal(t, "Hello", msg1.Content, "Should equal expected value")

		msg2, err := source.Recv()
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, RoleAssistant, msg2.Role, "Should equal expected value")
		assert.Equal(t, "Hi", msg2.Content, "Should equal expected value")

		msg3, err := source.Recv()
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, "there", msg3.Content, "Should equal expected value")

		_, err = source.Recv()
		assert.ErrorIs(t, err, io.EOF, "Error should be io.EOF")
	})

	t.Run("ReturnsEofAfterClose", func(t *testing.T) {
		ch := make(chan Message, 1)
		ch <- Message{Role: RoleUser, Content: "test"}

		source := NewChannelSource(ch)
		err := source.Close()
		require.NoError(t, err, "Should not return error")

		_, err = source.Recv()
		assert.ErrorIs(t, err, io.EOF, "Error should be io.EOF")
	})

	t.Run("HandlesEmptyChannel", func(t *testing.T) {
		ch := make(chan Message)
		close(ch)

		source := NewChannelSource(ch)
		defer source.Close()

		_, err := source.Recv()
		assert.ErrorIs(t, err, io.EOF, "Error should be io.EOF")
	})
}

// TestCallbackSource tests callback source functionality.
func TestCallbackSource(t *testing.T) {
	t.Run("ReceivesTextMessages", func(t *testing.T) {
		source := NewCallbackSource(func(w CallbackWriter) error {
			w.WriteText("Hello")
			w.WriteText(" World")

			return nil
		})
		defer source.Close()

		msg1, err := source.Recv()
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, RoleAssistant, msg1.Role, "Should equal expected value")
		assert.Equal(t, "Hello", msg1.Content, "Should equal expected value")

		msg2, err := source.Recv()
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, " World", msg2.Content, "Should equal expected value")

		_, err = source.Recv()
		assert.ErrorIs(t, err, io.EOF, "Error should be io.EOF")
	})

	t.Run("ReceivesToolCalls", func(t *testing.T) {
		source := NewCallbackSource(func(w CallbackWriter) error {
			w.WriteToolCall("call_1", "get_weather", `{"city":"Beijing"}`)

			return nil
		})
		defer source.Close()

		msg, err := source.Recv()
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, RoleAssistant, msg.Role, "Should equal expected value")
		require.Len(t, msg.ToolCalls, 1, "Length should be 1")
		assert.Equal(t, "call_1", msg.ToolCalls[0].ID, "Should equal expected value")
		assert.Equal(t, "get_weather", msg.ToolCalls[0].Name, "Should equal expected value")
		assert.Equal(t, `{"city":"Beijing"}`, msg.ToolCalls[0].Arguments, "Should equal expected value")
	})

	t.Run("ReceivesToolResults", func(t *testing.T) {
		source := NewCallbackSource(func(w CallbackWriter) error {
			w.WriteToolResult("call_1", `{"temp":25}`)

			return nil
		})
		defer source.Close()

		msg, err := source.Recv()
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, RoleTool, msg.Role, "Should equal expected value")
		assert.Equal(t, "call_1", msg.ToolCallID, "Should equal expected value")
		assert.Equal(t, `{"temp":25}`, msg.Content, "Should equal expected value")
	})

	t.Run("ReceivesReasoning", func(t *testing.T) {
		source := NewCallbackSource(func(w CallbackWriter) error {
			w.WriteReasoning("Let me think...")

			return nil
		})
		defer source.Close()

		msg, err := source.Recv()
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, RoleAssistant, msg.Role, "Should equal expected value")
		assert.Equal(t, "Let me think...", msg.Reasoning, "Should equal expected value")
	})

	t.Run("ReceivesCustomData", func(t *testing.T) {
		source := NewCallbackSource(func(w CallbackWriter) error {
			w.WriteData("status", map[string]any{"progress": 50})

			return nil
		})
		defer source.Close()

		msg, err := source.Recv()
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, RoleAssistant, msg.Role, "Should equal expected value")
		assert.Equal(t, map[string]any{"progress": 50}, msg.Data["status"], "Should equal expected value")
	})

	t.Run("ReceivesFullMessage", func(t *testing.T) {
		customMsg := Message{
			Role:    RoleSystem,
			Content: "System prompt",
		}

		source := NewCallbackSource(func(w CallbackWriter) error {
			w.WriteMessage(customMsg)

			return nil
		})
		defer source.Close()

		msg, err := source.Recv()
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, customMsg, msg, "Should equal expected value")
	})

	t.Run("PropagatesError", func(t *testing.T) {
		expectedErr := io.ErrUnexpectedEOF

		source := NewCallbackSource(func(w CallbackWriter) error {
			w.WriteText("partial")

			return expectedErr
		})
		defer source.Close()

		msg, err := source.Recv()
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, "partial", msg.Content, "Should equal expected value")

		_, err = source.Recv()
		assert.ErrorIs(t, err, expectedErr, "Error should be expectedErr")
	})
}

// TestFromChannel tests from channel functionality.
func TestFromChannel(t *testing.T) {
	ch := make(chan Message, 1)
	ch <- Message{Role: RoleUser, Content: "test"}

	close(ch)

	builder := FromChannel(ch)
	assert.NotNil(t, builder, "Should not be nil")
	assert.NotNil(t, builder.source, "Should not be nil")
}

// TestFromCallback tests from callback functionality.
func TestFromCallback(t *testing.T) {
	builder := FromCallback(func(w CallbackWriter) error {
		w.WriteText("test")

		return nil
	})
	assert.NotNil(t, builder, "Should not be nil")
	assert.NotNil(t, builder.source, "Should not be nil")
}

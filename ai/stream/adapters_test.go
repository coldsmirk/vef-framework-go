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
		require.NoError(t, err, "First channel receive should return a message")
		assert.Equal(t, RoleUser, msg1.Role, "First channel message should keep the user role")
		assert.Equal(t, "Hello", msg1.Content, "First channel message should keep its content")

		msg2, err := source.Recv()
		require.NoError(t, err, "Second channel receive should return a message")
		assert.Equal(t, RoleAssistant, msg2.Role, "Second channel message should keep the assistant role")
		assert.Equal(t, "Hi", msg2.Content, "Second channel message should keep its content")

		msg3, err := source.Recv()
		require.NoError(t, err, "Third channel receive should return a message")
		assert.Equal(t, "there", msg3.Content, "Third channel message should keep its content")

		_, err = source.Recv()
		assert.ErrorIs(t, err, io.EOF, "Closed channel source should return io.EOF")
	})

	t.Run("ReturnsEofAfterClose", func(t *testing.T) {
		ch := make(chan Message, 1)
		ch <- Message{Role: RoleUser, Content: "test"}

		source := NewChannelSource(ch)
		err := source.Close()
		require.NoError(t, err, "Closing a channel source should succeed")

		_, err = source.Recv()
		assert.ErrorIs(t, err, io.EOF, "Closed source should return io.EOF")
	})

	t.Run("HandlesEmptyChannel", func(t *testing.T) {
		ch := make(chan Message)
		close(ch)

		source := NewChannelSource(ch)
		defer source.Close()

		_, err := source.Recv()
		assert.ErrorIs(t, err, io.EOF, "Empty channel source should return io.EOF")
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
		require.NoError(t, err, "First callback text receive should return a message")
		assert.Equal(t, RoleAssistant, msg1.Role, "Callback text message should use the assistant role")
		assert.Equal(t, "Hello", msg1.Content, "First callback text message should keep its content")

		msg2, err := source.Recv()
		require.NoError(t, err, "Second callback text receive should return a message")
		assert.Equal(t, " World", msg2.Content, "Second callback text message should keep its content")

		_, err = source.Recv()
		assert.ErrorIs(t, err, io.EOF, "Drained callback source should return io.EOF")
	})

	t.Run("ReceivesToolCalls", func(t *testing.T) {
		source := NewCallbackSource(func(w CallbackWriter) error {
			w.WriteToolCall("call_1", "get_weather", `{"city":"Beijing"}`)

			return nil
		})
		defer source.Close()

		msg, err := source.Recv()
		require.NoError(t, err, "Callback tool call receive should return a message")
		assert.Equal(t, RoleAssistant, msg.Role, "Tool call message should use the assistant role")
		require.Len(t, msg.ToolCalls, 1, "Tool call message should contain one tool call")
		assert.Equal(t, "call_1", msg.ToolCalls[0].ID, "Tool call should keep its ID")
		assert.Equal(t, "get_weather", msg.ToolCalls[0].Name, "Tool call should keep its name")
		assert.Equal(t, `{"city":"Beijing"}`, msg.ToolCalls[0].Arguments, "Tool call should keep its arguments")
	})

	t.Run("ReceivesToolResults", func(t *testing.T) {
		source := NewCallbackSource(func(w CallbackWriter) error {
			w.WriteToolResult("call_1", `{"temp":25}`)

			return nil
		})
		defer source.Close()

		msg, err := source.Recv()
		require.NoError(t, err, "Callback tool result receive should return a message")
		assert.Equal(t, RoleTool, msg.Role, "Tool result message should use the tool role")
		assert.Equal(t, "call_1", msg.ToolCallID, "Tool result message should keep the tool call ID")
		assert.Equal(t, `{"temp":25}`, msg.Content, "Tool result message should keep its content")
	})

	t.Run("ReceivesReasoning", func(t *testing.T) {
		source := NewCallbackSource(func(w CallbackWriter) error {
			w.WriteReasoning("Let me think...")

			return nil
		})
		defer source.Close()

		msg, err := source.Recv()
		require.NoError(t, err, "Callback reasoning receive should return a message")
		assert.Equal(t, RoleAssistant, msg.Role, "Reasoning message should use the assistant role")
		assert.Equal(t, "Let me think...", msg.Reasoning, "Reasoning message should keep its reasoning text")
	})

	t.Run("ReceivesCustomData", func(t *testing.T) {
		source := NewCallbackSource(func(w CallbackWriter) error {
			w.WriteData("status", map[string]any{"progress": 50})

			return nil
		})
		defer source.Close()

		msg, err := source.Recv()
		require.NoError(t, err, "Callback custom data receive should return a message")
		assert.Equal(t, RoleAssistant, msg.Role, "Custom data message should use the assistant role")
		assert.Equal(t, map[string]any{"progress": 50}, msg.Data["status"], "Custom data message should keep the status payload")
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
		require.NoError(t, err, "Callback full message receive should return a message")
		assert.Equal(t, customMsg, msg, "Callback full message should be forwarded unchanged")
	})

	t.Run("PropagatesError", func(t *testing.T) {
		expectedErr := io.ErrUnexpectedEOF

		source := NewCallbackSource(func(w CallbackWriter) error {
			w.WriteText("partial")

			return expectedErr
		})
		defer source.Close()

		msg, err := source.Recv()
		require.NoError(t, err, "Callback should deliver queued message before returning its error")
		assert.Equal(t, "partial", msg.Content, "Callback error path should preserve the queued message content")

		_, err = source.Recv()
		assert.ErrorIs(t, err, expectedErr, "Callback source should return the callback error after queued messages")
	})
}

// TestFromChannel tests from channel functionality.
func TestFromChannel(t *testing.T) {
	ch := make(chan Message, 1)
	ch <- Message{Role: RoleUser, Content: "test"}

	close(ch)

	builder := FromChannel(ch)
	assert.NotNil(t, builder, "FromChannel should return a builder")
	assert.NotNil(t, builder.source, "FromChannel should configure a source")
}

// TestFromCallback tests from callback functionality.
func TestFromCallback(t *testing.T) {
	builder := FromCallback(func(w CallbackWriter) error {
		w.WriteText("test")

		return nil
	})
	assert.NotNil(t, builder, "FromCallback should return a builder")
	assert.NotNil(t, builder.source, "FromCallback should configure a source")
}

package stream

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseSseChunks extracts json chunks from SSE output.
func parseSseChunks(t *testing.T, output string) []map[string]any {
	t.Helper()

	var chunks []map[string]any

	for line := range strings.SplitSeq(output, "\n") {
		if after, ok := strings.CutPrefix(line, "data: "); ok {
			data := after
			if data == "[DONE]" {
				continue
			}

			var chunk map[string]any
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				chunks = append(chunks, chunk)
			}
		}
	}

	return chunks
}

// TestBuilderConfiguration tests builder configuration functionality.
func TestBuilderConfiguration(t *testing.T) {
	t.Run("NewReturnsBuilderWithDefaults", func(t *testing.T) {
		b := New()

		assert.NotNil(t, b, "New should return a builder instance")
		assert.True(t, b.opts.SendReasoning, "Default options should send reasoning chunks")
		assert.True(t, b.opts.SendSources, "Default options should send source chunks")
		assert.True(t, b.opts.SendStart, "Default options should send start chunks")
		assert.True(t, b.opts.SendFinish, "Default options should send finish chunks")
	})

	t.Run("WithSourceSetsSource", func(t *testing.T) {
		ch := make(chan Message)
		close(ch)
		source := NewChannelSource(ch)

		b := New().WithSource(source)

		assert.Equal(t, source, b.source, "WithSource should store the configured source")
	})

	t.Run("WithMessageIDSetsMessageID", func(t *testing.T) {
		b := New().WithMessageID("custom_id")

		assert.Equal(t, "custom_id", b.messageID, "WithMessageID should store the configured message ID")
	})

	t.Run("WithReasoningSetsOption", func(t *testing.T) {
		b := New().WithReasoning(false)

		assert.False(t, b.opts.SendReasoning, "WithReasoning should update the reasoning option")
	})

	t.Run("WithSourcesSetsOption", func(t *testing.T) {
		b := New().WithSources(false)

		assert.False(t, b.opts.SendSources, "WithSources should update the sources option")
	})

	t.Run("WithStartSetsOption", func(t *testing.T) {
		b := New().WithStart(false)

		assert.False(t, b.opts.SendStart, "WithStart should update the start option")
	})

	t.Run("WithFinishSetsOption", func(t *testing.T) {
		b := New().WithFinish(false)

		assert.False(t, b.opts.SendFinish, "WithFinish should update the finish option")
	})

	t.Run("OnErrorSetsHandler", func(t *testing.T) {
		handler := func(err error) string { return "custom: " + err.Error() }
		b := New().OnError(handler)

		assert.NotNil(t, b.opts.OnError, "OnError should store an error formatter")
		assert.Equal(t, "custom: test", b.opts.OnError(errors.New("test")), "OnError should use the configured formatter")
	})

	t.Run("OnFinishSetsHandler", func(t *testing.T) {
		var captured string

		handler := func(content string) { captured = content }
		b := New().OnFinish(handler)

		assert.NotNil(t, b.opts.OnFinish, "OnFinish should store a finish callback")
		b.opts.OnFinish("test content")
		assert.Equal(t, "test content", captured, "OnFinish should receive the final content")
	})

	t.Run("WithIDGeneratorSetsGenerator", func(t *testing.T) {
		gen := func(prefix string) string { return prefix + "_fixed" }
		b := New().WithIDGenerator(gen)

		assert.NotNil(t, b.opts.GenerateID, "WithIDGenerator should store an ID generator")
		assert.Equal(t, "msg_fixed", b.opts.GenerateID("msg"), "WithIDGenerator should use the configured generator")
	})

	t.Run("WithHeaderAddsHeader", func(t *testing.T) {
		b := New().
			WithHeader("X-Custom", "value1").
			WithHeader("X-Another", "value2")

		assert.Equal(t, "value1", b.headers["X-Custom"], "WithHeader should store the first header value")
		assert.Equal(t, "value2", b.headers["X-Another"], "WithHeader should store additional header values")
	})

	t.Run("FluentChaining", func(t *testing.T) {
		ch := make(chan Message)
		close(ch)

		b := New().
			WithSource(NewChannelSource(ch)).
			WithMessageID("msg_1").
			WithReasoning(true).
			WithSources(true).
			WithStart(true).
			WithFinish(true).
			WithHeader("X-Test", "value")

		assert.NotNil(t, b.source, "Fluent chaining should preserve the configured source")
		assert.Equal(t, "msg_1", b.messageID, "Fluent chaining should preserve the configured message ID")
		assert.True(t, b.opts.SendReasoning, "Fluent chaining should preserve the reasoning option")
		assert.Equal(t, "value", b.headers["X-Test"], "Fluent chaining should preserve the configured header")
	})
}

// TestBuilderStreamToWriter tests builder stream to writer functionality.
func TestBuilderStreamToWriter(t *testing.T) {
	t.Run("StreamsTextContent", func(t *testing.T) {
		ch := make(chan Message, 2)
		ch <- Message{Role: RoleAssistant, Content: "Hello"}

		ch <- Message{Role: RoleAssistant, Content: " World"}

		close(ch)

		var buf bytes.Buffer

		w := bufio.NewWriter(&buf)

		New().
			WithSource(NewChannelSource(ch)).
			WithMessageID("msg_test").
			WithIDGenerator(func(prefix string) string { return prefix + "_1" }).
			StreamToWriter(w)

		output := buf.String()
		chunks := parseSseChunks(t, output)

		require.GreaterOrEqual(t, len(chunks), 4, "Text stream should include start, text, and finish chunks")

		// Verify start chunk
		assert.Equal(t, "start", chunks[0]["type"], "First chunk should start the message stream")
		assert.Equal(t, "msg_test", chunks[0]["messageID"], "Start chunk should use the configured message ID")

		// Verify text chunks exist
		hasTextStart := false

		hasTextDelta := false
		for _, c := range chunks {
			if c["type"] == "text-start" {
				hasTextStart = true
			}

			if c["type"] == "text-delta" {
				hasTextDelta = true
			}
		}

		assert.True(t, hasTextStart, "Text stream should include a text-start chunk")
		assert.True(t, hasTextDelta, "Text stream should include a text-delta chunk")

		// Verify done marker
		assert.Contains(t, output, "data: [DONE]", "Text stream should include the DONE marker")
	})

	t.Run("StreamsReasoningContent", func(t *testing.T) {
		ch := make(chan Message, 1)
		ch <- Message{Role: RoleAssistant, Reasoning: "Thinking..."}

		close(ch)

		var buf bytes.Buffer

		w := bufio.NewWriter(&buf)

		New().
			WithSource(NewChannelSource(ch)).
			WithReasoning(true).
			WithIDGenerator(func(prefix string) string { return prefix + "_1" }).
			StreamToWriter(w)

		output := buf.String()
		chunks := parseSseChunks(t, output)

		hasReasoningStart := false

		hasReasoningDelta := false
		for _, c := range chunks {
			if c["type"] == "reasoning-start" {
				hasReasoningStart = true
			}

			if c["type"] == "reasoning-delta" {
				hasReasoningDelta = true

				assert.Equal(t, "Thinking...", c["delta"], "Reasoning delta should contain the message reasoning")
			}
		}

		assert.True(t, hasReasoningStart, "Reasoning stream should include a reasoning-start chunk")
		assert.True(t, hasReasoningDelta, "Reasoning stream should include a reasoning-delta chunk")
	})

	t.Run("SkipsReasoningWhenDisabled", func(t *testing.T) {
		ch := make(chan Message, 1)
		ch <- Message{Role: RoleAssistant, Reasoning: "Thinking..."}

		close(ch)

		var buf bytes.Buffer

		w := bufio.NewWriter(&buf)

		New().
			WithSource(NewChannelSource(ch)).
			WithReasoning(false).
			StreamToWriter(w)

		output := buf.String()
		assert.NotContains(t, output, "reasoning-start", "Disabled reasoning should omit reasoning-start chunks")
		assert.NotContains(t, output, "reasoning-delta", "Disabled reasoning should omit reasoning-delta chunks")
	})

	t.Run("StreamsToolCalls", func(t *testing.T) {
		ch := make(chan Message, 1)
		ch <- Message{
			Role: RoleAssistant,
			ToolCalls: []ToolCall{{
				ID:        "call_1",
				Name:      "get_weather",
				Arguments: `{"city":"Beijing"}`,
			}},
		}

		close(ch)

		var buf bytes.Buffer

		w := bufio.NewWriter(&buf)

		New().
			WithSource(NewChannelSource(ch)).
			StreamToWriter(w)

		output := buf.String()
		chunks := parseSseChunks(t, output)

		hasToolInputStart := false

		hasToolInputAvailable := false
		for _, c := range chunks {
			if c["type"] == "tool-input-start" {
				hasToolInputStart = true

				assert.Equal(t, "call_1", c["toolCallID"], "Tool input start should include the tool call ID")
				assert.Equal(t, "get_weather", c["toolName"], "Tool input start should include the tool name")
			}

			if c["type"] == "tool-input-available" {
				hasToolInputAvailable = true
			}
		}

		assert.True(t, hasToolInputStart, "Tool call stream should include a tool-input-start chunk")
		assert.True(t, hasToolInputAvailable, "Tool call stream should include a tool-input-available chunk")
	})

	t.Run("StreamsToolResults", func(t *testing.T) {
		ch := make(chan Message, 1)
		ch <- Message{
			Role:       RoleTool,
			ToolCallID: "call_1",
			Content:    `{"temp":25}`,
		}

		close(ch)

		var buf bytes.Buffer

		w := bufio.NewWriter(&buf)

		New().
			WithSource(NewChannelSource(ch)).
			StreamToWriter(w)

		output := buf.String()
		chunks := parseSseChunks(t, output)

		hasToolOutput := false
		for _, c := range chunks {
			if c["type"] == "tool-output-available" {
				hasToolOutput = true

				assert.Equal(t, "call_1", c["toolCallID"], "Tool output chunk should include the tool call ID")
			}
		}

		assert.True(t, hasToolOutput, "Tool result stream should include a tool-output-available chunk")
	})

	t.Run("StreamsCustomData", func(t *testing.T) {
		ch := make(chan Message, 1)
		ch <- Message{
			Role: RoleAssistant,
			Data: map[string]any{"status": "processing"},
		}

		close(ch)

		var buf bytes.Buffer

		w := bufio.NewWriter(&buf)

		New().
			WithSource(NewChannelSource(ch)).
			StreamToWriter(w)

		output := buf.String()
		assert.Contains(t, output, "data-status", "Custom data stream should include the data chunk type")
	})

	t.Run("HandlesErrorFromSource", func(t *testing.T) {
		expectedErr := errors.New("source error")
		source := NewCallbackSource(func(CallbackWriter) error {
			return expectedErr
		})

		var buf bytes.Buffer

		w := bufio.NewWriter(&buf)

		New().
			WithSource(source).
			StreamToWriter(w)

		output := buf.String()
		chunks := parseSseChunks(t, output)

		hasError := false
		for _, c := range chunks {
			if c["type"] == "error" {
				hasError = true

				assert.Equal(t, "source error", c["errorText"], "Error chunk should contain the source error text")
			}
		}

		assert.True(t, hasError, "Source error should be emitted as an error chunk")
	})

	t.Run("CallsOnErrorHandler", func(t *testing.T) {
		expectedErr := errors.New("test error")
		source := NewCallbackSource(func(CallbackWriter) error {
			return expectedErr
		})

		var buf bytes.Buffer

		w := bufio.NewWriter(&buf)

		New().
			WithSource(source).
			OnError(func(err error) string {
				return "Custom: " + err.Error()
			}).
			StreamToWriter(w)

		output := buf.String()
		assert.Contains(t, output, "Custom: test error", "Error stream should use the configured error formatter")
	})

	t.Run("CallsOnFinishHandler", func(t *testing.T) {
		ch := make(chan Message, 2)
		ch <- Message{Role: RoleAssistant, Content: "Hello"}

		ch <- Message{Role: RoleAssistant, Content: " World"}

		close(ch)

		var (
			finishedContent string
			buf             bytes.Buffer
		)

		w := bufio.NewWriter(&buf)

		New().
			WithSource(NewChannelSource(ch)).
			OnFinish(func(content string) {
				finishedContent = content
			}).
			StreamToWriter(w)

		assert.Equal(t, "Hello World", finishedContent, "OnFinish should receive concatenated assistant content")
	})

	t.Run("SkipsStartFinishWhenDisabled", func(t *testing.T) {
		ch := make(chan Message, 1)
		ch <- Message{Role: RoleAssistant, Content: "test"}

		close(ch)

		var buf bytes.Buffer

		w := bufio.NewWriter(&buf)

		New().WithSource(NewChannelSource(ch)).
			WithStart(false).
			WithFinish(false).
			StreamToWriter(w)

		output := buf.String()
		chunks := parseSseChunks(t, output)

		for _, c := range chunks {
			assert.NotEqual(t, "start", c["type"], "Disabled start option should omit start chunks")
			assert.NotEqual(t, "start-step", c["type"], "Disabled start option should omit start-step chunks")
			assert.NotEqual(t, "finish", c["type"], "Disabled finish option should omit finish chunks")
			assert.NotEqual(t, "finish-step", c["type"], "Disabled finish option should omit finish-step chunks")
		}
	})
}

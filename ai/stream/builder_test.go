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

// TestBuilderConfiguration tests builder configuration functionality.
func TestBuilderConfiguration(t *testing.T) {
	t.Run("NewReturnsBuilderWithDefaults", func(t *testing.T) {
		b := New()

		assert.NotNil(t, b, "Should not be nil")
		assert.True(t, b.opts.SendReasoning, "Should be true")
		assert.True(t, b.opts.SendSources, "Should be true")
		assert.True(t, b.opts.SendStart, "Should be true")
		assert.True(t, b.opts.SendFinish, "Should be true")
	})

	t.Run("WithSourceSetsSource", func(t *testing.T) {
		ch := make(chan Message)
		close(ch)
		source := NewChannelSource(ch)

		b := New().WithSource(source)

		assert.Equal(t, source, b.source, "Should equal expected value")
	})

	t.Run("WithMessageIDSetsMessageID", func(t *testing.T) {
		b := New().WithMessageID("custom_id")

		assert.Equal(t, "custom_id", b.messageID, "Should equal expected value")
	})

	t.Run("WithReasoningSetsOption", func(t *testing.T) {
		b := New().WithReasoning(false)

		assert.False(t, b.opts.SendReasoning, "Should be false")
	})

	t.Run("WithSourcesSetsOption", func(t *testing.T) {
		b := New().WithSources(false)

		assert.False(t, b.opts.SendSources, "Should be false")
	})

	t.Run("WithStartSetsOption", func(t *testing.T) {
		b := New().WithStart(false)

		assert.False(t, b.opts.SendStart, "Should be false")
	})

	t.Run("WithFinishSetsOption", func(t *testing.T) {
		b := New().WithFinish(false)

		assert.False(t, b.opts.SendFinish, "Should be false")
	})

	t.Run("OnErrorSetsHandler", func(t *testing.T) {
		handler := func(err error) string { return "custom: " + err.Error() }
		b := New().OnError(handler)

		assert.NotNil(t, b.opts.OnError, "Should not be nil")
		assert.Equal(t, "custom: test", b.opts.OnError(errors.New("test")), "Should equal expected value")
	})

	t.Run("OnFinishSetsHandler", func(t *testing.T) {
		var captured string

		handler := func(content string) { captured = content }
		b := New().OnFinish(handler)

		assert.NotNil(t, b.opts.OnFinish, "Should not be nil")
		b.opts.OnFinish("test content")
		assert.Equal(t, "test content", captured, "Should equal expected value")
	})

	t.Run("WithIDGeneratorSetsGenerator", func(t *testing.T) {
		gen := func(prefix string) string { return prefix + "_fixed" }
		b := New().WithIDGenerator(gen)

		assert.NotNil(t, b.opts.GenerateID, "Should not be nil")
		assert.Equal(t, "msg_fixed", b.opts.GenerateID("msg"), "Should equal expected value")
	})

	t.Run("WithHeaderAddsHeader", func(t *testing.T) {
		b := New().
			WithHeader("X-Custom", "value1").
			WithHeader("X-Another", "value2")

		assert.Equal(t, "value1", b.headers["X-Custom"], "Should equal expected value")
		assert.Equal(t, "value2", b.headers["X-Another"], "Should equal expected value")
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

		assert.NotNil(t, b.source, "Should not be nil")
		assert.Equal(t, "msg_1", b.messageID, "Should equal expected value")
		assert.True(t, b.opts.SendReasoning, "Should be true")
		assert.Equal(t, "value", b.headers["X-Test"], "Should equal expected value")
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

		require.GreaterOrEqual(t, len(chunks), 4, "Should be greater or equal")

		// Verify start chunk
		assert.Equal(t, "start", chunks[0]["type"], "Should equal expected value")
		assert.Equal(t, "msg_test", chunks[0]["messageID"], "Should equal expected value")

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

		assert.True(t, hasTextStart, "Should be true")
		assert.True(t, hasTextDelta, "Should be true")

		// Verify done marker
		assert.Contains(t, output, "data: [DONE]", "Should contain expected value")
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

				assert.Equal(t, "Thinking...", c["delta"], "Should equal expected value")
			}
		}

		assert.True(t, hasReasoningStart, "Should be true")
		assert.True(t, hasReasoningDelta, "Should be true")
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
		assert.NotContains(t, output, "reasoning-start", "Should not contain value")
		assert.NotContains(t, output, "reasoning-delta", "Should not contain value")
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

				assert.Equal(t, "call_1", c["toolCallID"], "Should equal expected value")
				assert.Equal(t, "get_weather", c["toolName"], "Should equal expected value")
			}

			if c["type"] == "tool-input-available" {
				hasToolInputAvailable = true
			}
		}

		assert.True(t, hasToolInputStart, "Should be true")
		assert.True(t, hasToolInputAvailable, "Should be true")
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

				assert.Equal(t, "call_1", c["toolCallID"], "Should equal expected value")
			}
		}

		assert.True(t, hasToolOutput, "Should be true")
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
		assert.Contains(t, output, "data-status", "Should contain expected value")
	})

	t.Run("HandlesErrorFromSource", func(t *testing.T) {
		expectedErr := errors.New("source error")
		source := NewCallbackSource(func(_ CallbackWriter) error {
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

				assert.Equal(t, "source error", c["errorText"], "Should equal expected value")
			}
		}

		assert.True(t, hasError, "Should be true")
	})

	t.Run("CallsOnErrorHandler", func(t *testing.T) {
		expectedErr := errors.New("test error")
		source := NewCallbackSource(func(_ CallbackWriter) error {
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
		assert.Contains(t, output, "Custom: test error", "Should contain expected value")
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

		assert.Equal(t, "Hello World", finishedContent, "Should equal expected value")
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
			assert.NotEqual(t, "start", c["type"], "Should not equal")
			assert.NotEqual(t, "start-step", c["type"], "Should not equal")
			assert.NotEqual(t, "finish", c["type"], "Should not equal")
			assert.NotEqual(t, "finish-step", c["type"], "Should not equal")
		}
	})
}

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

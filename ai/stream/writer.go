package stream

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/id"
)

// StreamWriter writes UI message stream chunks.
type StreamWriter interface {
	// WriteChunk writes a single chunk to the stream output.
	WriteChunk(chunk Chunk) error
	// Flush flushes any buffered data to the underlying writer.
	Flush() error
}

// ResponseWriter is compatible with fiber.Ctx.SendStreamWriter.
type ResponseWriter interface {
	io.Writer
}

func defaultIDGenerator(prefix string) string {
	return prefix + "_" + id.GenerateUUID()
}

type sseWriter struct {
	w *bufio.Writer
}

func newSseWriter(w *bufio.Writer) *sseWriter {
	return &sseWriter{w: w}
}

func (s *sseWriter) WriteChunk(chunk Chunk) error {
	data, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("failed to marshal chunk: %w", err)
	}

	if _, err := s.w.WriteString("data: " + string(data) + "\n\n"); err != nil {
		return fmt.Errorf("failed to write sse data: %w", err)
	}

	return s.w.Flush()
}

func (s *sseWriter) Flush() error {
	return s.w.Flush()
}

func (s *sseWriter) writeDone() error {
	if _, err := s.w.WriteString("data: [DONE]\n\n"); err != nil {
		return err
	}

	return s.w.Flush()
}

// SseHeaders contains the standard headers for AI SDK UI Message Stream.
var SseHeaders = map[string]string{
	fiber.HeaderContentType:         "text/event-stream",
	fiber.HeaderCacheControl:        "no-cache",
	fiber.HeaderConnection:          "keep-alive",
	fiber.HeaderTransferEncoding:    "chunked",
	"X-Vercel-AI-UI-Message-Stream": "v1",
	"X-Accel-Buffering":             "no",
}

package exec

import (
	"context"
	"net/http"
	"time"

	"github.com/coldsmirk/vef-framework-go/integration"
)

// ConnectionCheck is the outcome of a connection probe. Transport failures
// are data (Reachable=false), not errors — the probe answered the question.
type ConnectionCheck struct {
	Reachable  bool   `json:"reachable"`
	Status     int    `json:"status,omitempty"`
	StatusText string `json:"statusText,omitempty"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

// TestConnection probes system with a single request — the entry point
// behind a management UI "test connection" button. Configuration faults
// (unknown auth scheme, undecryptable credential) return an error; any
// completed exchange, whatever its status, reports Reachable.
func (inv *Invoker) TestConnection(ctx context.Context, system *integration.System, method, path string) (*ConnectionCheck, error) {
	client, err := inv.clients.ClientFor(system)
	if err != nil {
		return nil, err
	}

	if method == "" {
		method = http.MethodGet
	}

	if path == "" {
		path = "/"
	}

	start := time.Now()

	resp, err := client.NewRequest().Do(ctx, method, path)
	if err != nil {
		// A transport failure is the probe's answer, not an error of the
		// probe itself.
		return &ConnectionCheck{DurationMs: time.Since(start).Milliseconds(), Error: err.Error()}, nil //nolint:nilerr
	}

	return &ConnectionCheck{
		Reachable:  true,
		Status:     resp.StatusCode(),
		StatusText: http.StatusText(resp.StatusCode()),
		DurationMs: resp.Duration().Milliseconds(),
	}, nil
}

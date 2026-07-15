package exec

import (
	"context"
	"net/http"
	"time"

	"github.com/coldsmirk/vef-framework-go/integration"
)

// ConnectionCheck is the outcome of probing a system's configured
// transports; each probe is present iff the system configures that
// transport. Probe failures are data (Reachable=false), not errors — the
// probe answered the question.
type ConnectionCheck struct {
	HTTP     *HTTPProbe     `json:"http,omitempty"`
	Database *DatabaseProbe `json:"database,omitempty"`
}

// HTTPProbe reports one probe request against the system base URL.
type HTTPProbe struct {
	Reachable  bool   `json:"reachable"`
	Status     int    `json:"status,omitempty"`
	StatusText string `json:"statusText,omitempty"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

// DatabaseProbe reports one throwaway connection against the system data
// source, carrying the server version on success.
type DatabaseProbe struct {
	Reachable  bool   `json:"reachable"`
	Version    string `json:"version,omitempty"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

// TestConnection probes system on every transport it configures — the entry
// point behind a management UI "test connection" button. Configuration
// faults (unknown auth scheme, undecryptable credential) return an error;
// any completed probe, whatever its result, reports as data.
func (inv *Invoker) TestConnection(ctx context.Context, system *integration.System, method, path string) (*ConnectionCheck, error) {
	check := new(ConnectionCheck)

	if system.BaseURL != "" {
		probe, err := inv.probeHTTP(ctx, system, method, path)
		if err != nil {
			return nil, err
		}

		check.HTTP = probe
	}

	if system.DataSource != nil {
		probe, err := inv.databases.Probe(ctx, system.DataSource)
		if err != nil {
			return nil, err
		}

		check.Database = probe
	}

	return check, nil
}

// probeHTTP performs the single HTTP probe request.
func (inv *Invoker) probeHTTP(ctx context.Context, system *integration.System, method, path string) (*HTTPProbe, error) {
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
		return &HTTPProbe{DurationMs: time.Since(start).Milliseconds(), Error: err.Error()}, nil //nolint:nilerr
	}

	return &HTTPProbe{
		Reachable:  true,
		Status:     resp.StatusCode(),
		StatusText: http.StatusText(resp.StatusCode()),
		DurationMs: resp.Duration().Milliseconds(),
	}, nil
}

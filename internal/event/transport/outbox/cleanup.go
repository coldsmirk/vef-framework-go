package outbox

import (
	"context"
	"time"

	puboutbox "github.com/coldsmirk/vef-framework-go/event/transport/outbox"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	publogx "github.com/coldsmirk/vef-framework-go/logx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Cleaner deletes completed outbox records older than the configured
// TTL so the table stays bounded while dead rows remain available for
// diagnostics.
type Cleaner struct {
	repo puboutbox.Repository
	ttl  time.Duration
	log  publogx.Logger
}

// NewCleaner constructs a Cleaner. A nil logger is replaced with
// logx.Discard so tests can omit it.
func NewCleaner(repo puboutbox.Repository, ttl time.Duration, log publogx.Logger) *Cleaner {
	if log == nil {
		log = logx.Discard()
	}

	return &Cleaner{repo: repo, ttl: ttl, log: log}
}

// Cleanup runs one delete cycle.
func (c *Cleaner) Cleanup(ctx context.Context) {
	cutoff := timex.Now().Add(-c.ttl)

	deleted, err := c.repo.DeleteCompletedOlderThan(ctx, cutoff)
	if err != nil {
		c.log.Errorf("outbox cleanup failed: %v", err)

		return
	}

	if deleted > 0 {
		c.log.Infof("outbox cleanup deleted %d record(s) older than %s", deleted, cutoff.Unwrap().Format(time.RFC3339))
	}
}

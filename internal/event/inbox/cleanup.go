package inbox

import (
	"context"
	"time"

	pubinbox "github.com/coldsmirk/vef-framework-go/event/inbox"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	publogx "github.com/coldsmirk/vef-framework-go/logx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Cleaner deletes inbox records older than the configured retention so
// the table stays bounded. Intended to be invoked from a cron job.
type Cleaner struct {
	repo      pubinbox.Repository
	retention time.Duration
	logger    publogx.Logger
}

// NewCleaner constructs a Cleaner. A nil logger is replaced with
// logx.Discard so tests can omit it.
func NewCleaner(repo pubinbox.Repository, retention time.Duration, log publogx.Logger) *Cleaner {
	if log == nil {
		log = logx.Discard()
	}

	return &Cleaner{repo: repo, retention: retention, logger: log}
}

// Cleanup runs one delete cycle, removing records older than the
// retention window. Safe to invoke periodically from a cron task.
func (c *Cleaner) Cleanup(ctx context.Context) {
	cutoff := timex.Now().Add(-c.retention)

	deleted, err := c.repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		c.logger.Errorf("inbox cleanup failed: %v", err)

		return
	}

	if deleted > 0 {
		c.logger.Infof("inbox cleanup deleted %d record(s) older than %s", deleted, cutoff.Unwrap().Format(time.RFC3339))
	}
}

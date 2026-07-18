package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/cron"
)

// SeedDefaultSchedules inserts every handler-shipped default schedule that
// does not exist yet. Existing rows — operator-tuned or created by a peer
// booting concurrently — are left untouched: shipping a default is an
// insert-if-absent, never an overwrite. An invalid shipped spec fails
// start-up; it is a programming error, not runtime input.
func SeedDefaultSchedules(ctx context.Context, manager cron.ScheduleManager, registry *Registry) error {
	for _, handler := range registry.All() {
		provider, ok := handler.(cron.DefaultScheduleProvider)
		if !ok {
			continue
		}

		spec := provider.DefaultSchedule()
		if spec.Name == "" {
			spec.Name = handler.Name()
		}

		if spec.JobName == "" {
			spec.JobName = handler.Name()
		}

		if _, err := manager.Create(ctx, spec); err != nil {
			if errors.Is(err, cron.ErrScheduleExists) {
				continue
			}

			return fmt.Errorf("seed default schedule %q of job %q: %w", spec.Name, handler.Name(), err)
		}

		logger.Infof("Seeded default schedule %q for job %q", spec.Name, handler.Name())
	}

	return nil
}

package timeout

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// HandoverScanner discovers eligible instances and delegates migration to the
// same transactional command as the service. Each dispatch locks one instance.
type HandoverScanner struct {
	db        orm.DB
	bus       cqrs.Bus
	handovers approval.HandoverResolver
}

func NewHandoverScanner(db orm.DB, bus cqrs.Bus, handovers approval.HandoverResolver) *HandoverScanner {
	return &HandoverScanner{db: db, bus: bus, handovers: handovers}
}

func registerHandoverJob(scheduler cron.Scheduler, scanner *HandoverScanner, cfg *config.ApprovalConfig) error {
	interval := cfg.EffectiveHandoverScanInterval()
	if interval == 0 {
		return nil
	}

	_, err := scheduler.NewJob(cron.NewDurationJob(interval, cron.WithName("approval:handover:scan"), cron.WithTags("approval", "handover"), cron.WithTask(scanner.Scan)))

	return err
}

func (s *HandoverScanner) Scan(ctx context.Context) {
	workCtx := orm.WithoutQuietSQLLog(ctx)
	ctx = orm.WithQuietSQLLog(ctx)

	var cursor string
	for {
		page, err := command.FindHandoverScanPage(ctx, s.db, cursor, scanBatchSize)
		if err != nil {
			logger.Errorf("Scan approval handover candidates: %v", err)

			return
		}

		for _, c := range page {
			id := c.Instance.ID

			ready, err := c.NeedsMigration(ctx, s.handovers)
			if err != nil {
				logger.Errorf("Check handovers for instance %s: %v", id, err)

				continue
			}

			if !ready {
				continue
			}

			_, err = cqrs.Send[command.ApplyHandoversCmd, *approval.HandoverReport](
				workCtx,
				s.bus,
				command.ApplyHandoversCmd{ScanInstanceID: id, Operator: shared.SystemOperator, Caller: approval.SystemCaller},
			)
			if err != nil {
				logger.Errorf("Apply handovers to instance %s: %v", id, err)
			}
		}

		if len(page) < scanBatchSize {
			return
		}

		cursor = page[len(page)-1].Instance.ID
	}
}

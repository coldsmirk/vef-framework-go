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
	db  orm.DB
	bus cqrs.Bus
}

func NewHandoverScanner(db orm.DB, bus cqrs.Bus) *HandoverScanner {
	return &HandoverScanner{db: db, bus: bus}
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
		var ids []string

		err := command.HandoverCandidates(s.db, nil).Select("id").Where(func(cb orm.ConditionBuilder) { cb.GreaterThan("id", cursor) }).Limit(scanBatchSize).Scan(ctx, &ids)
		if err != nil {
			logger.Errorf("Scan approval handover candidates: %v", err)

			return
		}

		for _, id := range ids {
			_, err := cqrs.Send[command.ApplyHandoversCmd, *approval.HandoverReport](
				workCtx,
				s.bus,
				command.ApplyHandoversCmd{ScanInstanceID: id, Operator: shared.SystemOperator, Caller: approval.SystemCaller},
			)
			if err != nil {
				logger.Errorf("Apply handovers to instance %s: %v", id, err)
			}
		}

		if len(ids) < scanBatchSize {
			return
		}

		cursor = ids[len(ids)-1]
	}
}

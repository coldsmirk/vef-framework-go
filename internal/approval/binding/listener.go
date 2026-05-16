package binding

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

var logger = logx.Named("approval:binding")

// Listener subscribes to InstanceCompletedEvent and runs the business
// binding write-back asynchronously, decoupled from the approval transaction.
//
// Failure semantics: if OnInstanceCompleted returns an error, the listener
// publishes InstanceBindingFailedEvent so operators (or compensating workers)
// can retry. The approval status itself is never rolled back — the workflow
// has already decided.
type Listener struct {
	db   orm.DB
	bus  event.Bus
	hook approval.BusinessBindingHook
}

// NewListener constructs the listener. The hook is the host-overridable
// BusinessBindingHook bound via FX (defaults to DefaultHook).
func NewListener(db orm.DB, bus event.Bus, hook approval.BusinessBindingHook) *Listener {
	return &Listener{db: db, bus: bus, hook: hook}
}

// Start registers the event subscription. Called by FX Invoke during boot.
func (l *Listener) Start() error {
	_, err := event.SubscribeTyped(l.bus, l.handle)
	if err != nil {
		return fmt.Errorf("subscribe instance completed: %w", err)
	}

	logger.Infof("Instance binding listener subscribed to %s", new(approval.InstanceCompletedEvent).EventType())

	return nil
}

func (l *Listener) handle(ctx context.Context, evt *approval.InstanceCompletedEvent, _ event.Envelope) error {
	if l.hook == nil {
		return nil
	}

	var instance approval.Instance

	instance.ID = evt.InstanceID

	if err := l.db.NewSelect().
		Model(&instance).
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			// Instance went away between event publication and consumption;
			// nothing to bind. Acknowledge so the outbox doesn't retry forever.
			return nil
		}

		return fmt.Errorf("load instance for binding: %w", err)
	}

	var flow approval.Flow

	flow.ID = instance.FlowID

	if err := l.db.NewSelect().
		Model(&flow).
		WherePK().
		Scan(ctx); err != nil {
		return fmt.Errorf("load flow for binding: %w", err)
	}

	if flow.BindingMode != approval.BindingBusiness {
		return nil
	}

	if err := l.hook.OnInstanceCompleted(ctx, l.db, &flow, &instance, evt.FinalStatus); err != nil {
		// Surface as a domain event so operators / Saga workers can
		// retry. Failed bindings on a misconfigured flow surface with
		// ErrBindingMisconfigured; transient failures show their wrapped
		// cause. Either way we do not propagate the error back to the
		// event bus — the approval is final.
		businessTable := ""
		if flow.BusinessTable != nil {
			businessTable = *flow.BusinessTable
		}

		failureEvent := approval.NewInstanceBindingFailedEvent(
			instance.ID, instance.TenantID, flow.ID, evt.FinalStatus, businessTable, err.Error(),
		)

		if pubErr := l.bus.Publish(ctx, failureEvent); pubErr != nil {
			logger.Errorf("publish binding failure event for instance %s: %v", instance.ID, pubErr)
		}

		// Differentiate misconfiguration (caller bug) from transient
		// errors. Misconfigured flows shouldn't be retried by the outbox;
		// returning nil acknowledges the message. Transient errors are
		// returned so the framework retries until the budget runs out.
		if errors.Is(err, ErrBindingMisconfigured) {
			logger.Errorf("binding misconfigured for instance %s: %v", instance.ID, err)

			return nil
		}

		return fmt.Errorf("binding hook for instance %s: %w", instance.ID, err)
	}

	return nil
}

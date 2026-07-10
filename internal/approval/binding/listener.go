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

// Listener subscribes to the instance-lifecycle events that drive the
// asynchronous legs of the engine-owned business write-back — completed,
// returned, withdrawn, resubmitted — decoupled from the approval
// transaction. (The started leg runs synchronously inside the
// start_instance transaction and never reaches this listener.)
//
// Failure semantics: if the write-back returns an error, the listener
// publishes InstanceBindingFailedEvent so operators (or compensating workers)
// can retry. The approval status itself is never rolled back — the workflow
// has already decided.
type Listener struct {
	db     orm.DB
	bus    event.Bus
	writer *Writer
}

// NewListener constructs the listener around the engine-owned Writer.
func NewListener(db orm.DB, bus event.Bus, writer *Writer) *Listener {
	return &Listener{db: db, bus: bus, writer: writer}
}

// bindingConsumerGroup is the stable consumer group name for the binding
// listener. It is the Inbox dedupe scope (so retries do not re-write the
// business table) and, when the route lands on Redis Streams, the XGROUP
// identifier (which must remain stable across restarts).
const bindingConsumerGroup = "approval:binding"

// Start registers the event subscriptions. Called by FX Invoke during boot.
func (l *Listener) Start() error {
	group := event.WithGroup(bindingConsumerGroup)

	if _, err := event.SubscribeTyped(l.bus, func(ctx context.Context, evt *approval.InstanceCompletedEvent, _ event.Envelope) error {
		return l.handle(ctx, evt.InstanceID, approval.BindingTriggerCompleted)
	}, group); err != nil {
		return fmt.Errorf("subscribe instance completed: %w", err)
	}

	if _, err := event.SubscribeTyped(l.bus, func(ctx context.Context, evt *approval.InstanceReturnedEvent, _ event.Envelope) error {
		return l.handle(ctx, evt.InstanceID, approval.BindingTriggerReturned)
	}, group); err != nil {
		return fmt.Errorf("subscribe instance returned: %w", err)
	}

	if _, err := event.SubscribeTyped(l.bus, func(ctx context.Context, evt *approval.InstanceWithdrawnEvent, _ event.Envelope) error {
		return l.handle(ctx, evt.InstanceID, approval.BindingTriggerWithdrawn)
	}, group); err != nil {
		return fmt.Errorf("subscribe instance withdrawn: %w", err)
	}

	if _, err := event.SubscribeTyped(l.bus, func(ctx context.Context, evt *approval.InstanceResubmittedEvent, _ event.Envelope) error {
		return l.handle(ctx, evt.InstanceID, approval.BindingTriggerResubmitted)
	}, group); err != nil {
		return fmt.Errorf("subscribe instance resubmitted: %w", err)
	}

	logger.Infof("Instance binding listener subscribed to %s / %s / %s / %s (group=%s)",
		new(approval.InstanceCompletedEvent).EventType(),
		new(approval.InstanceReturnedEvent).EventType(),
		new(approval.InstanceWithdrawnEvent).EventType(),
		new(approval.InstanceResubmittedEvent).EventType(),
		bindingConsumerGroup)

	return nil
}

// handle loads the instance and flow fresh (the write-back projects current
// state, so a late or redelivered event converges on the truth instead of
// replaying a stale snapshot) and runs the engine-owned write-back for the
// trigger.
func (l *Listener) handle(ctx context.Context, instanceID string, trigger approval.BindingTrigger) error {
	if l.writer == nil {
		return nil
	}

	var (
		instance       approval.Instance
		flow           approval.Flow
		writeAttempted bool
	)

	err := l.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		instance.ID = instanceID
		if err := tx.NewSelect().Model(&instance).WherePK().Scan(ctx); err != nil {
			if result.IsRecordNotFound(err) {
				// Instance went away between event publication and consumption;
				// nothing to bind. Acknowledge so the outbox doesn't retry forever.
				return nil
			}

			return fmt.Errorf("load instance for binding: %w", err)
		}

		flow.ID = instance.FlowID
		if err := tx.NewSelect().Model(&flow).WherePK().Scan(ctx); err != nil {
			return fmt.Errorf("load flow for binding: %w", err)
		}

		if flow.BindingMode != approval.BindingBusiness {
			return nil
		}

		writeAttempted = true

		return l.writer.WriteBack(ctx, tx, &flow, &instance, trigger)
	})
	if err != nil {
		if !writeAttempted {
			return err
		}

		// Surface as a domain event so operators / Saga workers can
		// retry. Failed bindings on a misconfigured flow surface with
		// ErrBindingMisconfigured; transient failures show their wrapped
		// cause. Either way we do not propagate the error back to the
		// event bus — the approval action is already committed.
		businessTable := ""
		if flow.BusinessBinding != nil {
			businessTable = flow.BusinessBinding.TableName
		}

		failureEvent := approval.NewInstanceBindingFailedEvent(
			&instance, trigger, instance.Status, businessTable, err.Error(),
		)

		if pubErr := l.publishFailure(ctx, failureEvent); pubErr != nil {
			logger.Errorf("publish binding failure event for instance %s: %v", instance.ID, pubErr)
		}

		// Differentiate permanent binding failures from transient errors.
		// Invalid configuration, refs, and target cardinality do not heal by
		// retry; returning nil acknowledges them. Transient errors are returned
		// so the framework retries until the budget runs out.
		if isPermanentError(err) {
			logger.Errorf("binding permanently failed for instance %s: %v", instance.ID, err)

			return nil
		}

		return fmt.Errorf("binding write-back (%s) for instance %s: %w", trigger, instance.ID, err)
	}

	return nil
}

func (l *Listener) publishFailure(ctx context.Context, failureEvent approval.DomainEvent) error {
	opts := []event.PublishOption{}
	if t := approval.PayloadOccurredAt(failureEvent); !t.IsZero() {
		opts = append(opts, event.WithOccurredAt(t.Unwrap()))
	}

	if l.db == nil {
		return l.bus.Publish(ctx, failureEvent, opts...)
	}

	err := l.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		return l.bus.Publish(ctx, failureEvent, append(opts, event.WithTx(tx))...)
	})
	if err == nil {
		return nil
	}

	if errors.Is(err, event.ErrTxRequired) {
		return l.bus.Publish(ctx, failureEvent, opts...)
	}

	return err
}

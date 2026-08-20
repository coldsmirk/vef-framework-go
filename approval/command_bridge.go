package approval

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/coldsmirk/vef-framework-go/cqrs"
	"github.com/coldsmirk/vef-framework-go/event"
)

// ErrNonCommandAction is returned by BindCommand when the bound action type
// is a query. The bridge exists to turn approval facts into side effects; a
// query has no side effect to trigger, so binding one is always a
// programming error.
var ErrNonCommandAction = errors.New(
	"approval: BindCommand requires a command action type; queries cannot be bound to instance events",
)

// ErrUnnamedCommandType is returned when the bound command type is not a
// named concrete type (an anonymous struct or an interface). Commands are the
// stable vocabulary of a host: the bridge derives the consumer group, log
// lines, and the dispatch key from the type's identity, which such types do
// not have.
var ErrUnnamedCommandType = errors.New(
	"approval: bound command must be a named concrete type",
)

// derivedCommandGroupPrefix parallels derivedGroupPrefix: handler-identity
// groups use "vef:sub:", command-identity groups use "vef:cmd:".
const derivedCommandGroupPrefix = "vef:cmd:"

// BindCommand subscribes to instance event E and dispatches the mapped
// command C through the host's CQRS bus — the declarative bridge from
// approval facts to host side effects. mapper is a pure translation: it
// shapes the command from the event plus its delivery Envelope and reports
// whether the event is relevant (ok=false acknowledges without
// dispatching). Business logic belongs in the command handler, which runs
// the host's full behavior pipeline (transaction, audit, validation); the
// handler's result is discarded — dispatch is fire-and-record, results
// belong to request/response callers.
//
// The consumer group defaults to the command type's identity ("vef:cmd:" +
// module-relative package + type name) — renaming or moving the command
// deliberately re-keys the subscription, exactly like renaming a
// SubscribeInstance handler. Binding the same command type twice in one
// process fails with ErrDerivedGroupConflict; WithGroup disambiguates (or
// pins the group ahead of a rename). Filters (ForFlows / ForTenants) and
// WithConcurrency apply as in SubscribeInstance.
//
// Delivery inherits the event route's semantics: an at-least-once transport
// (outbox / redis_stream) can redeliver, so the command handler must be
// idempotent — copy Envelope.ID (the Inbox dedupe key, stable across
// redeliveries) into the command when the handler needs a dedupe key of its
// own. Unlike the eventual business projection — which converges on the
// latest desired state and may skip intermediate statuses — every instance
// transition dispatches its own command, making this the lane for side
// effects tied to a specific lifecycle moment.
func BindCommand[E InstanceEvent, C cqrs.Action](
	bus event.Bus,
	commands cqrs.Bus,
	mapper func(evt E, env event.Envelope) (cmd C, ok bool),
	opts ...InstanceSubscribeOption,
) (event.Unsubscribe, error) {
	commandType, err := namedCommandType[C]()
	if err != nil {
		return nil, err
	}

	if actionKindFor(commandType) != cqrs.Command {
		return nil, ErrNonCommandAction
	}

	var cfg instanceSubscribeConfig
	for _, opt := range opts {
		opt.applyInstanceSubscribe(&cfg)
	}

	group := cfg.group
	derived := false

	if group == "" {
		group = derivedCommandGroupPrefix + trimMainModulePrefix(commandType.PkgPath()) + "." + commandType.Name()

		if !claimDerivedGroup(group) {
			return nil, ErrDerivedGroupConflict
		}

		derived = true

		var zero E

		subscribeLogger.Infof("Command binding for %s using derived group %q", zero.EventType(), group)
	}

	handler := func(ctx context.Context, evt E, env event.Envelope) error {
		cmd, ok := mapper(evt, env)
		if !ok {
			return nil
		}

		if _, err := cqrs.Send[C, any](ctx, commands, cmd); err != nil {
			return fmt.Errorf("dispatch %T for %s: %w", cmd, evt.EventType(), err)
		}

		return nil
	}

	unsubscribe, err := SubscribeInstance(bus, handler, append(slices.Clone(opts), WithGroup(group))...)
	if err != nil {
		if derived {
			releaseDerivedGroup(group)
		}

		return nil, err
	}

	if !derived {
		return unsubscribe, nil
	}

	return releaseOnce(unsubscribe, group), nil
}

// namedCommandType resolves C to its named concrete (non-pointer) type. The
// bridge needs one both to read the action kind and to derive a stable
// consumer group, so anonymous structs and interface-typed C are rejected.
func namedCommandType[C cqrs.Action]() (reflect.Type, error) {
	typ := reflect.TypeFor[C]()
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() == reflect.Interface || typ.PkgPath() == "" || typ.Name() == "" {
		return nil, ErrUnnamedCommandType
	}

	return typ, nil
}

// actionKindFor reads the Kind of the named command type by instantiating its
// zero value through a pointer — the pointer method set contains Kind for
// either receiver form that can satisfy cqrs.Action.
func actionKindFor(typ reflect.Type) cqrs.ActionKind {
	return reflect.New(typ).Interface().(cqrs.Action).Kind()
}

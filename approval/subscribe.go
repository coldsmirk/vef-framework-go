package approval

import (
	"context"
	"errors"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"sync"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

var subscribeLogger = logx.Named("approval:subscribe")

// ErrAnonymousSubscriberGroup is returned by SubscribeInstance when the
// handler is an anonymous function and no explicit group was supplied. The
// consumer group is the subscriber's durable identity (Redis XGROUP name,
// Inbox dedupe scope); an anonymous function's runtime name carries a
// positional counter (func1, func2, …) that silently changes when code
// moves, so it can never be a stable identity.
var ErrAnonymousSubscriberGroup = errors.New(
	"approval: anonymous handler cannot derive a stable consumer group; pass approval.WithGroup")

// ErrDerivedGroupConflict is returned when two subscriptions in the same
// process derive the same consumer group — almost always the same method
// subscribed twice. Explicit groups are exempt: sharing a named group is a
// legitimate load-balancing choice.
var ErrDerivedGroupConflict = errors.New(
	"approval: derived consumer group already registered in this process; pass approval.WithGroup to disambiguate")

// InstanceEvent is the closed set of instance-scoped domain events — every
// event type embedding InstanceEventBase satisfies it automatically. It is
// the type bound of SubscribeInstance.
type InstanceEvent interface {
	event.Event
	instanceEventBase() InstanceEventBase
}

// InstanceFilter is a declarative, data-backed routing filter for instance
// events and lifecycle hooks. It captures the routing question — "is this
// instance mine?" — as data rather than an opaque predicate, so the
// framework can evaluate it anywhere (consumer side today, transport
// push-down tomorrow) and operators can read a subscription's scope off its
// registration site.
//
// Semantics: within one filter, an empty dimension is unconstrained and a
// populated dimension matches when the value is in the list (OR); multiple
// filters passed to the same subscription must all match (AND). Business
// predicates (final status, form values, …) deliberately stay out — they
// belong in the handler.
type InstanceFilter struct {
	FlowCodes []string
	TenantIDs []string
}

// ForFlows restricts a subscription (or a filtered lifecycle hook) to
// instances of the named flow codes.
func ForFlows(codes ...string) InstanceFilter {
	return InstanceFilter{FlowCodes: codes}
}

// ForTenants restricts a subscription (or a filtered lifecycle hook) to
// instances of the named tenants.
func ForTenants(ids ...string) InstanceFilter {
	return InstanceFilter{TenantIDs: ids}
}

// Matches reports whether an instance identified by flowCode / tenantID
// passes this filter.
func (f InstanceFilter) Matches(flowCode, tenantID string) bool {
	if len(f.FlowCodes) > 0 && !slices.Contains(f.FlowCodes, flowCode) {
		return false
	}

	if len(f.TenantIDs) > 0 && !slices.Contains(f.TenantIDs, tenantID) {
		return false
	}

	return true
}

// matchesAll reports whether every filter accepts the instance. No filters
// means unconstrained.
func matchesAll(filters []InstanceFilter, flowCode, tenantID string) bool {
	for _, f := range filters {
		if !f.Matches(flowCode, tenantID) {
			return false
		}
	}

	return true
}

// applyInstanceSubscribe makes InstanceFilter usable directly as a
// SubscribeInstance option.
func (f InstanceFilter) applyInstanceSubscribe(cfg *instanceSubscribeConfig) {
	cfg.filters = append(cfg.filters, f)
}

// InstanceSubscribeOption configures a SubscribeInstance call. InstanceFilter
// values are options themselves, so filters and settings mix in one list.
type InstanceSubscribeOption interface {
	applyInstanceSubscribe(*instanceSubscribeConfig)
}

type instanceSubscribeConfig struct {
	group       string
	concurrency int
	filters     []InstanceFilter
}

type instanceSubscribeOptionFunc func(*instanceSubscribeConfig)

func (fn instanceSubscribeOptionFunc) applyInstanceSubscribe(cfg *instanceSubscribeConfig) { fn(cfg) }

// WithGroup pins the subscription's consumer group explicitly, overriding
// the derived default. Use it for production-critical subscribers (the name
// survives refactors) and whenever the handler is an anonymous function.
func WithGroup(name string) InstanceSubscribeOption {
	return instanceSubscribeOptionFunc(func(cfg *instanceSubscribeConfig) { cfg.group = name })
}

// WithConcurrency sets the per-subscription worker count, forwarded to the
// underlying event subscription.
func WithConcurrency(n int) InstanceSubscribeOption {
	return instanceSubscribeOptionFunc(func(cfg *instanceSubscribeConfig) { cfg.concurrency = n })
}

// SubscribeInstance subscribes a typed handler to one instance event type
// with declarative routing filters. Events whose envelope does not match
// every filter are acknowledged without invoking the handler — the filters
// answer "is this instance mine?", while business predicates (final status,
// form values) stay in the handler body. The handler also receives the
// delivery Envelope: Envelope.ID is the Inbox dedupe key, stable across
// redeliveries, and therefore the key to build manual idempotency on when
// the route is at-least-once.
//
// The consumer group defaults to a name derived from the handler's method
// identity, normalized into the same "vef:<scope>:<name>" shape as the
// framework's other groups: "smp/internal/mms.(*RightApplicationSvc).Handle-fm"
// becomes "vef:sub:internal/mms.RightApplicationSvc.Handle" (the main module
// prefix is stripped). Package-level named functions derive the same way;
// anonymous functions have no stable name and fail with
// ErrAnonymousSubscriberGroup unless WithGroup is given. Renaming or moving
// a handler changes its derived group — a new XGROUP starts and the old one
// is orphaned — so pin the group with WithGroup before such refactors.
// Deriving the same group twice in one process fails with
// ErrDerivedGroupConflict.
func SubscribeInstance[T InstanceEvent](
	bus event.Bus,
	handler func(ctx context.Context, evt T, env event.Envelope) error,
	opts ...InstanceSubscribeOption,
) (event.Unsubscribe, error) {
	var cfg instanceSubscribeConfig
	for _, opt := range opts {
		opt.applyInstanceSubscribe(&cfg)
	}

	group := cfg.group
	derived := false

	if group == "" {
		var err error

		group, err = deriveGroup(handler)
		if err != nil {
			return nil, err
		}

		if !claimDerivedGroup(group) {
			return nil, ErrDerivedGroupConflict
		}

		derived = true

		var zero T

		subscribeLogger.Infof("Instance subscription for %s using derived group %q",
			zero.EventType(), group)
	}

	subscribeOpts := []event.SubscribeOption{event.WithGroup(group)}
	if cfg.concurrency > 0 {
		subscribeOpts = append(subscribeOpts, event.WithConcurrency(cfg.concurrency))
	}

	filters := cfg.filters

	unsubscribe, err := event.SubscribeTyped(bus, func(ctx context.Context, evt T, env event.Envelope) error {
		base := evt.instanceEventBase()
		if !matchesAll(filters, base.FlowCode, base.TenantID) {
			return nil
		}

		return handler(ctx, evt, env)
	}, subscribeOpts...)
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

// releaseOnce wraps an inner unsubscribe and its derived-group release into
// one idempotent, concurrency-safe teardown — the Unsubscribe contract says
// subsequent calls are no-ops, and a repeated release would otherwise delete
// the claim of a newer subscription that re-derived the same group.
func releaseOnce(unsubscribe event.Unsubscribe, group string) event.Unsubscribe {
	var once sync.Once

	return func() {
		once.Do(func() {
			unsubscribe()
			releaseDerivedGroup(group)
		})
	}
}

// derivedGroupPrefix aligns derived names with the framework's existing
// auto-generated group shape ("vef:default:<uuid>" for blank fan-out groups).
const derivedGroupPrefix = "vef:sub:"

// anonymousFuncSuffix matches the positional segments the Go runtime appends
// to anonymous functions and closures (glob..func1, Method.func2.1, …).
var anonymousFuncSuffix = regexp.MustCompile(`\.func\d+(\.\d+)*$`)

// deriveGroup normalizes the handler's runtime identity into a stable
// consumer group name. Method values and package-level named functions have
// developer-chosen names and derive cleanly; anonymous functions carry
// positional counters and are rejected.
func deriveGroup(handler any) (string, error) {
	fn := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
	if fn == nil {
		return "", ErrAnonymousSubscriberGroup
	}

	// Method values are wrapped by the runtime with an "-fm" suffix.
	name := strings.TrimSuffix(fn.Name(), "-fm")

	if anonymousFuncSuffix.MatchString(name) {
		return "", ErrAnonymousSubscriberGroup
	}

	// Split "<pkg path>.<symbol>": the first dot after the last slash ends
	// the package path (package names cannot contain dots; import paths can).
	slash := strings.LastIndex(name, "/")

	dot := strings.Index(name[slash+1:], ".")
	if dot < 0 {
		return "", ErrAnonymousSubscriberGroup
	}

	dot += slash + 1
	pkgPath, symbol := name[:dot], name[dot+1:]

	// "(*Type).Method" → "Type.Method".
	symbol = strings.NewReplacer("(*", "", ")", "").Replace(symbol)

	return derivedGroupPrefix + trimMainModulePrefix(pkgPath) + "." + symbol, nil
}

// trimMainModulePrefix strips the main module path from pkgPath so derived
// consumer-group names do not repeat the module path on every name; the full
// import path is kept when the build carries no module info (best effort —
// the name stays stable either way).
func trimMainModulePrefix(pkgPath string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return pkgPath
	}

	return trimModulePrefix(pkgPath, info.Main.Path)
}

// trimModulePrefix strips modulePath from pkgPath only at a path-segment
// boundary — module "example.com/foo" must not swallow package
// "example.com/foobar/pkg", or two unrelated packages could derive the same
// consumer group. A package at the module root collapses to the module
// path's base name.
func trimModulePrefix(pkgPath, modulePath string) string {
	if modulePath == "" {
		return pkgPath
	}

	if pkgPath == modulePath {
		return path.Base(modulePath)
	}

	trimmed, found := strings.CutPrefix(pkgPath, modulePath+"/")
	if !found {
		return pkgPath
	}

	return trimmed
}

// derivedGroups guards against two subscriptions in one process deriving the
// same group name (same method registered twice). Explicit groups bypass it.
var derivedGroups = struct {
	mu    sync.Mutex
	names map[string]struct{}
}{names: make(map[string]struct{})}

func claimDerivedGroup(name string) bool {
	derivedGroups.mu.Lock()
	defer derivedGroups.mu.Unlock()

	if _, taken := derivedGroups.names[name]; taken {
		return false
	}

	derivedGroups.names[name] = struct{}{}

	return true
}

func releaseDerivedGroup(name string) {
	derivedGroups.mu.Lock()
	defer derivedGroups.mu.Unlock()

	delete(derivedGroups.names, name)
}

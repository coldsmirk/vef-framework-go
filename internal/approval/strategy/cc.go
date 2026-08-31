package strategy

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

var ccLogger = logx.Named("approval:cc")

// NewUserCCResolver creates a new UserCCResolver.
func NewUserCCResolver() approval.CCResolver {
	return new(UserCCResolver)
}

// UserCCResolver resolves CC recipients from fixed user IDs.
type UserCCResolver struct{}

func (*UserCCResolver) Describe() approval.KindDescriptor[approval.CCKind] {
	return approval.KindDescriptor[approval.CCKind]{
		Kind:      approval.CCUser,
		Label:     i18n.T("approval_cc_kind_user"),
		Selection: approval.SelectionUser,
	}
}

func (*UserCCResolver) Resolve(_ context.Context, rc *approval.CCResolveContext) ([]string, error) {
	return shared.NormalizeUniqueIDs(rc.IDs), nil
}

// NewRoleCCResolver creates a new RoleCCResolver.
func NewRoleCCResolver(svc approval.AssigneeService) approval.CCResolver {
	return &RoleCCResolver{svc: svc}
}

// RoleCCResolver resolves the members of the configured roles as recipients.
type RoleCCResolver struct {
	svc approval.AssigneeService
}

func (*RoleCCResolver) Describe() approval.KindDescriptor[approval.CCKind] {
	return approval.KindDescriptor[approval.CCKind]{
		Kind:      approval.CCRole,
		Label:     i18n.T("approval_cc_kind_role"),
		Selection: approval.SelectionRole,
	}
}

func (r *RoleCCResolver) Resolve(ctx context.Context, rc *approval.CCResolveContext) ([]string, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	return collectOrgUserIDs(ctx, rc.IDs, r.svc.GetRoleUsers)
}

// NewDepartmentCCResolver creates a new DepartmentCCResolver.
func NewDepartmentCCResolver(svc approval.AssigneeService) approval.CCResolver {
	return &DepartmentCCResolver{svc: svc}
}

// DepartmentCCResolver resolves the leaders of the configured departments as
// recipients, mirroring the assignee kind of the same name.
type DepartmentCCResolver struct {
	svc approval.AssigneeService
}

func (*DepartmentCCResolver) Describe() approval.KindDescriptor[approval.CCKind] {
	return approval.KindDescriptor[approval.CCKind]{
		Kind:      approval.CCDepartment,
		Label:     i18n.T("approval_cc_kind_department"),
		Selection: approval.SelectionDepartment,
	}
}

func (r *DepartmentCCResolver) Resolve(ctx context.Context, rc *approval.CCResolveContext) ([]string, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	return collectOrgUserIDs(ctx, rc.IDs, r.svc.GetDepartmentLeaders)
}

// NewFormFieldCCResolver creates a new FormFieldCCResolver.
func NewFormFieldCCResolver() approval.CCResolver {
	return new(FormFieldCCResolver)
}

// FormFieldCCResolver resolves CC recipients from a form field value.
type FormFieldCCResolver struct{}

func (*FormFieldCCResolver) Describe() approval.KindDescriptor[approval.CCKind] {
	return approval.KindDescriptor[approval.CCKind]{
		Kind:      approval.CCFormField,
		Label:     i18n.T("approval_cc_kind_form_field"),
		Selection: approval.SelectionFormField,
	}
}

func (*FormFieldCCResolver) Resolve(_ context.Context, rc *approval.CCResolveContext) ([]string, error) {
	ids, err := formFieldUserIDs(rc.FormField, rc.FormData)
	if err != nil {
		return nil, err
	}

	return shared.NormalizeUniqueIDs(ids), nil
}

// collectOrgUserIDs queries lookup for each configured ID and collects the
// resulting user IDs, unique in first-seen order.
func collectOrgUserIDs(ctx context.Context, ids []string, lookup func(context.Context, string) ([]approval.UserInfo, error)) ([]string, error) {
	out := shared.NewOrderedUnique[string](len(ids))

	for _, id := range shared.NormalizeUniqueIDs(ids) {
		users, err := lookup(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("resolve cc recipients for %q: %w", id, err)
		}

		for _, user := range users {
			if user.ID != "" {
				out.Add(user.ID)
			}
		}
	}

	return out.ToSlice(), nil
}

// CCConfigSelector decides whether a FlowNodeCC config should be included.
// The engine uses it to apply CCTiming, which is the engine's decision rather
// than a resolver's — see CCResolveContext.
type CCConfigSelector func(cfg approval.FlowNodeCC) bool

// CompositeCCResolver dispatches each CC rule to the resolver registered for
// its kind, and owns the best-effort collection policy that CC delivery runs
// under.
type CompositeCCResolver struct {
	ordered   []approval.CCResolver
	resolvers map[approval.CCKind]approval.CCResolver
}

// NewCompositeCCResolver merges host-registered resolvers onto the built-ins
// with the same override-by-kind semantics as the assignee side.
func NewCompositeCCResolver(builtins, hosts []approval.CCResolver) (*CompositeCCResolver, error) {
	ordered, index, err := overlayKinds(builtins, hosts)
	if err != nil {
		return nil, err
	}

	return &CompositeCCResolver{ordered: ordered, resolvers: index}, nil
}

// Descriptors returns the registered kinds in designer order.
func (c *CompositeCCResolver) Descriptors() []approval.KindDescriptor[approval.CCKind] {
	return describeAll(c.ordered)
}

// Describe returns the descriptor registered for a kind.
func (c *CompositeCCResolver) Describe(kind approval.CCKind) (approval.KindDescriptor[approval.CCKind], bool) {
	resolver, ok := c.resolvers[kind]
	if !ok {
		return approval.KindDescriptor[approval.CCKind]{}, false
	}

	return resolver.Describe(), true
}

// Resolve resolves a single CC configuration to recipient user IDs.
func (c *CompositeCCResolver) Resolve(ctx context.Context, cfg approval.FlowNodeCC, base *approval.NodeResolveContext) ([]string, error) {
	resolver, ok := c.resolvers[cfg.Kind]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrCCResolverNotFound, cfg.Kind)
	}

	return resolver.Resolve(ctx, &approval.CCResolveContext{
		NodeResolveContext: *base,
		Kind:               cfg.Kind,
		IDs:                cfg.IDs,
		FormField:          cfg.FormField,
	})
}

// CollectUserIDs resolves and deduplicates CC recipients while preserving
// first-seen order. It is the best-effort boundary for CC resolution: a config
// that cannot be resolved (missing AssigneeService, transient org-lookup
// error, unexpected form-field value, or an unregistered kind) is logged and
// skipped rather than failing the approval that triggered the CC — a
// notification side effect must never roll back the business decision.
func (c *CompositeCCResolver) CollectUserIDs(
	ctx context.Context,
	configs []approval.FlowNodeCC,
	base *approval.NodeResolveContext,
	selector CCConfigSelector,
) []string {
	userIDs := shared.NewOrderedUnique[string](len(configs))

	for _, cfg := range configs {
		if selector != nil && !selector(cfg) {
			continue
		}

		resolved, err := c.Resolve(ctx, cfg, base)
		if err != nil {
			ccLogger.Warnf("skipping cc config (kind=%s ids=%v): %v", cfg.Kind, cfg.IDs, err)

			continue
		}

		userIDs.AddAll(resolved...)
	}

	return userIDs.ToSlice()
}

package strategy

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cast"

	streams "github.com/coldsmirk/go-streams"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

// ResolveContext provides context for assignee resolution.
type ResolveContext struct {
	ApplicantID             string
	ApplicantName           string
	ApplicantDepartmentID   *string
	ApplicantDepartmentName *string
	FormData                approval.FormData
	UserResolver            approval.UserInfoResolver

	IDs       []string
	FormField *string
}

// AssigneeResolver resolves assignees for a specific kind.
type AssigneeResolver interface {
	// Kind returns the assignee kind this resolver handles (user, role, department_leader, superior, etc.).
	Kind() approval.AssigneeKind
	// Resolve resolves concrete assignees from the assignee configuration in ResolveContext.
	Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error)
}

// NewUserAssigneeResolver creates a new UserAssigneeResolver.
func NewUserAssigneeResolver() AssigneeResolver {
	return new(UserAssigneeResolver)
}

// UserAssigneeResolver resolves assignees from fixed user IDs.
type UserAssigneeResolver struct{}

func (*UserAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeUser }

func (*UserAssigneeResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	ids := make([]string, 0, len(rc.IDs))
	for _, rawID := range rc.IDs {
		if userID := strings.TrimSpace(rawID); userID != "" {
			ids = append(ids, userID)
		}
	}

	if len(ids) == 0 {
		return nil, nil
	}

	return resolveAssigneesByIDs(ctx, rc, ids, "user assignee resolver")
}

// NewRoleAssigneeResolver creates a new RoleAssigneeResolver.
func NewRoleAssigneeResolver(svc approval.AssigneeService) AssigneeResolver {
	return &RoleAssigneeResolver{svc: svc}
}

// RoleAssigneeResolver resolves assignees from role IDs via AssigneeService.
type RoleAssigneeResolver struct {
	svc approval.AssigneeService
}

func (*RoleAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeRole }

func (r *RoleAssigneeResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	return streams.CollectResults(streams.FlatMapErr(streams.FromSlice(rc.IDs), func(roleID string) (streams.Stream[approval.ResolvedAssignee], error) {
		users, err := r.svc.GetRoleUsers(ctx, roleID)
		if err != nil {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("role assignee resolver: %w", err)
		}

		return streams.MapTo(streams.FromSlice(users), userInfoToResolvedAssignee), nil
	}))
}

// NewDepartmentAssigneeResolver creates a new DepartmentAssigneeResolver.
func NewDepartmentAssigneeResolver(svc approval.AssigneeService) AssigneeResolver {
	return &DepartmentAssigneeResolver{svc: svc}
}

// DepartmentAssigneeResolver resolves the leaders of the configured
// department IDs (rc.IDs) as assignees.
type DepartmentAssigneeResolver struct {
	svc approval.AssigneeService
}

func (*DepartmentAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeDepartment }

func (r *DepartmentAssigneeResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	return streams.CollectResults(streams.FlatMapErr(streams.FromSlice(rc.IDs), func(departmentID string) (streams.Stream[approval.ResolvedAssignee], error) {
		leaders, err := r.svc.GetDepartmentLeaders(ctx, departmentID)
		if err != nil {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("department assignee resolver: %w", err)
		}

		return streams.MapTo(streams.FromSlice(leaders), userInfoToResolvedAssignee), nil
	}))
}

// NewSelfAssigneeResolver creates a new SelfAssigneeResolver.
func NewSelfAssigneeResolver() AssigneeResolver {
	return new(SelfAssigneeResolver)
}

// SelfAssigneeResolver resolves the applicant as assignee.
type SelfAssigneeResolver struct{}

func (*SelfAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeSelf }

func (*SelfAssigneeResolver) Resolve(_ context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if rc.ApplicantID == "" {
		return nil, ErrApplicantIDEmpty
	}

	return []approval.ResolvedAssignee{{User: approval.UserInfo{
		ID:             rc.ApplicantID,
		Name:           rc.ApplicantName,
		DepartmentID:   rc.ApplicantDepartmentID,
		DepartmentName: rc.ApplicantDepartmentName,
	}}}, nil
}

// NewSuperiorAssigneeResolver creates a new SuperiorAssigneeResolver.
func NewSuperiorAssigneeResolver(svc approval.AssigneeService) AssigneeResolver {
	return &SuperiorAssigneeResolver{svc: svc}
}

// SuperiorAssigneeResolver resolves the direct superior as assignee.
type SuperiorAssigneeResolver struct {
	svc approval.AssigneeService
}

func (*SuperiorAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeSuperior }

func (r *SuperiorAssigneeResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	info, err := r.svc.GetSuperior(ctx, rc.ApplicantID)
	if err != nil {
		return nil, fmt.Errorf("superior assignee resolver: %w", err)
	}

	if info == nil || info.ID == "" {
		return []approval.ResolvedAssignee{}, nil
	}

	return []approval.ResolvedAssignee{{User: *info}}, nil
}

// NewDepartmentLeaderAssigneeResolver creates a new DepartmentLeaderAssigneeResolver.
func NewDepartmentLeaderAssigneeResolver(svc approval.AssigneeService) AssigneeResolver {
	return &DepartmentLeaderAssigneeResolver{svc: svc}
}

// DepartmentLeaderAssigneeResolver resolves the leaders of the applicant's
// own department as assignees. This is a single-level lookup
// (GetDepartmentLeaders); it does not walk a multi-level supervisor chain.
type DepartmentLeaderAssigneeResolver struct {
	svc approval.AssigneeService
}

func (*DepartmentLeaderAssigneeResolver) Kind() approval.AssigneeKind {
	return approval.AssigneeDepartmentLeader
}

func (r *DepartmentLeaderAssigneeResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	if rc.ApplicantDepartmentID == nil || *rc.ApplicantDepartmentID == "" {
		return []approval.ResolvedAssignee{}, nil
	}

	leaders, err := r.svc.GetDepartmentLeaders(ctx, *rc.ApplicantDepartmentID)
	if err != nil {
		return nil, fmt.Errorf("department leader assignee resolver: %w", err)
	}

	return streams.MapTo(streams.FromSlice(leaders), userInfoToResolvedAssignee).Collect(), nil
}

// NewFormFieldAssigneeResolver creates a new FormFieldAssigneeResolver.
func NewFormFieldAssigneeResolver() AssigneeResolver {
	return new(FormFieldAssigneeResolver)
}

// FormFieldAssigneeResolver resolves assignees from a form field value.
type FormFieldAssigneeResolver struct{}

func (*FormFieldAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeFormField }

func (*FormFieldAssigneeResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if rc.FormField == nil || strings.TrimSpace(*rc.FormField) == "" {
		return nil, ErrFormFieldNameEmpty
	}

	field := strings.TrimSpace(*rc.FormField)
	value := rc.FormData.Get(field)

	var ids []string

	switch v := value.(type) {
	case nil:
		return []approval.ResolvedAssignee{}, nil
	case string:
		// A blank value means the applicant left the field empty — same as
		// an absent key. Resolve to no assignees so the node's
		// EmptyAssigneeAction decides, instead of failing the whole step.
		userID := strings.TrimSpace(v)
		if userID == "" {
			return []approval.ResolvedAssignee{}, nil
		}

		ids = []string{userID}

	case []string:
		for _, rawID := range v {
			if userID := strings.TrimSpace(rawID); userID != "" {
				ids = append(ids, userID)
			}
		}

	case []any:
		for _, item := range v {
			if uid := strings.TrimSpace(cast.ToString(item)); uid != "" {
				ids = append(ids, uid)
			}
		}

	default:
		return nil, fmt.Errorf("%w: %T", ErrUnsupportedFieldValueType, value)
	}

	if len(ids) == 0 {
		return []approval.ResolvedAssignee{}, nil
	}

	return resolveAssigneesByIDs(ctx, rc, ids, "form field assignee resolver")
}

// resolveAssigneesByIDs resolves user info for explicit IDs and converts them
// to assignees, preserving input order. IDs the resolver cannot find still
// yield an assignee (empty name) so a stale reference remains visible instead
// of silently vanishing.
func resolveAssigneesByIDs(ctx context.Context, rc *ResolveContext, ids []string, label string) ([]approval.ResolvedAssignee, error) {
	infos, err := shared.ResolveUserInfoMap(ctx, rc.UserResolver, ids)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}

	result := make([]approval.ResolvedAssignee, 0, len(ids))

	for _, userID := range ids {
		info := infos[userID]
		info.ID = userID
		result = append(result, approval.ResolvedAssignee{User: info})
	}

	return result, nil
}

// userInfoToResolvedAssignee converts a UserInfo to a ResolvedAssignee.
func userInfoToResolvedAssignee(info approval.UserInfo) approval.ResolvedAssignee {
	return approval.ResolvedAssignee{User: info}
}

// CompositeAssigneeResolver chains multiple resolvers and resolves assignees based on config kind.
type CompositeAssigneeResolver struct {
	resolvers map[approval.AssigneeKind]AssigneeResolver
}

// NewCompositeAssigneeResolver creates a composite resolver from individual resolvers.
func NewCompositeAssigneeResolver(resolvers ...AssigneeResolver) *CompositeAssigneeResolver {
	return &CompositeAssigneeResolver{
		resolvers: streams.AssociateBy(streams.FromSlice(resolvers), func(r AssigneeResolver) approval.AssigneeKind {
			return r.Kind()
		}),
	}
}

// ResolveAll resolves assignees from multiple configs.
func (c *CompositeAssigneeResolver) ResolveAll(ctx context.Context, assignees []approval.FlowNodeAssignee, baseRC *ResolveContext) ([]approval.ResolvedAssignee, error) {
	return streams.CollectResults(streams.FlatMapErr(streams.FromSlice(assignees), func(assignee approval.FlowNodeAssignee) (streams.Stream[approval.ResolvedAssignee], error) {
		resolver, ok := c.resolvers[assignee.Kind]
		if !ok {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("%w: %s", ErrAssigneeResolverNotFound, assignee.Kind)
		}

		rc := *baseRC
		rc.IDs = assignee.IDs
		rc.FormField = assignee.FormField

		resolved, err := resolver.Resolve(ctx, &rc)
		if err != nil {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("composite assignee resolver %q: %w", assignee.Kind, err)
		}

		return streams.FromSlice(resolved), nil
	}))
}

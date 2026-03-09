package strategy

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cast"

	streams "github.com/coldsmirk/go-streams"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ResolveContext provides context for assignee resolution.
type ResolveContext struct {
	DB              orm.DB
	ApplicantID     string
	ApplicantName   string
	ApplicantDeptID *string
	FormData        approval.FormData
	UserResolver    approval.UserInfoResolver

	IDs       []string
	FormField *string
}

// AssigneeResolver resolves assignees for a specific kind.
type AssigneeResolver interface {
	// Kind returns the assignee kind this resolver handles (user, role, dept_leader, superior, etc.).
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

	names, err := shared.ResolveUserNameMap(ctx, rc.UserResolver, ids)
	if err != nil {
		return nil, fmt.Errorf("user assignee resolver: %w", err)
	}

	result := make([]approval.ResolvedAssignee, 0, len(ids))
	for _, userID := range ids {
		result = append(result, approval.ResolvedAssignee{
			UserID:   userID,
			UserName: names[userID],
		})
	}

	return result, nil
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

// NewDeptAssigneeResolver creates a new DeptAssigneeResolver.
func NewDeptAssigneeResolver(svc approval.AssigneeService) AssigneeResolver {
	return &DeptAssigneeResolver{svc: svc}
}

// DeptAssigneeResolver resolves department leaders as assignees.
type DeptAssigneeResolver struct {
	svc approval.AssigneeService
}

func (*DeptAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeDept }

func (r *DeptAssigneeResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	return streams.CollectResults(streams.FlatMapErr(streams.FromSlice(rc.IDs), func(deptID string) (streams.Stream[approval.ResolvedAssignee], error) {
		leaders, err := r.svc.GetDeptLeaders(ctx, deptID)
		if err != nil {
			return streams.Empty[approval.ResolvedAssignee](), fmt.Errorf("dept assignee resolver: %w", err)
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

	return []approval.ResolvedAssignee{{UserID: rc.ApplicantID, UserName: rc.ApplicantName}}, nil
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

	return []approval.ResolvedAssignee{{UserID: info.ID, UserName: info.Name}}, nil
}

// NewDeptLeaderAssigneeResolver creates a new DeptLeaderAssigneeResolver.
func NewDeptLeaderAssigneeResolver(svc approval.AssigneeService) AssigneeResolver {
	return &DeptLeaderAssigneeResolver{svc: svc}
}

// DeptLeaderAssigneeResolver resolves department leaders as assignees.
type DeptLeaderAssigneeResolver struct {
	svc approval.AssigneeService
}

func (*DeptLeaderAssigneeResolver) Kind() approval.AssigneeKind { return approval.AssigneeDeptLeader }

func (r *DeptLeaderAssigneeResolver) Resolve(ctx context.Context, rc *ResolveContext) ([]approval.ResolvedAssignee, error) {
	if r.svc == nil {
		return nil, ErrAssigneeServiceNil
	}

	if rc.ApplicantDeptID == nil || *rc.ApplicantDeptID == "" {
		return []approval.ResolvedAssignee{}, nil
	}

	leaders, err := r.svc.GetDeptLeaders(ctx, *rc.ApplicantDeptID)
	if err != nil {
		return nil, fmt.Errorf("dept leader assignee resolver: %w", err)
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
		userID := strings.TrimSpace(v)
		if userID == "" {
			return nil, ErrFormFieldValueEmpty
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

	names, err := shared.ResolveUserNameMap(ctx, rc.UserResolver, ids)
	if err != nil {
		return nil, fmt.Errorf("form field assignee resolver: %w", err)
	}

	result := make([]approval.ResolvedAssignee, 0, len(ids))
	for _, userID := range ids {
		result = append(result, approval.ResolvedAssignee{
			UserID:   userID,
			UserName: names[userID],
		})
	}

	return result, nil
}

// userInfoToResolvedAssignee converts a UserInfo to a ResolvedAssignee.
func userInfoToResolvedAssignee(info approval.UserInfo) approval.ResolvedAssignee {
	return approval.ResolvedAssignee{UserID: info.ID, UserName: info.Name}
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

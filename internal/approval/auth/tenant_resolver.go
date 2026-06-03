package auth

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/security"
)

// ErrTenantNotResolved indicates the resolver received a principal whose
// Details shape doesn't expose a tenant_id / tenantId / TenantID key. The
// resource layer surfaces this as an authentication failure so a
// misconfigured host can't accidentally serve cross-tenant data through
// a zero-value CallerContext.
var ErrTenantNotResolved = errors.New("approval: cannot resolve caller tenant from principal")

// tenantFieldCandidates lists the keys (map case) and field names (struct
// case) the default resolver will probe in order. The first non-empty
// match wins; everything else is ignored.
var tenantFieldCandidates = []string{"tenant_id", "tenantId", "TenantID", "Tenant"}

// DefaultPrincipalTenantResolver extracts the tenant id from
// Principal.Details. It handles two common shapes:
//
//   - map[string]any (JWT claims deserialized without a typed model) —
//     looks up the candidate keys in order.
//   - typed struct or pointer to struct — looks up the candidate field
//     names in order via reflection.
//
// Anonymous and system principals (Details == nil) are explicitly
// recognized: they return an empty tenant without an error so the resource
// layer can treat them as needing an explicit fallback (e.g. the system
// caller marker). Authenticated principals whose Details does not expose
// a tenant return ErrTenantNotResolved — fail-closed.
type DefaultPrincipalTenantResolver struct{}

// NewDefaultPrincipalTenantResolver constructs the default resolver.
func NewDefaultPrincipalTenantResolver() approval.PrincipalTenantResolver {
	return new(DefaultPrincipalTenantResolver)
}

// Resolve probes Principal.Details for the first non-empty tenant field.
// See type docstring for the resolution policy.
func (*DefaultPrincipalTenantResolver) Resolve(_ context.Context, p *security.Principal) (string, error) {
	if p == nil || p.Details == nil {
		return "", nil
	}

	if tenant := lookupTenantInMap(p.Details); tenant != "" {
		return tenant, nil
	}

	if tenant := lookupTenantInStruct(p.Details); tenant != "" {
		return tenant, nil
	}

	return "", ErrTenantNotResolved
}

func lookupTenantInMap(details any) string {
	m, ok := details.(map[string]any)
	if !ok {
		return ""
	}

	for _, key := range tenantFieldCandidates {
		if v, ok := m[key].(string); ok && v != "" {
			return v
		}
	}

	return ""
}

func lookupTenantInStruct(details any) string {
	v := reflect.Indirect(reflect.ValueOf(details))
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return ""
	}

	t := v.Type()
	for i := range v.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		if !isTenantField(field) {
			continue
		}

		if s, ok := v.Field(i).Interface().(string); ok && s != "" {
			return s
		}
	}

	return ""
}

func isTenantField(f reflect.StructField) bool {
	for _, candidate := range tenantFieldCandidates {
		if f.Name == candidate {
			return true
		}

		if tag := f.Tag.Get("json"); tag != "" {
			name, _, _ := strings.Cut(tag, ",")
			if name == candidate {
				return true
			}
		}
	}

	return false
}

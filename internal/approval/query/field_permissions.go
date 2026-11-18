package query

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
)

func resolveViewerFieldPermissions(b *instanceDetailBundle, userID string) map[string]approval.Permission {
	return service.ResolveViewerFieldPermissions(&b.Instance, b.FormFields, b.FlowNodes, b.Tasks, b.CCRecords, userID)
}

// stripHiddenFormData returns a copy of formData with every key the viewer may
// not see (resolved permission hidden) removed. Keys absent from perms — legacy
// form data with no schema entry — are kept, since permissions govern schema
// fields only. The input map is never mutated; a nil input yields nil.
func stripHiddenFormData(formData map[string]any, perms map[string]approval.Permission) map[string]any {
	if formData == nil {
		return nil
	}

	filtered := make(map[string]any, len(formData))
	for k, v := range formData {
		if perms[k] == approval.PermissionHidden {
			continue
		}

		filtered[k] = v
	}

	return filtered
}

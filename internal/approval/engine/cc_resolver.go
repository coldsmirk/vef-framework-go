package engine

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

// ResolveCCUserIDs resolves CC recipients from static IDs or form fields.
func ResolveCCUserIDs(cfg approval.FlowNodeCC, formData approval.FormData) ([]string, error) {
	return shared.ResolveCCUserIDs(cfg, formData)
}

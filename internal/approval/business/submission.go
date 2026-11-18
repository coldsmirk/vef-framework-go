package business

import (
	"context"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Submit runs the version's handler and enforces a stable business reference.
func (r *Registry) Submit(ctx context.Context, db orm.DB, sc *approval.SubmissionContext) (*approval.Submission, error) {
	kind := sc.Version.BusinessKind
	if sc.Instance != nil {
		kind = sc.Instance.BusinessKind
	}

	if kind == nil {
		return nil, approval.ErrBusinessKindRequired
	}

	h, err := r.Require(*kind)
	if err != nil {
		return nil, err
	}

	result, err := h.Submit(ctx, db, sc)
	if err != nil {
		return nil, fmt.Errorf("submit business kind %q: %w", *kind, err)
	}

	if result == nil || strings.TrimSpace(result.BusinessRef) == "" {
		return nil, ErrEmptyReference
	}

	if sc.IsResubmission() && (sc.BusinessRef == nil || *sc.BusinessRef != result.BusinessRef) {
		return nil, ErrChangedReference
	}

	return &approval.Submission{BusinessRef: result.BusinessRef, FormData: Snapshot(sc.Version.FormFields, result.FormData)}, nil
}

// Snapshot retains only the keys named by the deployed field catalog.
func Snapshot(fields []approval.FormFieldDefinition, data map[string]any) map[string]any {
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		if v, ok := data[f.Key]; ok {
			out[f.Key] = v
		}
	}

	return out
}

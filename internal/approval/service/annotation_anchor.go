package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func (e *AnnotationEditor) validateAnchor(ctx context.Context, db orm.DB, ac *approval.AnchorContext) error {
	if len(ac.Anchor) == 0 {
		return nil
	}

	if !json.Valid(ac.Anchor) {
		return approval.ErrAnnotationAnchorInvalid
	}

	if ac.Instance.BusinessKind != nil {
		h, ok := e.business.Lookup(*ac.Instance.BusinessKind)

		validator, supports := h.(approval.AnnotationAnchorValidator)
		if !ok || !supports {
			return approval.ErrAnnotationAnchorInvalid
		}

		if err := validator.ValidateAnchor(ctx, db, ac); err != nil {
			return fmt.Errorf("%w: %w", approval.ErrAnnotationAnchorInvalid, err)
		}

		return nil
	}

	var anchor approval.FieldAnchor

	decoder := json.NewDecoder(bytes.NewReader(ac.Anchor))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&anchor); err != nil || anchor.Field == "" {
		return approval.ErrAnnotationAnchorInvalid
	}

	version := approval.FlowVersion{ID: ac.Instance.FlowVersionID}
	if err := db.NewSelect().Model(&version).Select("form_fields").WherePK().Scan(ctx); err != nil {
		return err
	}

	for _, field := range version.FormFields {
		if field.Key != anchor.Field {
			continue
		}

		if (anchor.Row != nil || anchor.Column != "") && field.Kind != approval.FieldTable {
			return approval.ErrAnnotationAnchorInvalid
		}

		if anchor.Row != nil && *anchor.Row < 0 {
			return approval.ErrAnnotationAnchorInvalid
		}

		if anchor.Column != "" && !slices.ContainsFunc(field.Columns, func(c approval.FormFieldDefinition) bool { return c.Key == anchor.Column }) {
			return approval.ErrAnnotationAnchorInvalid
		}
		// The held task contributes at least visible unless it explicitly hides this field.
		if ac.Node.FieldPermissions[field.Key] != approval.PermissionHidden {
			return nil
		}

		tasks, ccs, err := viewerRows(ctx, db, ac.Instance.ID, ac.Author.ID)
		if err != nil {
			return err
		}

		var nodes []approval.FlowNode
		if err := db.NewSelect().Model(&nodes).Select("id", "field_permissions").Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", version.ID) }).Scan(ctx); err != nil {
			return err
		}

		perms := ResolveViewerFieldPermissions(ac.Instance, version.FormFields, nodes, tasks, ccs, ac.Author.ID)
		if perms[field.Key] == approval.PermissionHidden {
			return approval.ErrAnnotationAnchorInvalid
		}

		return nil
	}

	return approval.ErrAnnotationAnchorInvalid
}

package query

import (
	"context"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// loadAnnotations runs after detail authorization. Administrative views select only published rows.
func loadAnnotations(ctx context.Context, db orm.DB, b *instanceDetailBundle, viewer *service.AnnotationViewer) ([]approval.AnnotationView, error) {
	var annotations []approval.Annotation
	err := db.NewSelect().Model(&annotations).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", b.Instance.ID).Group(func(cb orm.ConditionBuilder) {
			cb.Equals("status", approval.AnnotationPublished)
			if viewer != nil {
				cb.OrEquals("author_id", viewer.UserID)
			}
		})
	}).OrderBy("created_at").OrderBy("id").Scan(ctx)
	if err != nil {
		return nil, err
	}
	authors := make(map[string]string, len(annotations))
	for i := range annotations {
		authors[annotations[i].ID] = annotations[i].Author.ID
	}
	views := make([]approval.AnnotationView, 0, len(annotations))
	for i := range annotations {
		a := &annotations[i]
		parentAuthor := ""
		if a.ParentID != nil {
			parentAuthor = authors[*a.ParentID]
		}
		if viewer == nil || viewer.CanSee(a, parentAuthor) {
			views = append(views, a.View(b.NodeNameMap[a.NodeID]))
		}
	}
	return views, nil
}

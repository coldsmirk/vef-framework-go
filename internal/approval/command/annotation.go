package command

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

type SaveAnnotationCmd struct {
	cqrs.BaseCommand
	approval.SaveAnnotationInput
}
type DeleteAnnotationCmd struct {
	cqrs.BaseCommand
	approval.DeleteAnnotationInput
}
type SaveAnnotationHandler struct {
	db     orm.DB
	editor *service.AnnotationEditor
}
type DeleteAnnotationHandler struct {
	db     orm.DB
	editor *service.AnnotationEditor
}

func NewSaveAnnotationHandler(db orm.DB, editor *service.AnnotationEditor) *SaveAnnotationHandler {
	return &SaveAnnotationHandler{db: db, editor: editor}
}

func NewDeleteAnnotationHandler(db orm.DB, editor *service.AnnotationEditor) *DeleteAnnotationHandler {
	return &DeleteAnnotationHandler{db: db, editor: editor}
}

func (h *SaveAnnotationHandler) Handle(ctx context.Context, cmd SaveAnnotationCmd) (*approval.AnnotationView, error) {
	return h.editor.Save(ctx, contextx.DB(ctx, h.db), cmd.SaveAnnotationInput)
}

func (h *DeleteAnnotationHandler) Handle(ctx context.Context, cmd DeleteAnnotationCmd) (cqrs.Unit, error) {
	return cqrs.Unit{}, h.editor.Delete(ctx, contextx.DB(ctx, h.db), cmd.DeleteAnnotationInput)
}

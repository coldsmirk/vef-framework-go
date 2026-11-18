package approval

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/coldsmirk/vef-framework-go/copier"
	"github.com/coldsmirk/vef-framework-go/mapx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/validator"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

type modelBusinessOptions[M any] struct {
	access   func(context.Context, orm.DB, *SubmissionContext, *M) error
	snapshot func(*M) map[string]any
	view     func(context.Context, orm.DB, *ViewContext, *M) (any, error)
}

// ModelBusinessHandlerOption customizes a model-backed business handler.
type ModelBusinessHandlerOption[M any] func(*modelBusinessOptions[M])

// WithRecordAccess authorizes submission of an existing record under its row lock.
func WithRecordAccess[M any](check func(context.Context, orm.DB, *SubmissionContext, *M) error) ModelBusinessHandlerOption[M] {
	return func(o *modelBusinessOptions[M]) { o.access = check }
}

// WithSnapshot projects the business record into approval fields.
func WithSnapshot[M any](snapshot func(*M) map[string]any) ModelBusinessHandlerOption[M] {
	return func(o *modelBusinessOptions[M]) { o.snapshot = snapshot }
}

// WithView renders a viewer-scoped payload, including any field masking it needs.
func WithView[M any](view func(context.Context, orm.DB, *ViewContext, *M) (any, error)) ModelBusinessHandlerOption[M] {
	return func(o *modelBusinessOptions[M]) { o.view = view }
}

type modelBusinessHandler[M, P any] struct {
	modelBusinessOptions[M]
	descriptor BusinessKindDescriptor
	pk         *orm.Field
	columns    map[string]*orm.Field
}

// NewModelBusinessHandler serves CRUD records with a single string primary key.
// Embed the returned BusinessHandler in a host type to add optional capabilities
// such as business voiding or lifecycle hooks.
func NewModelBusinessHandler[M, P any](descriptor BusinessKindDescriptor, opts ...ModelBusinessHandlerOption[M]) (BusinessHandler, error) {
	if reflect.TypeFor[M]().Kind() != reflect.Struct || reflect.TypeFor[P]().Kind() != reflect.Struct {
		return nil, errModelBusinessTypes
	}
	// Bun's model metadata supplies the same key and field paths in all dialects.
	table := sqlitedialect.New().Tables().Get(reflect.TypeFor[M]())
	if len(table.PKs) != 1 || table.PKs[0].StructField.Type.Kind() != reflect.String {
		return nil, errModelBusinessKey
	}
	h := &modelBusinessHandler[M, P]{descriptor: descriptor, pk: table.PKs[0], columns: make(map[string]*orm.Field)}
	for _, f := range table.DataFields {
		name, _, _ := strings.Cut(f.StructField.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.GoName
		}
		h.columns[name] = f
	}
	for _, apply := range opts {
		apply(&h.modelBusinessOptions)
	}
	return h, nil
}

func (h *modelBusinessHandler[M, P]) Describe() BusinessKindDescriptor { return h.descriptor }

func (h *modelBusinessHandler[M, P]) load(ctx context.Context, db orm.DB, ref *string, lock bool) (*M, error) {
	if ref == nil || strings.TrimSpace(*ref) == "" {
		return nil, ErrInvalidBusinessRef
	}
	model := new(M)
	h.pk.Value(reflect.ValueOf(model).Elem()).SetString(*ref)
	q := db.NewSelect().Model(model).WherePK()
	if lock {
		q = q.ForUpdate()
	}
	if err := q.Scan(ctx); err != nil {
		return nil, err
	}
	return model, nil
}

func modelJSONMap[M any](model *M) (map[string]any, error) {
	encoded, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.UseNumber()
	var data map[string]any
	err = dec.Decode(&data)
	return data, err
}

func (h *modelBusinessHandler[M, P]) project(model *M) (map[string]any, error) {
	if h.snapshot != nil {
		return h.snapshot(model), nil
	}
	data, err := modelJSONMap(model)
	if err != nil {
		return nil, err
	}
	projected := make(map[string]any, len(h.descriptor.FormFields))
	for _, f := range h.descriptor.FormFields {
		if val, ok := data[f.Key]; ok {
			projected[f.Key] = val
		}
	}
	return projected, nil
}

func (h *modelBusinessHandler[M, P]) Submit(ctx context.Context, db orm.DB, sc *SubmissionContext) (*Submission, error) {
	updating := sc.IsResubmission() || sc.BusinessRef != nil
	if updating && !sc.IsResubmission() && h.access == nil {
		return nil, ErrInvalidBusinessRef
	}
	params, err := mapx.FromMap[P](sc.FormData)
	if err != nil {
		return nil, err
	}
	if err := validator.Validate(params); err != nil {
		return nil, err
	}
	model := new(M)
	if updating {
		model, err = h.load(ctx, db, sc.BusinessRef, true)
		if err != nil {
			return nil, err
		}
		if !sc.IsResubmission() {
			if err := h.access(ctx, db, sc, model); err != nil {
				return nil, err
			}
		}
	}
	if err := copier.Copy(params, model); err != nil {
		return nil, err
	}
	pk := h.pk.Value(reflect.ValueOf(model).Elem())
	if updating {
		pk.SetString(*sc.BusinessRef)
		_, err = db.NewUpdate().Model(model).WherePK().Exec(ctx)
	} else {
		_, err = db.NewInsert().Model(model).Exec(ctx)
	}
	if err != nil {
		return nil, err
	}
	data, err := h.project(model)
	if err != nil {
		return nil, err
	}
	return &Submission{BusinessRef: pk.String(), FormData: data}, nil
}

func (h *modelBusinessHandler[M, P]) OnTaskAction(ctx context.Context, db orm.DB, ac *TaskActionContext) (*TaskActionOutcome, error) {
	edits := make(map[string]any)
	var columns []string
	for key, value := range ac.FormData {
		perm := ac.FieldPermissions[key]
		if f := h.columns[key]; f != nil && (perm == PermissionEditable || perm == PermissionRequired) {
			edits[key] = value
			columns = append(columns, f.Name)
		}
	}
	if len(columns) == 0 {
		return nil, nil
	}
	model, err := h.load(ctx, db, ac.Instance.BusinessRef, true)
	if err != nil {
		return nil, err
	}
	patch, err := mapx.FromMap[M](edits)
	if err != nil {
		return nil, err
	}
	target, source := reflect.ValueOf(model).Elem(), reflect.ValueOf(patch).Elem()
	for key := range edits {
		f := h.columns[key]
		f.Value(target).Set(f.Value(source))
	}
	if _, err := db.NewUpdate().Model(model).Select(columns...).WherePK().Exec(ctx); err != nil {
		return nil, err
	}
	data, err := h.project(model)
	if err != nil {
		return nil, err
	}
	return &TaskActionOutcome{FormData: data}, nil
}

func (h *modelBusinessHandler[M, P]) View(ctx context.Context, db orm.DB, vc *ViewContext) (any, error) {
	model, err := h.load(ctx, db, vc.Instance.BusinessRef, false)
	if err != nil {
		return nil, err
	}
	if h.view != nil {
		return h.view(ctx, db, vc, model)
	}
	data, err := modelJSONMap(model)
	if err != nil {
		return nil, err
	}
	for key, perm := range vc.FieldPermissions {
		if perm == PermissionHidden {
			delete(data, key)
		}
	}
	return data, nil
}

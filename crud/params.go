package crud

import (
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/sortx"
)

// CreateManyParams is a wrapper type for batch create parameters.
type CreateManyParams[TParams any] struct {
	api.P

	List []TParams `json:"list" validate:"required,min=1,dive" label_i18n:"batch_create_list"`
}

// UpdateManyParams is a wrapper type for batch update parameters.
type UpdateManyParams[TParams any] struct {
	api.P

	List []TParams `json:"list" validate:"required,min=1,dive" label_i18n:"batch_update_list"`
}

// DeleteManyParams is a wrapper type for batch delete parameters.
// For single primary key models: PKs can be []any with direct values (e.g., ["id1", "id2"])
// For composite primary key models: PKs should be []map[string]any with each map containing all PK fields.
type DeleteManyParams struct {
	api.P

	PKs []any `json:"pks" validate:"required,min=1" label_i18n:"batch_delete_pks"`
}

// Sortable provides sorting capability for API search parameters.
type Sortable struct {
	api.M

	Sort []sortx.OrderSpec `json:"sort"`
}

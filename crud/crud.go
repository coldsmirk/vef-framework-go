package crud

import "github.com/coldsmirk/vef-framework-go/api"

// NewBuilder creates a new base API builder instance.
func NewBuilder[T any](self T, kind ...api.Kind) Builder[T] {
	k := api.KindRPC
	if len(kind) > 0 {
		k = kind[0]
	}

	return &baseBuilder[T]{self: self, kind: k}
}

func getAction(rpcAction, restAction string, kind ...api.Kind) string {
	if len(kind) > 0 && kind[0] == api.KindREST {
		return restAction
	}

	return rpcAction
}

// NewCreate creates a new Create instance for single record creation.
func NewCreate[TModel, TParams any](kind ...api.Kind) Create[TModel, TParams] {
	api := new(createOperation[TModel, TParams])
	api.Builder = NewBuilder[Create[TModel, TParams]](api, kind...)

	return api.Action(getAction(RPCActionCreate, RESTActionCreate, kind...))
}

// NewUpdate creates a new Update instance for single record update.
func NewUpdate[TModel, TParams any](kind ...api.Kind) Update[TModel, TParams] {
	api := new(updateOperation[TModel, TParams])
	api.Builder = NewBuilder[Update[TModel, TParams]](api, kind...)

	return api.Action(getAction(RPCActionUpdate, RESTActionUpdate, kind...))
}

// NewDelete creates a new Delete operation instance for single record deletion.
func NewDelete[TModel any](kind ...api.Kind) Delete[TModel] {
	api := new(deleteOperation[TModel])
	api.Builder = NewBuilder[Delete[TModel]](api, kind...)

	return api.Action(getAction(RPCActionDelete, RESTActionDelete, kind...))
}

// NewCreateMany creates a new CreateMany instance for batch creation.
func NewCreateMany[TModel, TParams any](kind ...api.Kind) CreateMany[TModel, TParams] {
	api := new(createManyOperation[TModel, TParams])
	api.Builder = NewBuilder[CreateMany[TModel, TParams]](api, kind...)

	return api.Action(getAction(RPCActionCreateMany, RESTActionCreateMany, kind...))
}

// NewUpdateMany creates a new UpdateMany instance for batch update.
func NewUpdateMany[TModel, TParams any](kind ...api.Kind) UpdateMany[TModel, TParams] {
	api := new(updateManyOperation[TModel, TParams])
	api.Builder = NewBuilder[UpdateMany[TModel, TParams]](api, kind...)

	return api.Action(getAction(RPCActionUpdateMany, RESTActionUpdateMany, kind...))
}

// NewDeleteMany creates a new DeleteMany instance for batch deletion.
func NewDeleteMany[TModel any](kind ...api.Kind) DeleteMany[TModel] {
	api := new(deleteManyOperation[TModel])
	api.Builder = NewBuilder[DeleteMany[TModel]](api, kind...)

	return api.Action(getAction(RPCActionDeleteMany, RESTActionDeleteMany, kind...))
}

// NewFind creates the base Find instance used by all find-type endpoints.
func NewFind[TModel, TSearch, TProcessor, TOperation any](self TOperation, kind ...api.Kind) Find[TModel, TSearch, TProcessor, TOperation] {
	return &baseFindOperation[TModel, TSearch, TProcessor, TOperation]{
		Builder: NewBuilder(self, kind...),

		self: self,
	}
}

// NewFindOne creates a new FindOne instance.
func NewFindOne[TModel, TSearch any](kind ...api.Kind) FindOne[TModel, TSearch] {
	api := new(findOneOperation[TModel, TSearch])
	api.Find = NewFind[TModel, TSearch, TModel, FindOne[TModel, TSearch]](
		api,
		kind...,
	)

	return api.Action(getAction(RPCActionFindOne, RESTActionFindOne, kind...))
}

// NewFindAll creates a new FindAll instance.
func NewFindAll[TModel, TSearch any](kind ...api.Kind) FindAll[TModel, TSearch] {
	api := new(findAllOperation[TModel, TSearch])
	api.Find = NewFind[TModel, TSearch, []TModel, FindAll[TModel, TSearch]](
		api,
		kind...,
	)

	return api.Action(getAction(RPCActionFindAll, RESTActionFindAll, kind...))
}

// NewFindPage creates a new FindPage instance.
func NewFindPage[TModel, TSearch any](kind ...api.Kind) FindPage[TModel, TSearch] {
	api := new(findPageOperation[TModel, TSearch])
	api.Find = NewFind[TModel, TSearch, []TModel, FindPage[TModel, TSearch]](
		api,
		kind...,
	)

	return api.Action(getAction(RPCActionFindPage, RESTActionFindPage, kind...))
}

// NewFindOptions creates a new FindOptions instance.
func NewFindOptions[TModel, TSearch any](kind ...api.Kind) FindOptions[TModel, TSearch] {
	api := new(findOptionsOperation[TModel, TSearch])
	api.Find = NewFind[TModel, TSearch, []DataOption, FindOptions[TModel, TSearch]](
		api,
		kind...,
	)

	return api.Action(getAction(RPCActionFindOptions, RESTActionFindOptions, kind...))
}

// NewFindTree creates a new FindTree for hierarchical data retrieval.
// The treeBuilder function converts flat database records into nested tree structures.
// Requires models to have id and parent_id columns for parent-child relationships.
func NewFindTree[TModel, TSearch any](
	treeBuilder func(flatModels []TModel) []TModel,
	kind ...api.Kind,
) FindTree[TModel, TSearch] {
	api := &findTreeOperation[TModel, TSearch]{
		idColumn:       IDColumn,
		parentIDColumn: ParentIDColumn,
		treeBuilder:    treeBuilder,
	}
	api.Find = NewFind[TModel, TSearch, []TModel, FindTree[TModel, TSearch]](
		api,
		kind...,
	)

	return api.Action(getAction(RPCActionFindTree, RESTActionFindTree, kind...))
}

// NewFindTreeOptions creates a new FindTreeOptions instance.
func NewFindTreeOptions[TModel, TSearch any](kind ...api.Kind) FindTreeOptions[TModel, TSearch] {
	api := &findTreeOptionsOperation[TModel, TSearch]{
		idColumn:       IDColumn,
		parentIDColumn: ParentIDColumn,
	}
	api.Find = NewFind[TModel, TSearch, []TreeDataOption, FindTreeOptions[TModel, TSearch]](
		api,
		kind...,
	)

	return api.Action(getAction(RPCActionFindTreeOptions, RESTActionFindTreeOptions, kind...))
}

// NewExport creates a new Export instance.
func NewExport[TModel, TSearch any](kind ...api.Kind) Export[TModel, TSearch] {
	api := new(exportOperation[TModel, TSearch])
	api.Find = NewFind[TModel, TSearch, []TModel, Export[TModel, TSearch]](
		api,
		kind...,
	)

	return api.Action(getAction(RPCActionExport, RESTActionExport, kind...))
}

// NewImport creates a new Import instance.
func NewImport[TModel any](kind ...api.Kind) Import[TModel] {
	api := new(importOperation[TModel])
	api.Builder = NewBuilder[Import[TModel]](api, kind...)

	return api.Action(getAction(RPCActionImport, RESTActionImport, kind...))
}

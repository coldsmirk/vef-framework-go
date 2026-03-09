package crud

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// Processor transforms query results after execution but before JSON serialization.
// Commonly used for data formatting, field selection, or computed properties.
type Processor[TIn, TSearch any] func(input TIn, search TSearch, ctx fiber.Ctx) any

// PreCreateProcessor handles business logic before model creation.
// Runs within the same transaction. Uses: validation, default values, related data setup.
type PreCreateProcessor[TModel, TParams any] func(model *TModel, params *TParams, query orm.InsertQuery, ctx fiber.Ctx, tx orm.DB) error

// PostCreateProcessor handles side effects after successful model creation.
// Runs within the same transaction. Uses: audit logging, notifications, cache updates.
type PostCreateProcessor[TModel, TParams any] func(model *TModel, params *TParams, ctx fiber.Ctx, tx orm.DB) error

// PreUpdateProcessor handles business logic before model update.
// Runs within the same transaction. Provides both old and new model states for comparison and validation.
type PreUpdateProcessor[TModel, TParams any] func(oldModel, model *TModel, params *TParams, query orm.UpdateQuery, ctx fiber.Ctx, tx orm.DB) error

// PostUpdateProcessor handles side effects after successful model update.
// Runs within the same transaction. Uses: audit trails, change notifications.
type PostUpdateProcessor[TModel, TParams any] func(oldModel, model *TModel, params *TParams, ctx fiber.Ctx, tx orm.DB) error

// PreDeleteProcessor handles validation and checks before model deletion.
// Runs within the same transaction. Common uses: referential integrity checks, soft delete logic.
type PreDeleteProcessor[TModel any] func(model *TModel, query orm.DeleteQuery, ctx fiber.Ctx, tx orm.DB) error

// PostDeleteProcessor handles cleanup tasks after successful deletion.
// Runs within the same transaction. Uses: cascade operations, audit logging.
type PostDeleteProcessor[TModel any] func(model *TModel, ctx fiber.Ctx, tx orm.DB) error

// PreCreateManyProcessor handles business logic before batch model creation.
// Runs within the same transaction. Common uses: batch validation, default values, related data setup.
type PreCreateManyProcessor[TModel, TParams any] func(models []TModel, paramsList []TParams, query orm.InsertQuery, ctx fiber.Ctx, tx orm.DB) error

// PostCreateManyProcessor handles side effects after successful batch model creation.
// Runs within the same transaction. Uses: audit logging, notifications, cache updates.
type PostCreateManyProcessor[TModel, TParams any] func(models []TModel, paramsList []TParams, ctx fiber.Ctx, tx orm.DB) error

// PreUpdateManyProcessor handles business logic before batch model update.
// Runs within the same transaction. Provides both old and new model states for comparison and validation.
type PreUpdateManyProcessor[TModel, TParams any] func(oldModels, models []TModel, paramsList []TParams, query orm.UpdateQuery, ctx fiber.Ctx, tx orm.DB) error

// PostUpdateManyProcessor handles side effects after successful batch model update.
// Runs within the same transaction. Uses: audit trails, change notifications.
type PostUpdateManyProcessor[TModel, TParams any] func(oldModels, models []TModel, paramsList []TParams, ctx fiber.Ctx, tx orm.DB) error

// PreDeleteManyProcessor handles validation and checks before batch model deletion.
// Runs within the same transaction. Common uses: referential integrity checks, soft delete logic.
type PreDeleteManyProcessor[TModel any] func(models []TModel, query orm.DeleteQuery, ctx fiber.Ctx, tx orm.DB) error

// PostDeleteManyProcessor handles cleanup tasks after successful batch deletion.
// Runs within the same transaction. Uses: cascade operations, audit logging.
type PostDeleteManyProcessor[TModel any] func(models []TModel, ctx fiber.Ctx, tx orm.DB) error

// PreExportProcessor handles data modification before exporting to Excel.
// Common uses: data formatting, field filtering, additional data loading.
type PreExportProcessor[TModel, TSearch any] func(models []TModel, search TSearch, ctx fiber.Ctx, db orm.DB) error

// FilenameBuilder generates the filename for exported Excel files based on search parameters.
// Allows dynamic filename generation with timestamp, filters, etc.
type FilenameBuilder[TSearch any] func(search TSearch, ctx fiber.Ctx) string

// PreImportProcessor handles validation and transformation before saving imported data.
// Runs within the same transaction. Common uses: data validation, default values, duplicate checking.
type PreImportProcessor[TModel any] func(models []TModel, query orm.InsertQuery, ctx fiber.Ctx, tx orm.DB) error

// PostImportProcessor handles side effects after successful import.
// Runs within the same transaction. Uses: audit logging, notifications, cache updates.
type PostImportProcessor[TModel any] func(models []TModel, ctx fiber.Ctx, tx orm.DB) error

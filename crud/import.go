package crud

import (
	"context"
	"mime/multipart"

	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/csv"
	"github.com/coldsmirk/vef-framework-go/excel"
	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/log"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/tabular"
)

// Import provides a fluent interface for building import endpoints.
// Parses uploaded Excel or Csv file and creates records in database.
type Import[TModel any] interface {
	api.OperationsProvider
	Builder[Import[TModel]]

	// WithDefaultFormat sets the default import format (Excel or CSV) when not specified in the request.
	WithDefaultFormat(format TabularFormat) Import[TModel]
	// WithExcelOptions configures Excel-specific import options (sheet index, header row, etc.).
	WithExcelOptions(opts ...excel.ImportOption) Import[TModel]
	// WithCsvOptions configures CSV-specific import options (delimiter, encoding, etc.).
	WithCsvOptions(opts ...csv.ImportOption) Import[TModel]
	// WithPreImport registers a processor that is called before parsed models are inserted into the database.
	WithPreImport(processor PreImportProcessor[TModel]) Import[TModel]
	// WithPostImport registers a processor that is called after models are inserted within the same transaction.
	WithPostImport(processor PostImportProcessor[TModel]) Import[TModel]
}

type importOperation[TModel any] struct {
	Builder[Import[TModel]]

	defaultFormat TabularFormat
	excelOpts     []excel.ImportOption
	csvOpts       []csv.ImportOption
	preImport     PreImportProcessor[TModel]
	postImport    PostImportProcessor[TModel]
}

func (i *importOperation[TModel]) Provide() []api.OperationSpec {
	return []api.OperationSpec{i.Build(i.importData)}
}

func (i *importOperation[TModel]) WithDefaultFormat(format TabularFormat) Import[TModel] {
	i.defaultFormat = format

	return i
}

func (i *importOperation[TModel]) WithExcelOptions(opts ...excel.ImportOption) Import[TModel] {
	i.excelOpts = opts

	return i
}

func (i *importOperation[TModel]) WithCsvOptions(opts ...csv.ImportOption) Import[TModel] {
	i.csvOpts = opts

	return i
}

func (i *importOperation[TModel]) WithPreImport(processor PreImportProcessor[TModel]) Import[TModel] {
	i.preImport = processor

	return i
}

func (i *importOperation[TModel]) WithPostImport(processor PostImportProcessor[TModel]) Import[TModel] {
	i.postImport = processor

	return i
}

type importParams struct {
	api.P

	File *multipart.FileHeader `json:"file"`
}

type importConfig struct {
	api.M

	Format TabularFormat `json:"format"`
}

func (i *importOperation[TModel]) importData() func(ctx fiber.Ctx, db orm.DB, logger log.Logger, config importConfig, params importParams) error {
	excelImporter := excel.NewImporterFor[TModel](i.excelOpts...)
	csvImporter := csv.NewImporterFor[TModel](i.csvOpts...)

	return func(ctx fiber.Ctx, db orm.DB, logger log.Logger, config importConfig, params importParams) error {
		// Import requests must use multipart/form-data format
		if httpx.IsJSON(ctx) {
			return result.Err(i18n.T("import_requires_multipart"))
		}

		if params.File == nil {
			return result.Err(i18n.T("import_requires_file"))
		}

		var (
			format   = lo.CoalesceOrEmpty(config.Format, i.defaultFormat, FormatExcel)
			importer tabular.Importer
		)

		switch format {
		case FormatExcel:
			importer = excelImporter
		case FormatCsv:
			importer = csvImporter
		default:
			return result.Err(i18n.T("unsupported_import_format"))
		}

		file, err := params.File.Open()
		if err != nil {
			return result.Err(i18n.T("file_open_failed"))
		}

		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				logger.Errorf("failed to close file: %v", closeErr)
			}
		}()

		modelsAny, importErrors, err := importer.Import(file)
		if err != nil {
			return err
		}

		models, ok := modelsAny.([]TModel)
		if !ok {
			return result.Err("import type assertion failed")
		}

		if len(importErrors) > 0 {
			return result.Result{
				Code:    result.ErrCodeDefault,
				Message: i18n.T("import_validation_failed"),
				Data: fiber.Map{
					"errors": importErrors,
				},
			}.Response(ctx)
		}

		return db.RunInTX(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
			query := tx.NewInsert().Model(&models)
			if i.preImport != nil {
				if err := i.preImport(models, query, ctx, tx); err != nil {
					return err
				}
			}

			if len(models) > 0 {
				if _, err := query.Exec(txCtx); err != nil {
					return err
				}
			}

			if i.postImport != nil {
				if err := i.postImport(models, ctx, tx); err != nil {
					return err
				}
			}

			return result.Ok(
				fiber.Map{
					"total": len(models),
				}).
				Response(ctx)
		})
	}
}

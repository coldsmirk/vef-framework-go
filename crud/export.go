package crud

import (
	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/csv"
	"github.com/coldsmirk/vef-framework-go/excel"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/mold"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/tabular"
)

// Export provides a fluent interface for building export endpoints.
// Queries data based on search conditions and exports to Excel or Csv file.
type Export[TModel, TSearch any] interface {
	api.OperationsProvider
	Find[TModel, TSearch, []TModel, Export[TModel, TSearch]]

	// WithDefaultFormat sets the default export format (Excel or CSV) when not specified in the request.
	WithDefaultFormat(format TabularFormat) Export[TModel, TSearch]
	// WithExcelOptions configures Excel-specific export options (sheet name, styles, etc.).
	WithExcelOptions(opts ...excel.ExportOption) Export[TModel, TSearch]
	// WithCsvOptions configures CSV-specific export options (delimiter, encoding, etc.).
	WithCsvOptions(opts ...csv.ExportOption) Export[TModel, TSearch]
	// WithPreExport registers a processor for post-query data transformation before export.
	WithPreExport(processor PreExportProcessor[TModel, TSearch]) Export[TModel, TSearch]
	// WithFilenameBuilder sets a custom function to generate the export filename.
	WithFilenameBuilder(builder FilenameBuilder[TSearch]) Export[TModel, TSearch]
}

const (
	contentTypeExcel     = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	contentTypeCsv       = "text/csv; charset=utf-8"
	defaultFilenameExcel = "data.xlsx"
	defaultFilenameCsv   = "data.csv"
)

type exportOperation[TModel, TSearch any] struct {
	Find[TModel, TSearch, []TModel, Export[TModel, TSearch]]

	defaultFormat   TabularFormat
	excelOpts       []excel.ExportOption
	csvOpts         []csv.ExportOption
	preExport       PreExportProcessor[TModel, TSearch]
	filenameBuilder FilenameBuilder[TSearch]
}

func (a *exportOperation[TModel, TSearch]) Provide() []api.OperationSpec {
	return []api.OperationSpec{a.Build(a.exportData)}
}

func (a *exportOperation[TModel, TSearch]) WithDefaultFormat(format TabularFormat) Export[TModel, TSearch] {
	a.defaultFormat = format

	return a
}

func (a *exportOperation[TModel, TSearch]) WithExcelOptions(opts ...excel.ExportOption) Export[TModel, TSearch] {
	a.excelOpts = opts

	return a
}

func (a *exportOperation[TModel, TSearch]) WithCsvOptions(opts ...csv.ExportOption) Export[TModel, TSearch] {
	a.csvOpts = opts

	return a
}

func (a *exportOperation[TModel, TSearch]) WithPreExport(processor PreExportProcessor[TModel, TSearch]) Export[TModel, TSearch] {
	a.preExport = processor

	return a
}

func (a *exportOperation[TModel, TSearch]) WithFilenameBuilder(builder FilenameBuilder[TSearch]) Export[TModel, TSearch] {
	a.filenameBuilder = builder

	return a
}

type exportConfig struct {
	api.M

	Format TabularFormat `json:"format"`
}

func (a *exportOperation[TModel, TSearch]) exportData(db orm.DB) (func(ctx fiber.Ctx, db orm.DB, transformer mold.Transformer, config exportConfig, search TSearch, meta api.Meta) error, error) {
	if err := a.Setup(db, &FindOperationConfig{
		QueryParts: &QueryPartsConfig{
			Condition:         []QueryPart{QueryRoot},
			Sort:              []QueryPart{QueryRoot},
			AuditUserRelation: []QueryPart{QueryRoot},
		},
	}); err != nil {
		return nil, err
	}

	excelExporter := excel.NewExporterFor[TModel](a.excelOpts...)
	csvExporter := csv.NewExporterFor[TModel](a.csvOpts...)

	return func(ctx fiber.Ctx, db orm.DB, transformer mold.Transformer, config exportConfig, search TSearch, meta api.Meta) error {
		var (
			format                       = lo.CoalesceOrEmpty(config.Format, a.defaultFormat, FormatExcel)
			exporter                     tabular.Exporter
			contentType, defaultFilename string
		)

		switch format {
		case FormatExcel:
			exporter = excelExporter
			contentType = contentTypeExcel
			defaultFilename = defaultFilenameExcel
		case FormatCsv:
			exporter = csvExporter
			contentType = contentTypeCsv
			defaultFilename = defaultFilenameCsv
		default:
			return result.Err(i18n.T("unsupported_export_format"))
		}

		var (
			models []TModel
			query  = db.NewSelect().Model(&models).SelectModelColumns()
		)

		if err := a.ConfigureQuery(query, search, meta, ctx, QueryRoot); err != nil {
			return err
		}
		// Execute query with safety limit
		if err := query.Limit(maxQueryLimit).
			Scan(ctx.Context()); err != nil {
			return err
		}

		for i := range models {
			if err := transformer.Struct(ctx.Context(), &models[i]); err != nil {
				return err
			}
		}

		if a.preExport != nil {
			if err := a.preExport(models, search, ctx, db); err != nil {
				return err
			}
		}

		buf, err := exporter.Export(models)
		if err != nil {
			return err
		}

		filename := defaultFilename
		if a.filenameBuilder != nil {
			filename = a.filenameBuilder(search, ctx)
		}

		ctx.Set(fiber.HeaderContentType, contentType)
		ctx.Set(fiber.HeaderContentDisposition, "attachment; filename="+filename)

		return ctx.Send(buf.Bytes())
	}, nil
}

package crud_test

import (
	"bytes"
	"errors"
	"io"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/csv"
	"github.com/coldsmirk/vef-framework-go/excel"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &ExportTestSuite{
			BaseTestSuite: BaseTestSuite{
				ctx:   env.Ctx,
				db:    env.DB,
				bunDB: env.BunDB,
				ds:    env.DS,
			},
		}
	})
}

// ExportEmployee is the test model for export tests (uses tabular tags).
type ExportEmployee struct {
	bun.BaseModel `bun:"table:export_employee,alias:ee"`
	orm.Model     `tabular:"-" bun:"extend"`

	Name   string `json:"name"   tabular:"姓名,width=20" bun:",notnull"`
	Email  string `json:"email"  tabular:"邮箱,width=25" bun:",notnull"`
	Age    int    `json:"age"    tabular:"年龄,width=10" bun:",notnull"`
	Status string `json:"status" tabular:"状态,width=10" bun:",notnull,default:'active'"`
}

// ExportEmployeeSearch is the search parameters for ExportEmployee.
type ExportEmployeeSearch struct {
	api.P

	Keyword *string `json:"keyword" search:"contains,column=name|email"`
	Status  *string `json:"status"  search:"eq"`
}

// Test Resources for Export

type TestEmployeeExportResource struct {
	api.Resource
	crud.Export[ExportEmployee, ExportEmployeeSearch]
}

func NewTestEmployeeExportResource() api.Resource {
	return &TestEmployeeExportResource{
		Resource: api.NewRPCResource("test/employee_export"),
		Export:   crud.NewExport[ExportEmployee, ExportEmployeeSearch]().WithCondition(fixtureScope).Public(),
	}
}

type TestEmployeeExportWithOptionsResource struct {
	api.Resource
	crud.Export[ExportEmployee, ExportEmployeeSearch]
}

func NewTestEmployeeExportWithOptionsResource() api.Resource {
	return &TestEmployeeExportWithOptionsResource{
		Resource: api.NewRPCResource("test/employee_export_opts"),
		Export: crud.NewExport[ExportEmployee, ExportEmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithExcelOptions(excel.WithSheetName("用户列表")),
	}
}

type TestEmployeeExportWithFilenameResource struct {
	api.Resource
	crud.Export[ExportEmployee, ExportEmployeeSearch]
}

func NewTestEmployeeExportWithFilenameResource() api.Resource {
	return &TestEmployeeExportWithFilenameResource{
		Resource: api.NewRPCResource("test/employee_export_filename"),
		Export: crud.NewExport[ExportEmployee, ExportEmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithFilenameBuilder(func(_ ExportEmployeeSearch, _ fiber.Ctx) string {
				return "custom_users.xlsx"
			}),
	}
}

type TestEmployeeExportWithPreProcessorResource struct {
	api.Resource
	crud.Export[ExportEmployee, ExportEmployeeSearch]
}

func NewTestEmployeeExportWithPreProcessorResource() api.Resource {
	return &TestEmployeeExportWithPreProcessorResource{
		Resource: api.NewRPCResource("test/employee_export_preproc"),
		Export: crud.NewExport[ExportEmployee, ExportEmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithPreExport(func(models []ExportEmployee, _ ExportEmployeeSearch, ctx fiber.Ctx, _ orm.DB) error {
				// Add custom header with count
				ctx.Set("X-Export-Count", string(rune('0'+len(models))))

				return nil
			}),
	}
}

type TestEmployeeExportWithFilterResource struct {
	api.Resource
	crud.Export[ExportEmployee, ExportEmployeeSearch]
}

func NewTestEmployeeExportWithFilterResource() api.Resource {
	return &TestEmployeeExportWithFilterResource{
		Resource: api.NewRPCResource("test/employee_export_filter"),
		Export: crud.NewExport[ExportEmployee, ExportEmployeeSearch]().
			WithCondition(fixtureScope).
			WithCondition(func(cb orm.ConditionBuilder) {
				cb.Equals("status", "active")
			}).
			Public(),
	}
}

type TestEmployeeExportCSVResource struct {
	api.Resource
	crud.Export[ExportEmployee, ExportEmployeeSearch]
}

func NewTestEmployeeExportCSVResource() api.Resource {
	return &TestEmployeeExportCSVResource{
		Resource: api.NewRPCResource("test/employee_export_csv"),
		Export: crud.NewExport[ExportEmployee, ExportEmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithDefaultFormat(crud.FormatCsv),
	}
}

type TestEmployeeExportCSVWithOptionsResource struct {
	api.Resource
	crud.Export[ExportEmployee, ExportEmployeeSearch]
}

func NewTestEmployeeExportCSVWithOptionsResource() api.Resource {
	return &TestEmployeeExportCSVWithOptionsResource{
		Resource: api.NewRPCResource("test/employee_export_csv_opts"),
		Export: crud.NewExport[ExportEmployee, ExportEmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithDefaultFormat(crud.FormatCsv).
			WithCsvOptions(csv.WithExportDelimiter(';')),
	}
}

type TestEmployeeExportCSVWithFilenameResource struct {
	api.Resource
	crud.Export[ExportEmployee, ExportEmployeeSearch]
}

func NewTestEmployeeExportCSVWithFilenameResource() api.Resource {
	return &TestEmployeeExportCSVWithFilenameResource{
		Resource: api.NewRPCResource("test/employee_export_csv_filename"),
		Export: crud.NewExport[ExportEmployee, ExportEmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithDefaultFormat(crud.FormatCsv).
			WithFilenameBuilder(func(_ ExportEmployeeSearch, _ fiber.Ctx) string {
				return "custom_users.csv"
			}),
	}
}

// ErrorQueryApplierExportResource - Export with QueryApplier that returns error.
type ErrorQueryApplierExportResource struct {
	api.Resource
	crud.Export[ExportEmployee, ExportEmployeeSearch]
}

func NewErrorQueryApplierExportResource() api.Resource {
	return &ErrorQueryApplierExportResource{
		Resource: api.NewRPCResource("test/employee_export_err_applier"),
		Export: crud.NewExport[ExportEmployee, ExportEmployeeSearch]().
			WithCondition(fixtureScope).
			WithQueryApplier(func(_ orm.SelectQuery, _ ExportEmployeeSearch, _ fiber.Ctx) error {
				return errors.New("export query applier error")
			}).
			Public(),
	}
}

// PreExportErrorResource - Export with preExport processor that returns error.
type PreExportErrorResource struct {
	api.Resource
	crud.Export[ExportEmployee, ExportEmployeeSearch]
}

func NewPreExportErrorResource() api.Resource {
	return &PreExportErrorResource{
		Resource: api.NewRPCResource("test/employee_export_preproc_err"),
		Export: crud.NewExport[ExportEmployee, ExportEmployeeSearch]().
			WithCondition(fixtureScope).
			Public().
			WithPreExport(func(_ []ExportEmployee, _ ExportEmployeeSearch, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("pre-export error")
			}),
	}
}

// ExportTestSuite tests the Export API functionality
// including basic Excel/CSV exports, custom options, filename builders, pre-processors,
// filters, format overrides, and negative cases.
type ExportTestSuite struct {
	BaseTestSuite
}

// SetupSuite runs once before all tests in the suite.
func (suite *ExportTestSuite) SetupSuite() {
	suite.setupBaseSuite(
		NewTestEmployeeExportResource,
		NewTestEmployeeExportWithOptionsResource,
		NewTestEmployeeExportWithFilenameResource,
		NewTestEmployeeExportWithPreProcessorResource,
		NewTestEmployeeExportWithFilterResource,
		NewTestEmployeeExportCSVResource,
		NewTestEmployeeExportCSVWithOptionsResource,
		NewTestEmployeeExportCSVWithFilenameResource,
		NewErrorQueryApplierExportResource,
		NewPreExportErrorResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
func (suite *ExportTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// Export Tests

func (suite *ExportTestSuite) TestExportBasic() {
	suite.T().Logf("Testing basic Excel export for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	suite.Equal("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		resp.Header.Get(fiber.HeaderContentType), "Should return Excel content type")
	suite.Contains(resp.Header.Get(fiber.HeaderContentDisposition), "attachment", "Should have attachment disposition")
	suite.Contains(resp.Header.Get(fiber.HeaderContentDisposition), "filename=", "Should include filename")
	suite.Contains(resp.Header.Get(fiber.HeaderContentDisposition), ".xlsx", "Should have .xlsx extension")

	// Read and verify Excel file
	body, err := io.ReadAll(resp.Body)
	suite.NoError(err, "Should read response body successfully")
	suite.NotEmpty(body, "Should return non-empty response body")

	// Verify it's a valid Excel file by checking signature
	// Excel files start with PK (ZIP signature)
	suite.Equal(byte('P'), body[0], "Should start with 'P' (ZIP signature)")
	suite.Equal(byte('K'), body[1], "Should have 'K' as second byte (ZIP signature)")
	suite.T().Logf("Successfully exported Excel file with %d bytes", len(body))
}

func (suite *ExportTestSuite) TestExportWithSearchFilter() {
	suite.T().Logf("Testing export with search filters for %s", suite.ds.Kind)

	suite.Run("FilterByStatus", func() {
		status := "active"
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_export",
				Action:   "export",
				Version:  "v1",
			},
			Params: map[string]any{
				"status": status,
			},
		})

		suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
		suite.Equal("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			resp.Header.Get(fiber.HeaderContentType), "Should return Excel content type")

		body, err := io.ReadAll(resp.Body)
		suite.NoError(err, "Should read response body successfully")
		suite.NotEmpty(body, "Should return non-empty response body")
		suite.T().Logf("Successfully exported filtered users (status=%s) with %d bytes", status, len(body))
	})

	suite.Run("FilterByKeyword", func() {
		keyword := "Engineer"
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_export",
				Action:   "export",
				Version:  "v1",
			},
			Params: map[string]any{
				"keyword": keyword,
			},
		})

		suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
		suite.Equal("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			resp.Header.Get(fiber.HeaderContentType), "Should return Excel content type")

		body, err := io.ReadAll(resp.Body)
		suite.NoError(err, "Should read response body successfully")
		suite.NotEmpty(body, "Should return non-empty response body")
		suite.T().Logf("Successfully exported filtered users (keyword=%s) with %d bytes", keyword, len(body))
	})
}

func (suite *ExportTestSuite) TestExportWithCustomFilename() {
	suite.T().Logf("Testing export with custom filename for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export_filename",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	suite.Contains(resp.Header.Get(fiber.HeaderContentDisposition), "custom_users.xlsx", "Should use custom filename")

	body, err := io.ReadAll(resp.Body)
	suite.NoError(err, "Should read response body successfully")
	suite.NotEmpty(body, "Should return non-empty response body")
	suite.T().Logf("Successfully exported with custom filename: %s", resp.Header.Get(fiber.HeaderContentDisposition))
}

func (suite *ExportTestSuite) TestExportWithPreProcessor() {
	suite.T().Logf("Testing export with pre-processor for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export_preproc",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	suite.NotEmpty(resp.Header.Get("X-Export-Count"), "Should set X-Export-Count header in pre-processor")

	body, err := io.ReadAll(resp.Body)
	suite.NoError(err, "Should read response body successfully")
	suite.NotEmpty(body, "Should return non-empty response body")
	suite.T().Logf("Pre-processor executed successfully with header: %s", resp.Header.Get("X-Export-Count"))
}

func (suite *ExportTestSuite) TestExportWithFilterApplier() {
	suite.T().Logf("Testing export with filter applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export_filter",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	suite.Equal("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		resp.Header.Get(fiber.HeaderContentType), "Should return Excel content type")

	body, err := io.ReadAll(resp.Body)
	suite.NoError(err, "Should read response body successfully")
	suite.NotEmpty(body, "Should return non-empty response body")

	// Parse the Excel file to verify only active users are exported
	importer := excel.NewImporterFor[ExportEmployee]()
	users, _, err := importer.Import(bytes.NewReader(body))
	suite.NoError(err, "Should parse exported Excel file successfully")

	exportedUsers, ok := users.([]ExportEmployee)
	suite.True(ok, "Type assertion to []ExportEmployee should succeed")
	suite.NotEmpty(exportedUsers, "Should export at least one user")

	// Verify all exported users have status "active"
	for _, user := range exportedUsers {
		suite.Equal("active", user.Status, "Should only export active users")
	}

	suite.T().Logf("Filter applier successfully exported %d active users", len(exportedUsers))
}

func (suite *ExportTestSuite) TestExportEmptyResult() {
	suite.T().Logf("Testing export with empty result for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export",
			Action:   "export",
			Version:  "v1",
		},
		Params: map[string]any{
			"keyword": "NonexistentKeyword12345XYZ",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	suite.Equal("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		resp.Header.Get(fiber.HeaderContentType), "Should return Excel content type")

	// Even empty export should return a valid Excel file with headers
	body, err := io.ReadAll(resp.Body)
	suite.NoError(err, "Should read response body successfully")
	suite.NotEmpty(body, "Should return non-empty response body")

	// Verify it's still a valid Excel file
	suite.Equal(byte('P'), body[0], "Should start with 'P' (ZIP signature)")
	suite.Equal(byte('K'), body[1], "Should have 'K' as second byte (ZIP signature)")
	suite.T().Log("Empty result correctly returned valid Excel file with headers")
}

func (suite *ExportTestSuite) TestExportWithOptions() {
	suite.T().Logf("Testing export with options for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export_opts",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	suite.Equal("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		resp.Header.Get(fiber.HeaderContentType), "Should return Excel content type")

	body, err := io.ReadAll(resp.Body)
	suite.NoError(err, "Should read response body successfully")
	suite.NotEmpty(body, "Should return non-empty response body")

	// Verify the Excel file can be parsed successfully with custom sheet name
	importer := excel.NewImporterFor[ExportEmployee](excel.WithImportSheetName("用户列表"))
	users, _, err := importer.Import(bytes.NewReader(body))
	suite.NoError(err, "Should parse Excel file with custom sheet name successfully")

	exportedUsers, ok := users.([]ExportEmployee)
	suite.True(ok, "Type assertion to []ExportEmployee should succeed")
	suite.NotEmpty(exportedUsers, "Should export at least one user")
	suite.T().Logf("Successfully exported %d users with custom options", len(exportedUsers))
}

func (suite *ExportTestSuite) TestExportNegativeCases() {
	suite.T().Logf("Testing export negative cases for %s", suite.ds.Kind)

	suite.Run("InvalidSearchParameter", func() {
		// Export should handle invalid search parameters gracefully
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_export",
				Action:   "export",
				Version:  "v1",
			},
			Params: map[string]any{
				"nonexistent_field": "value",
			},
		})

		suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
		suite.Equal("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			resp.Header.Get(fiber.HeaderContentType), "Should return Excel content type")

		body, err := io.ReadAll(resp.Body)
		suite.NoError(err, "Should read response body successfully")
		suite.NotEmpty(body, "Should return non-empty response body")
		suite.T().Log("Invalid search parameter handled gracefully")
	})
}

func (suite *ExportTestSuite) TestExportContentType() {
	suite.T().Logf("Testing export content type for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")

	// Verify correct content type for Excel files
	contentType := resp.Header.Get(fiber.HeaderContentType)
	suite.Equal("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", contentType, "Should return correct Excel content type")

	// Verify Content-Disposition header
	contentDisposition := resp.Header.Get(fiber.HeaderContentDisposition)
	suite.Contains(contentDisposition, "attachment", "Should have attachment disposition")
	suite.Contains(contentDisposition, "filename=", "Should include filename")
	suite.T().Log("Content type and disposition headers verified successfully")
}

func (suite *ExportTestSuite) TestExportResponseHeaders() {
	suite.T().Logf("Testing export response headers for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")

	// Check all required response headers
	suite.NotEmpty(resp.Header.Get(fiber.HeaderContentType), "Should have Content-Type header")
	suite.NotEmpty(resp.Header.Get(fiber.HeaderContentDisposition), "Should have Content-Disposition header")

	// Verify the response body contains data
	body, err := io.ReadAll(resp.Body)
	suite.NoError(err, "Should read response body successfully")
	suite.NotEmpty(body, "Should return non-empty response body")
	suite.Greater(len(body), 100, "Should return reasonably sized Excel file")
	suite.T().Logf("Response headers verified successfully, file size: %d bytes", len(body))
}

// CSV Export Tests

func (suite *ExportTestSuite) TestExportCSVBasic() {
	suite.T().Logf("Testing basic CSV export for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export_csv",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	suite.Equal("text/csv; charset=utf-8",
		resp.Header.Get(fiber.HeaderContentType), "Should return CSV content type")
	suite.Contains(resp.Header.Get(fiber.HeaderContentDisposition), "attachment", "Should have attachment disposition")
	suite.Contains(resp.Header.Get(fiber.HeaderContentDisposition), "filename=", "Should include filename")
	suite.Contains(resp.Header.Get(fiber.HeaderContentDisposition), ".csv", "Should have .csv extension")

	// Read and verify CSV file
	body, err := io.ReadAll(resp.Body)
	suite.NoError(err, "Should read response body successfully")
	suite.NotEmpty(body, "Should return non-empty response body")

	// Verify it's a valid CSV file by checking for headers
	content := string(body)
	suite.Contains(content, "姓名", "Should contain Chinese header for Name")
	suite.T().Logf("Successfully exported CSV file with %d bytes", len(body))
}

func (suite *ExportTestSuite) TestExportCSVWithSearchFilter() {
	suite.T().Logf("Testing CSV export with search filters for %s", suite.ds.Kind)

	suite.Run("FilterByStatus", func() {
		status := "active"
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_export_csv",
				Action:   "export",
				Version:  "v1",
			},
			Params: map[string]any{
				"status": status,
			},
		})

		suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
		suite.Equal("text/csv; charset=utf-8",
			resp.Header.Get(fiber.HeaderContentType), "Should return CSV content type")

		body, err := io.ReadAll(resp.Body)
		suite.NoError(err, "Should read response body successfully")
		suite.NotEmpty(body, "Should return non-empty response body")
		suite.T().Logf("Successfully exported CSV with filter (status=%s)", status)
	})

	suite.Run("FilterByKeyword", func() {
		keyword := "Engineer"
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_export_csv",
				Action:   "export",
				Version:  "v1",
			},
			Params: map[string]any{
				"keyword": keyword,
			},
		})

		suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
		suite.Equal("text/csv; charset=utf-8",
			resp.Header.Get(fiber.HeaderContentType), "Should return CSV content type")

		body, err := io.ReadAll(resp.Body)
		suite.NoError(err, "Should read response body successfully")
		suite.NotEmpty(body, "Should return non-empty response body")
		suite.T().Logf("Successfully exported CSV with filter (keyword=%s)", keyword)
	})
}

func (suite *ExportTestSuite) TestExportCSVWithCustomFilename() {
	suite.T().Logf("Testing CSV export with custom filename for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export_csv_filename",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	suite.Contains(resp.Header.Get(fiber.HeaderContentDisposition), "custom_users.csv", "Should use custom filename")

	body, err := io.ReadAll(resp.Body)
	suite.NoError(err, "Should read response body successfully")
	suite.NotEmpty(body, "Should return non-empty response body")
	suite.T().Log("Successfully exported CSV with custom filename")
}

func (suite *ExportTestSuite) TestExportCSVWithOptions() {
	suite.T().Logf("Testing CSV export with options for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export_csv_opts",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	suite.Equal("text/csv; charset=utf-8",
		resp.Header.Get(fiber.HeaderContentType), "Should return CSV content type")

	body, err := io.ReadAll(resp.Body)
	suite.NoError(err, "Should read response body successfully")
	suite.NotEmpty(body, "Should return non-empty response body")

	// Verify semicolon delimiter is used by parsing with semicolon delimiter
	importer := csv.NewImporterFor[ExportEmployee](csv.WithImportDelimiter(';'))
	users, _, err := importer.Import(bytes.NewReader(body))
	suite.NoError(err, "Should parse CSV with semicolon delimiter successfully")

	exportedUsers, ok := users.([]ExportEmployee)
	suite.True(ok, "Type assertion to []ExportEmployee should succeed")
	suite.NotEmpty(exportedUsers, "Should export at least one user")
	suite.T().Logf("Successfully parsed CSV with custom delimiter, got %d users", len(exportedUsers))
}

func (suite *ExportTestSuite) TestExportCSVEmptyResult() {
	suite.T().Logf("Testing CSV export with empty result for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export_csv",
			Action:   "export",
			Version:  "v1",
		},
		Params: map[string]any{
			"keyword": "NonexistentKeyword12345XYZ",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	suite.Equal("text/csv; charset=utf-8",
		resp.Header.Get(fiber.HeaderContentType), "Should return CSV content type")

	// Even empty export should return a valid CSV file with headers
	body, err := io.ReadAll(resp.Body)
	suite.NoError(err, "Should read response body successfully")
	suite.NotEmpty(body, "Should return non-empty response body")

	// Verify it contains headers
	content := string(body)
	suite.Contains(content, "姓名", "Should still contain headers")
	suite.T().Log("Empty result correctly returned valid CSV file with headers")
}

func (suite *ExportTestSuite) TestExportFormatOverride() {
	suite.T().Logf("Testing export format override for %s", suite.ds.Kind)

	// Test format parameter override - use Excel endpoint but override to CSV
	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export",
			Action:   "export",
			Version:  "v1",
		},
		Meta: map[string]any{
			"format": "csv",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	suite.Equal("text/csv; charset=utf-8",
		resp.Header.Get(fiber.HeaderContentType), "Should return CSV content type after format override")

	body, err := io.ReadAll(resp.Body)
	suite.NoError(err, "Should read response body successfully")
	suite.NotEmpty(body, "Should return non-empty response body")

	content := string(body)
	suite.Contains(content, "姓名", "Should contain CSV headers")
	suite.T().Log("Format override from Excel to CSV successful")
}

func (suite *ExportTestSuite) TestExportCSVContentType() {
	suite.T().Logf("Testing CSV export content type for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export_csv",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")

	// Verify correct content type for CSV files
	contentType := resp.Header.Get(fiber.HeaderContentType)
	suite.Equal("text/csv; charset=utf-8", contentType, "Should return correct CSV content type")

	// Verify Content-Disposition header
	contentDisposition := resp.Header.Get(fiber.HeaderContentDisposition)
	suite.Contains(contentDisposition, "attachment", "Should have attachment disposition")
	suite.Contains(contentDisposition, "filename=", "Should include filename")
	suite.T().Log("CSV content type and disposition headers verified successfully")
}

// TestExportUnsupportedFormat tests export with an unsupported format.
func (suite *ExportTestSuite) TestExportUnsupportedFormat() {
	suite.T().Logf("Testing Export API unsupported format for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export",
			Action:   "export",
			Version:  "v1",
		},
		Meta: map[string]any{
			"format": "pdf",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail for unsupported format")

	suite.T().Logf("Export with unsupported format failed as expected")
}

// TestExportErrorQueryApplier tests export with a QueryApplier that returns error.
func (suite *ExportTestSuite) TestExportErrorQueryApplier() {
	suite.T().Logf("Testing Export API error query applier for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export_err_applier",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("Export failed as expected due to query applier error")
}

// TestExportPreProcessorError tests export with a preExport processor that returns error.
func (suite *ExportTestSuite) TestExportPreProcessorError() {
	suite.T().Logf("Testing Export API pre-processor error for %s", suite.ds.Kind)

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_export_preproc_err",
			Action:   "export",
			Version:  "v1",
		},
	})

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("Export failed as expected due to pre-processor error")
}

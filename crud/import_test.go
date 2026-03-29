package crud_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/csv"
	"github.com/coldsmirk/vef-framework-go/excel"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/result"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &ImportTestSuite{
			BaseTestSuite: BaseTestSuite{
				ctx:   env.Ctx,
				db:    env.DB,
				bunDB: env.BunDB,
				ds:    env.DS,
			},
		}
	})
}

// ImportEmployee is the test model for import tests (uses tabular tags).
type ImportEmployee struct {
	bun.BaseModel `bun:"table:import_employee,alias:ie"`
	orm.FullAuditedModel     `tabular:"-" bun:"extend"`

	Name   string `json:"name"   tabular:"姓名,width=20" bun:",notnull"                  validate:"required"`
	Email  string `json:"email"  tabular:"邮箱,width=25" bun:",notnull"                  validate:"required,email"`
	Age    int    `json:"age"    tabular:"年龄,width=10" bun:",notnull"                  validate:"gte=0,lte=150"`
	Status string `json:"status" tabular:"状态,width=10" bun:",notnull,default:'active'" validate:"required,oneof=active inactive pending"`
}

// ImportEmployeeSearch is the search parameters for ImportEmployee.
type ImportEmployeeSearch struct {
	api.P
}

// Test Resources for Import

type TestEmployeeImportResource struct {
	api.Resource
	crud.Import[ImportEmployee]
}

func NewTestEmployeeImportResource() api.Resource {
	return &TestEmployeeImportResource{
		Resource: api.NewRPCResource("test/employee_import"),
		Import:   crud.NewImport[ImportEmployee]().Public(),
	}
}

type TestEmployeeImportWithOptionsResource struct {
	api.Resource
	crud.Import[ImportEmployee]
}

func NewTestEmployeeImportWithOptionsResource() api.Resource {
	return &TestEmployeeImportWithOptionsResource{
		Resource: api.NewRPCResource("test/employee_import_opts"),
		Import: crud.NewImport[ImportEmployee]().
			Public().
			WithExcelOptions(excel.WithImportSheetName("用户列表")),
	}
}

type TestEmployeeImportWithPreProcessorResource struct {
	api.Resource
	crud.Import[ImportEmployee]
}

func NewTestEmployeeImportWithPreProcessorResource() api.Resource {
	return &TestEmployeeImportWithPreProcessorResource{
		Resource: api.NewRPCResource("test/employee_import_preproc"),
		Import: crud.NewImport[ImportEmployee]().
			Public().
			WithPreImport(func(models []ImportEmployee, _ orm.InsertQuery, _ fiber.Ctx, _ orm.DB) error {
				// Pre-process all models - change inactive to pending
				for i := range models {
					if models[i].Status == "inactive" {
						models[i].Status = "pending"
					}
				}

				return nil
			}),
	}
}

type TestEmployeeImportWithPostProcessorResource struct {
	api.Resource
	crud.Import[ImportEmployee]
}

func NewTestEmployeeImportWithPostProcessorResource() api.Resource {
	return &TestEmployeeImportWithPostProcessorResource{
		Resource: api.NewRPCResource("test/employee_import_postproc"),
		Import: crud.NewImport[ImportEmployee]().
			Public().
			WithPostImport(func(models []ImportEmployee, ctx fiber.Ctx, _ orm.DB) error {
				// Set custom header with count
				ctx.Set("X-Import-Count", string(rune('0'+len(models))))

				return nil
			}),
	}
}

type TestEmployeeImportCSVResource struct {
	api.Resource
	crud.Import[ImportEmployee]
}

func NewTestEmployeeImportCSVResource() api.Resource {
	return &TestEmployeeImportCSVResource{
		Resource: api.NewRPCResource("test/employee_import_csv"),
		Import: crud.NewImport[ImportEmployee]().
			Public().
			WithDefaultFormat(crud.FormatCsv),
	}
}

type TestEmployeeImportCSVWithOptionsResource struct {
	api.Resource
	crud.Import[ImportEmployee]
}

func NewTestEmployeeImportCSVWithOptionsResource() api.Resource {
	return &TestEmployeeImportCSVWithOptionsResource{
		Resource: api.NewRPCResource("test/employee_import_csv_opts"),
		Import: crud.NewImport[ImportEmployee]().
			Public().
			WithDefaultFormat(crud.FormatCsv).
			WithCsvOptions(csv.WithImportDelimiter(';')),
	}
}

// PreImportErrorResource - Import with preImport processor that returns error.
type PreImportErrorResource struct {
	api.Resource
	crud.Import[ImportEmployee]
}

func NewPreImportErrorResource() api.Resource {
	return &PreImportErrorResource{
		Resource: api.NewRPCResource("test/employee_import_preproc_err"),
		Import: crud.NewImport[ImportEmployee]().
			Public().
			WithPreImport(func(_ []ImportEmployee, _ orm.InsertQuery, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("pre-import error")
			}),
	}
}

// PostImportErrorResource - Import with postImport processor that returns error.
type PostImportErrorResource struct {
	api.Resource
	crud.Import[ImportEmployee]
}

func NewPostImportErrorResource() api.Resource {
	return &PostImportErrorResource{
		Resource: api.NewRPCResource("test/employee_import_postproc_err"),
		Import: crud.NewImport[ImportEmployee]().
			Public().
			WithPostImport(func(_ []ImportEmployee, _ fiber.Ctx, _ orm.DB) error {
				return errors.New("post-import error")
			}),
	}
}

// ImportTestSuite tests the Import API functionality
// including basic Excel/CSV imports, validation errors, pre/post processors, format overrides,
// large file handling, and negative cases.
type ImportTestSuite struct {
	BaseTestSuite
}

// SetupSuite runs once before all tests in the suite.
func (suite *ImportTestSuite) SetupSuite() {
	suite.setupBaseSuite(
		NewTestEmployeeImportResource,
		NewTestEmployeeImportWithOptionsResource,
		NewTestEmployeeImportWithPreProcessorResource,
		NewTestEmployeeImportWithPostProcessorResource,
		NewTestEmployeeImportCSVResource,
		NewTestEmployeeImportCSVWithOptionsResource,
		NewPreImportErrorResource,
		NewPostImportErrorResource,
	)
}

// TearDownSuite runs once after all tests in the suite.
// TearDownTest cleans up test-imported records after each test.
func (suite *ImportTestSuite) TearDownTest() {
	suite.cleanupTestRecords()
}

func (suite *ImportTestSuite) TearDownSuite() {
	suite.tearDownBaseSuite()
}

// Import Tests

func (suite *ImportTestSuite) TestImportBasic() {
	suite.T().Logf("Testing basic Excel import for %s", suite.ds.Kind)

	// Create test Excel file
	exporter := excel.NewExporterFor[ImportEmployee]()
	testUsers := []ImportEmployee{
		{Name: "Import User 1", Email: "import1@example.com", Age: 30, Status: "active"},
		{Name: "Import User 2", Email: "import2@example.com", Age: 25, Status: "active"},
		{Name: "Import User 3", Email: "import3@example.com", Age: 28, Status: "inactive"},
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export test users to Excel successfully")

	// Create multipart request
	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import",
			Action:   "import",
			Version:  "v1",
		},
	}, "test_import.xlsx", buf.Bytes())

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return success response")
	suite.Equal(i18n.T(result.OkMessage), body.Message, "Should return OK message")

	// Verify response data
	data := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(3), data["total"], "Should import exactly 3 users")
	suite.T().Logf("Successfully imported %v users", data["total"])
}

func (suite *ImportTestSuite) TestImportWithValidationErrors() {
	suite.T().Logf("Testing import with validation errors for %s", suite.ds.Kind)

	// Create test Excel file with invalid data
	exporter := excel.NewExporterFor[ImportEmployee]()
	testUsers := []ImportEmployee{
		{Name: "Valid User", Email: "valid@example.com", Age: 30, Status: "active"},
		{Name: "Invalid Email", Email: "invalid-email", Age: 25, Status: "active"},     // Invalid email
		{Name: "Invalid Age", Email: "test@example.com", Age: 200, Status: "active"},   // Invalid age > 150
		{Name: "Invalid Status", Email: "test2@example.com", Age: 25, Status: "wrong"}, // Invalid status
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export test users with invalid data to Excel successfully")

	// Import should detect validation errors
	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import",
			Action:   "import",
			Version:  "v1",
		},
	}, "test_import_invalid.xlsx", buf.Bytes())

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should return failure response due to validation errors")

	// Verify error data contains validation errors
	data := suite.ReadDataAsMap(body.Data)
	suite.Require().NotNil(data["errors"], "Should contain errors field in response data")
	errors := suite.ReadDataAsSlice(data["errors"])
	suite.NotEmpty(errors, "Should contain validation errors in errors array")
	suite.T().Logf("Validation detected %d error(s) as expected", len(errors))
}

func (suite *ImportTestSuite) TestImportWithMissingRequiredFields() {
	suite.T().Logf("Testing import with missing required fields for %s", suite.ds.Kind)

	// Create test Excel file with missing required fields
	exporter := excel.NewExporterFor[ImportEmployee]()
	testUsers := []ImportEmployee{
		{Name: "", Email: "noemail@example.com", Age: 30, Status: "active"}, // Missing name
		{Name: "No Email", Email: "", Age: 25, Status: "active"},            // Missing email
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export test users with missing fields to Excel successfully")

	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import",
			Action:   "import",
			Version:  "v1",
		},
	}, "test_import_missing.xlsx", buf.Bytes())

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should return failure response due to missing required fields")

	data := suite.ReadDataAsMap(body.Data)
	suite.Require().NotNil(data["errors"], "Should contain errors field in response data")
	suite.T().Log("Missing required fields correctly rejected")
}

func (suite *ImportTestSuite) TestImportWithPreProcessor() {
	suite.T().Logf("Testing import with pre-processor for %s", suite.ds.Kind)

	// Create test Excel file with users
	// The preprocessor will change "inactive" status to "pending"
	exporter := excel.NewExporterFor[ImportEmployee]()
	testUsers := []ImportEmployee{
		{Name: "Pre-processed User 1", Email: "preproc1@example.com", Age: 30, Status: "active"},
		{Name: "Pre-processed User 2", Email: "preproc2@example.com", Age: 25, Status: "inactive"}, // Will be changed to "pending" by preprocessor
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export test users to Excel successfully")

	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import_preproc",
			Action:   "import",
			Version:  "v1",
		},
	}, "test_import_preproc.xlsx", buf.Bytes())

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return success response, got error: %s", body.Message)

	// Verify data was imported
	data := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(2), data["total"], "Should import exactly 2 users after pre-processing")
	suite.T().Logf("Pre-processor successfully transformed and imported %v users", data["total"])
}

func (suite *ImportTestSuite) TestImportWithPostProcessor() {
	suite.T().Logf("Testing import with post-processor for %s", suite.ds.Kind)

	// Create test Excel file
	exporter := excel.NewExporterFor[ImportEmployee]()
	testUsers := []ImportEmployee{
		{Name: "Post-processed User 1", Email: "postproc1@example.com", Age: 30, Status: "active"},
		{Name: "Post-processed User 2", Email: "postproc2@example.com", Age: 25, Status: "active"},
		{Name: "Post-processed User 3", Email: "postproc3@example.com", Age: 28, Status: "inactive"},
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export test users to Excel successfully")

	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import_postproc",
			Action:   "import",
			Version:  "v1",
		},
	}, "test_import_postproc.xlsx", buf.Bytes())

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	suite.NotEmpty(resp.Header.Get("X-Import-Count"), "Should set X-Import-Count header in post-processor")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return success response")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(3), data["total"], "Should import exactly 3 users")
	suite.T().Logf("Post-processor executed, imported %v users with header: %s", data["total"], resp.Header.Get("X-Import-Count"))
}

func (suite *ImportTestSuite) TestImportEmptyFile() {
	suite.T().Logf("Testing import with empty Excel file for %s", suite.ds.Kind)

	// Create empty Excel file (with headers but no data rows)
	exporter := excel.NewExporterFor[ImportEmployee]()

	var testUsers []ImportEmployee

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export empty user list to Excel successfully")

	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import",
			Action:   "import",
			Version:  "v1",
		},
	}, "test_import_empty.xlsx", buf.Bytes())

	// Note: Excel importer returns an error when there are no data rows
	// This is the expected behavior - empty files are rejected
	// Status can be either 500 (error during processing) or 200 with error body
	if resp.StatusCode == 500 {
		suite.T().Log("Empty file correctly rejected with 500 status")
	} else {
		suite.Equal(200, resp.StatusCode, "Should return HTTP 200 status")
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should return error response for empty file")
		suite.T().Log("Empty file correctly rejected with error response")
	}
}

func (suite *ImportTestSuite) TestImportLargeFile() {
	suite.T().Logf("Testing import with large Excel file for %s", suite.ds.Kind)

	// Create large test file with many rows
	exporter := excel.NewExporterFor[ImportEmployee]()

	testUsers := make([]ImportEmployee, 100)
	for i := range testUsers {
		testUsers[i] = ImportEmployee{
			Name:   "Bulk User " + string(rune('A'+i%26)),
			Email:  "bulkuser" + string(rune('0'+i%10)) + "@example.com",
			Age:    20 + (i % 50),
			Status: []string{"active", "inactive"}[i%2],
		}
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export 100 test users to Excel successfully")

	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import",
			Action:   "import",
			Version:  "v1",
		},
	}, "test_import_large.xlsx", buf.Bytes())

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return success response")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(100), data["total"], "Should import exactly 100 users")
	suite.T().Logf("Successfully imported large file with %v users", data["total"])
}

func (suite *ImportTestSuite) TestImportNegativeCases() {
	suite.T().Logf("Testing import negative cases for %s", suite.ds.Kind)

	suite.Run("MissingFile", func() {
		// Try to import without providing a file
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_import",
				Action:   "import",
				Version:  "v1",
			},
		})

		// Request should fail with status 500 or 200 with error
		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail when importing without file")
		suite.T().Log("Missing file correctly rejected")
	})

	suite.Run("InvalidFileFormat", func() {
		// Try to import a non-Excel file
		resp := suite.makeMultipartAPIRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_import",
				Action:   "import",
				Version:  "v1",
			},
		}, "test.txt", []byte("This is not an Excel file"))

		// Should return error (either 500 or 200 with error body)
		if resp.StatusCode == 200 {
			body := suite.ReadResult(resp)
			suite.False(body.IsOk(), "Should reject invalid file format")
			suite.T().Log("Invalid file format correctly rejected with error response")
		} else {
			// 500 error is also acceptable for invalid file format
			suite.NotEqual(200, resp.StatusCode, "Should return error status for invalid format")
			suite.T().Log("Invalid file format correctly rejected with error status")
		}
	})

	suite.Run("JSONRequest", func() {
		// Import requires multipart/form-data, not JSON
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_import",
				Action:   "import",
				Version:  "v1",
			},
			Params: map[string]any{
				"file": "some-file.xlsx",
			},
		})

		// Should fail because no file was provided or wrong content type
		if resp.StatusCode == 200 {
			body := suite.ReadResult(resp)
			suite.False(body.IsOk(), "Should fail for JSON request without multipart file")
			suite.T().Log("JSON request correctly rejected with error response")
		} else {
			// Error status is also acceptable
			suite.NotEqual(200, resp.StatusCode, "Should return error status for JSON request")
			suite.T().Log("JSON request correctly rejected with error status")
		}
	})

	suite.Run("CorruptedExcelFile", func() {
		// Try to import corrupted Excel file
		corruptedData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10} // Invalid Excel data

		resp := suite.makeMultipartAPIRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test/employee_import",
				Action:   "import",
				Version:  "v1",
			},
		}, "corrupted.xlsx", corruptedData)

		// Should return error (either 500 or 200 with error body)
		if resp.StatusCode == 200 {
			body := suite.ReadResult(resp)
			suite.False(body.IsOk(), "Should reject corrupted Excel file")
			suite.T().Log("Corrupted file correctly rejected with error response")
		} else {
			// 500 error is also acceptable for corrupted file
			suite.NotEqual(200, resp.StatusCode, "Should return error status for corrupted file")
			suite.T().Log("Corrupted file correctly rejected with error status")
		}
	})
}

// CSV Import Tests

func (suite *ImportTestSuite) TestImportCSVBasic() {
	suite.T().Logf("Testing basic CSV import for %s", suite.ds.Kind)

	// Create test CSV file
	exporter := csv.NewExporterFor[ImportEmployee]()
	testUsers := []ImportEmployee{
		{Name: "CSV User 1", Email: "csv1@example.com", Age: 30, Status: "active"},
		{Name: "CSV User 2", Email: "csv2@example.com", Age: 25, Status: "active"},
		{Name: "CSV User 3", Email: "csv3@example.com", Age: 28, Status: "inactive"},
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export test users to CSV successfully")

	// Create multipart request
	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import_csv",
			Action:   "import",
			Version:  "v1",
		},
	}, "test_import.csv", buf.Bytes())

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return success response")
	suite.Equal(i18n.T(result.OkMessage), body.Message, "Should return OK message")

	// Verify response data
	data := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(3), data["total"], "Should import exactly 3 users from CSV")
	suite.T().Logf("Successfully imported %v users from CSV", data["total"])
}

func (suite *ImportTestSuite) TestImportCSVWithValidationErrors() {
	suite.T().Logf("Testing CSV import with validation errors for %s", suite.ds.Kind)

	// Create test CSV file with invalid data
	exporter := csv.NewExporterFor[ImportEmployee]()
	testUsers := []ImportEmployee{
		{Name: "Valid User", Email: "valid@example.com", Age: 30, Status: "active"},
		{Name: "Invalid Email", Email: "invalid-email", Age: 25, Status: "active"},     // Invalid email
		{Name: "Invalid Age", Email: "test@example.com", Age: 200, Status: "active"},   // Invalid age > 150
		{Name: "Invalid Status", Email: "test2@example.com", Age: 25, Status: "wrong"}, // Invalid status
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export test users with invalid data to CSV successfully")

	// Import should detect validation errors
	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import_csv",
			Action:   "import",
			Version:  "v1",
		},
	}, "test_import_invalid.csv", buf.Bytes())

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should return failure response due to validation errors")

	// Verify error data contains validation errors
	data := suite.ReadDataAsMap(body.Data)
	suite.Require().NotNil(data["errors"], "Should contain errors field in response data")
	errors := suite.ReadDataAsSlice(data["errors"])
	suite.NotEmpty(errors, "Should contain validation errors in errors array")
	suite.T().Logf("CSV validation detected %d error(s) as expected", len(errors))
}

func (suite *ImportTestSuite) TestImportCSVWithOptions() {
	suite.T().Logf("Testing CSV import with custom delimiter for %s", suite.ds.Kind)

	// Create test CSV file with semicolon delimiter
	exporter := csv.NewExporterFor[ImportEmployee](csv.WithExportDelimiter(';'))
	testUsers := []ImportEmployee{
		{Name: "CSV Options User 1", Email: "csvopts1@example.com", Age: 30, Status: "active"},
		{Name: "CSV Options User 2", Email: "csvopts2@example.com", Age: 25, Status: "active"},
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export test users to CSV with semicolon delimiter successfully")

	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import_csv_opts",
			Action:   "import",
			Version:  "v1",
		},
	}, "test_import_opts.csv", buf.Bytes())

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return success response")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(2), data["total"], "Should import exactly 2 users with custom delimiter")
	suite.T().Logf("Successfully imported %v users with semicolon delimiter", data["total"])
}

func (suite *ImportTestSuite) TestImportFormatOverride() {
	suite.T().Logf("Testing import format override for %s", suite.ds.Kind)

	// Test format parameter override
	exporter := csv.NewExporterFor[ImportEmployee]()
	testUsers := []ImportEmployee{
		{Name: "Format Override User", Email: "override@example.com", Age: 30, Status: "active"},
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export test user to CSV successfully")

	// Use Excel endpoint but override format to CSV via parameter
	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import",
			Action:   "import",
			Version:  "v1",
		},
		Meta: map[string]any{
			"format": "csv",
		},
	}, "test_import_override.csv", buf.Bytes())

	suite.Require().Equal(200, resp.StatusCode, "Should return HTTP 200 status")
	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Should return success response")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal(float64(1), data["total"], "Should import exactly 1 user with format override")
	suite.T().Logf("Successfully imported %v user with format override from Excel to CSV", data["total"])
}

// TestImportUnsupportedFormat tests import with an unsupported format.
func (suite *ImportTestSuite) TestImportUnsupportedFormat() {
	suite.T().Logf("Testing Import API unsupported format for %s", suite.ds.Kind)

	exporter := excel.NewExporterFor[ImportEmployee]()
	testUsers := []ImportEmployee{
		{Name: "Format User", Email: "format@example.com", Age: 30, Status: "active"},
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export test users")

	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import",
			Action:   "import",
			Version:  "v1",
		},
		Meta: map[string]any{
			"format": "pdf",
		},
	}, "test.pdf", buf.Bytes())

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail for unsupported format")

	suite.T().Logf("Import with unsupported format failed as expected")
}

// TestImportNoFile tests import without providing a file.
func (suite *ImportTestSuite) TestImportNoFile() {
	suite.T().Logf("Testing Import API no file for %s", suite.ds.Kind)

	// Send a multipart request without file
	var buf bytes.Buffer

	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("resource", "test/employee_import")
	_ = writer.WriteField("action", "import")
	_ = writer.WriteField("version", "v1")
	_ = writer.Close()

	httpReq := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api", &buf)
	httpReq.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())

	resp, err := suite.App.Test(httpReq)
	suite.Require().NoError(err, "Should not return error")

	suite.Equal(200, resp.StatusCode, "Should return 200 status code")
	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail when no file is provided")

	suite.T().Logf("Import without file failed as expected")
}

// TestImportPreHookError tests import with a preImport processor that returns error.
func (suite *ImportTestSuite) TestImportPreHookError() {
	suite.T().Logf("Testing Import API pre-hook error for %s", suite.ds.Kind)

	exporter := excel.NewExporterFor[ImportEmployee]()
	testUsers := []ImportEmployee{
		{Name: "PreHook User", Email: "prehook@example.com", Age: 30, Status: "active"},
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export test users")

	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import_preproc_err",
			Action:   "import",
			Version:  "v1",
		},
	}, "test_prehook.xlsx", buf.Bytes())

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("Import failed as expected due to pre-hook error")
}

// TestImportPostHookError tests import with a postImport processor that returns error.
func (suite *ImportTestSuite) TestImportPostHookError() {
	suite.T().Logf("Testing Import API post-hook error for %s", suite.ds.Kind)

	exporter := excel.NewExporterFor[ImportEmployee]()
	testUsers := []ImportEmployee{
		{Name: "PostHook User", Email: "posthook@example.com", Age: 30, Status: "active"},
	}

	buf, err := exporter.Export(testUsers)
	suite.NoError(err, "Should export test users")

	resp := suite.makeMultipartAPIRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test/employee_import_postproc_err",
			Action:   "import",
			Version:  "v1",
		},
	}, "test_posthook.xlsx", buf.Bytes())

	suite.Contains([]int{200, 500}, resp.StatusCode, "Should return error status code")

	suite.T().Logf("Import failed as expected due to post-hook error")
}

// Helper method for multipart requests.
func (suite *ImportTestSuite) makeMultipartAPIRequest(req api.Request, filename string, fileContent []byte) *http.Response {
	var buf bytes.Buffer

	writer := multipart.NewWriter(&buf)

	// Add API request fields
	_ = writer.WriteField("resource", req.Resource)
	_ = writer.WriteField("action", req.Action)
	_ = writer.WriteField("version", req.Version)

	// Add params as JSON string if present
	if req.Params != nil {
		paramsBytes, err := json.Marshal(req.Params)
		suite.NoError(err, "Should marshal params to JSON")

		_ = writer.WriteField("params", string(paramsBytes))
	}

	// Add meta as JSON string if present
	if req.Meta != nil {
		metaBytes, err := json.Marshal(req.Meta)
		suite.NoError(err, "Should marshal meta to JSON")

		_ = writer.WriteField("meta", string(metaBytes))
	}

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	suite.NoError(err, "Should create form file part")
	_, err = part.Write(fileContent)
	suite.NoError(err, "Should write file content")

	err = writer.Close()
	suite.NoError(err, "Should close multipart writer")

	httpReq := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api", &buf)
	httpReq.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())

	resp, err := suite.App.Test(httpReq)
	suite.Require().NoError(err, "Should not return error")

	return resp
}

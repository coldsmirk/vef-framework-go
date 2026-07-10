package resource_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	vef "github.com/coldsmirk/vef-framework-go"
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// stubMarkerField is the distinctive field list StubFormSchemaParser derives
// from every schema. The built-in vef-framework-react parser would project the
// deployed schema to a "reason" field instead, so finding this list persisted in
// form_fields proves the composed graph consumed the host override, not the
// framework default.
var stubMarkerField = []approval.FormFieldDefinition{
	{Key: "stub_marker", Kind: approval.FieldInput, Label: "Stub Marker", SortOrder: 0},
}

// StubFormSchemaParser is a host approval.FormSchemaParser that ignores the
// submitted schema and always derives stubMarkerField.
type StubFormSchemaParser struct{}

func (*StubFormSchemaParser) ParseFormFields(context.Context, json.RawMessage) ([]approval.FormFieldDefinition, error) {
	return stubMarkerField, nil
}

// FormSchemaParserOverrideTestSuite proves vef.ProvideApprovalFormSchemaParser
// replaces the built-in parser in a composed app graph: a deploy driven through
// the module's RPC command path must persist the stub parser's derived fields.
type FormSchemaParserOverrideTestSuite struct {
	apptest.Suite

	ctx        context.Context
	db         orm.DB
	token      string
	categoryID string
}

func TestFormSchemaParserOverride(t *testing.T) {
	suite.Run(t, new(FormSchemaParserOverrideTestSuite))
}

func (s *FormSchemaParserOverrideTestSuite) SetupSuite() {
	s.ctx = context.Background()

	// Override the framework's default approval.FormSchemaParser with the stub
	// through the real public helper — the graph under test.
	s.db, s.token = setupResourceApp(&s.Suite,
		vef.ProvideApprovalFormSchemaParser(func() approval.FormSchemaParser { return new(StubFormSchemaParser) }),
	)

	cat := &approval.FlowCategory{
		TenantID: "default",
		Code:     "parser-override-cat",
		Name:     "Parser Override Category",
		IsActive: true,
	}
	_, err := s.db.NewInsert().Model(cat).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")
	s.categoryID = cat.ID
}

func (s *FormSchemaParserOverrideTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
	s.TearDownApp()
}

func (s *FormSchemaParserOverrideTestSuite) createFlow(code, name string) string {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/flow", Action: "create", Version: "v1"},
		Params: map[string]any{
			"tenantId":               "default",
			"code":                   code,
			"name":                   name,
			"categoryId":             s.categoryID,
			"bindingMode":            "standalone",
			"isAllInitiationAllowed": true,
			"instanceTitleTemplate":  name + " {{.instanceNo}}",
		},
	}, s.token)

	s.Require().Equal(http.StatusOK, resp.StatusCode, "Flow create RPC should return HTTP 200")
	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should create flow")

	data := s.ReadDataAsMap(res.Data)
	flowID, ok := data["id"].(string)
	s.Require().True(ok, "Flow ID should be a string")

	return flowID
}

func (s *FormSchemaParserOverrideTestSuite) TestDeployConsumesOverriddenParser() {
	flowID := s.createFlow("parser-override-flow", "Parser Override")

	// A schema the BUILT-IN parser would project to a "reason" field — so the
	// persisted fields being stubMarkerField can only mean the stub ran.
	schema := json.RawMessage(`{"version":2,"presentations":{"pc":{"children":[` +
		`{"id":"F1","type":"textfield","key":"reason","label":"Reason"}]}}}`)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/flow", Action: "deploy", Version: "v1"},
		Params: map[string]any{
			"flowId":         flowID,
			"flowDefinition": toMap(simpleFlowDef()),
			"formSchema":     schema,
		},
	}, s.token)

	s.Require().Equal(http.StatusOK, resp.StatusCode, "Deploy RPC should return HTTP 200")
	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should deploy flow")

	data := s.ReadDataAsMap(res.Data)
	versionID, ok := data["id"].(string)
	s.Require().True(ok, "Version ID should be a string")

	var version approval.FlowVersion

	version.ID = versionID
	s.Require().NoError(s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx), "Should load the deployed version")

	// form_fields must be exactly what the stub derived — not the built-in
	// parser's "reason" projection.
	s.Require().Equal(stubMarkerField, version.FormFields,
		"Deploy must persist the overridden parser's fields, proving vef.ProvideApprovalFormSchemaParser replaced the built-in parser")

	// The host schema is still stored verbatim regardless of which parser derives
	// the fields.
	s.Assert().JSONEq(string(schema), string(version.FormSchema), "form_schema should persist the designer document verbatim")
}

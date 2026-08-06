package storage_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// PermissiveFileACL stands in for a business module's ACL: it grants
// every read, which is what an application that actually serves private
// files must supply.
type PermissiveFileACL struct{}

func (*PermissiveFileACL) CanRead(context.Context, *security.Principal, string) (bool, error) {
	return true, nil
}

// FileResolveTestSuite exercises sys/storage/file.resolve — the lookup a
// client uses to render a stored object key as its original filename.
type FileResolveTestSuite struct {
	apptest.Suite

	ctx        context.Context
	ownerToken string

	// permissive swaps in an ACL that grants every read, so one suite can
	// cover both the fail-closed default and a configured application.
	permissive bool
}

func (s *FileResolveTestSuite) SetupSuite() {
	s.ctx = context.Background()

	opts := []fx.Option{
		fx.Replace(
			&config.StorageConfig{
				Provider:           config.StorageMemory,
				AutoMigrate:        true,
				MaxUploadSize:      testMaxUploadSize,
				AllowPublicUploads: true,
			},
			&security.JWTConfig{
				Secret:   security.DefaultJWTSecret,
				Audience: "test_app",
			},
		),
	}

	if s.permissive {
		opts = append(opts, fx.Decorate(func(storage.FileACL) storage.FileACL {
			return new(PermissiveFileACL)
		}))
	}

	s.SetupApp(opts...)

	s.ownerToken = s.GenerateToken(&security.Principal{ID: "test-owner", Name: "owner"})
}

func (s *FileResolveTestSuite) TearDownSuite() {
	s.TearDownApp()
}

// uploadFile drives a complete single-part upload and returns the key.
func (s *FileResolveTestSuite) uploadFile(filename string, public bool) string {
	s.T().Helper()

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "init_upload", Version: "v1"},
		Params: map[string]any{
			"filename": filename,
			"size":     singleShotSize,
			"public":   public,
		},
	}, s.ownerToken)

	body := s.ReadResult(resp)
	s.Require().True(body.IsOk(), "Init upload should succeed: %s", body.Message)

	data := s.ReadDataAsMap(body.Data)
	claimID, _ := data["claimId"].(string)
	key, _ := data["key"].(string)

	paramsJSON, err := json.Marshal(map[string]any{"claimId": claimID, "partNumber": 1})
	s.Require().NoError(err, "Upload part params should marshal")

	form := &bytes.Buffer{}
	writer := multipart.NewWriter(form)

	for field, value := range map[string]string{
		"resource": "sys/storage",
		"action":   "upload_part",
		"version":  "v1",
		"params":   string(paramsJSON),
	} {
		s.Require().NoError(writer.WriteField(field, value), "Multipart request should write form field %s", field)
	}

	part, err := writer.CreateFormFile("file", "part.bin")
	s.Require().NoError(err, "Multipart request should create the file part")

	_, err = part.Write(bytes.Repeat([]byte{'a'}, int(singleShotSize)))
	s.Require().NoError(err, "Multipart request should write file content")
	s.Require().NoError(writer.Close(), "Multipart writer should close successfully")

	req := httptest.NewRequestWithContext(s.ctx, fiber.MethodPost, "/api", form)
	req.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())
	req.Header.Set(fiber.HeaderAuthorization, security.AuthSchemeBearer+" "+s.ownerToken)

	partResp, err := s.App.Test(req)
	s.Require().NoError(err, "Upload part request should not fail at the transport layer")
	s.Require().True(s.ReadResult(partResp).IsOk(), "Upload part should succeed")

	completeResp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)
	s.Require().True(s.ReadResult(completeResp).IsOk(), "Complete upload should succeed")

	return key
}

// resolve calls sys/storage/file.resolve and returns the resolved rows.
func (s *FileResolveTestSuite) resolve(keys ...string) []map[string]any {
	s.T().Helper()

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage/file", Action: "resolve", Version: "v1"},
		Params:     map[string]any{"keys": keys},
	}, s.ownerToken)

	body := s.ReadResult(resp)
	s.Require().True(body.IsOk(), "Resolve should succeed: %s", body.Message)

	data := s.ReadDataAsMap(body.Data)

	files, ok := data["files"].([]any)
	s.Require().True(ok, "Resolve result must carry a files array")

	rows := make([]map[string]any, 0, len(files))

	for _, entry := range files {
		row, isMap := entry.(map[string]any)
		s.Require().True(isMap, "Each resolved file should be an object")

		rows = append(rows, row)
	}

	return rows
}

func (s *FileResolveTestSuite) TestResolvesPublicKey() {
	key := s.uploadFile("季度报告.pdf", true)

	rows := s.resolve(key)
	s.Require().Len(rows, 1, "A readable recorded key should resolve")
	s.Equal(key, rows[0]["key"], "The row should echo the requested key")
	s.Equal("季度报告.pdf", rows[0]["originalFilename"], "The original filename is what the caller came for")
	s.Equal(float64(singleShotSize), rows[0]["size"], "The row should carry the recorded size")
	s.Equal("test-owner", rows[0]["uploadedBy"], "The row should carry the uploader")
	s.Equal(string(storage.FileStatusUploaded), rows[0]["status"], "An unadopted file resolves as uploaded")
}

func (s *FileResolveTestSuite) TestOmitsUnknownKeys() {
	key := s.uploadFile("known.pdf", true)

	rows := s.resolve(key, "pub/2026/01/01/never-uploaded.pdf")
	s.Require().Len(rows, 1, "An unknown key is omitted rather than reported")
	s.Equal(key, rows[0]["key"], "Only the recorded key should come back")
}

func (s *FileResolveTestSuite) TestPreservesRequestOrderAndCollapsesDuplicates() {
	first := s.uploadFile("first.pdf", true)
	second := s.uploadFile("second.pdf", true)

	rows := s.resolve(second, first, second)
	s.Require().Len(rows, 2, "Duplicate keys should collapse")
	s.Equal(second, rows[0]["key"], "The response should follow the request order")
	s.Equal(first, rows[1]["key"], "The response should follow the request order")
}

// The default ACL denies every private key, and this endpoint must not
// become a weaker door than the download proxy that sits behind the same
// check.
func (s *FileResolveTestSuite) TestOmitsPrivateKeysUnderTheDefaultACL() {
	key := s.uploadFile("secret.pdf", false)

	if s.permissive {
		s.Require().Len(s.resolve(key), 1, "A configured ACL should let the private key through")

		return
	}

	s.Empty(s.resolve(key), "Without an application ACL a private key must not be named")
}

// The shape this endpoint's authorization takes when a client resolves a
// mixed list, and the only reason a UI ever shows original filenames for
// public files while private ones fall back to the bare object key: a pub/
// key never reaches the ACL, so it always resolves, while a priv/ key
// resolves only if the application's FileACL grants it. Denied keys are
// omitted rather than reported, so the client cannot tell this apart from
// "never uploaded" — the server log is the only signal.
func (s *FileResolveTestSuite) TestPublicKeysResolveWhileDeniedPrivateOnesAreOmitted() {
	publicKey := s.uploadFile("公开通知.pdf", true)
	privateKey := s.uploadFile("内部纪要.pdf", false)

	rows := s.resolve(publicKey, privateKey)

	if s.permissive {
		s.Require().Len(rows, 2, "A configured ACL should resolve both visibilities")
		s.Equal("公开通知.pdf", rows[0]["originalFilename"], "The public file keeps its name")
		s.Equal("内部纪要.pdf", rows[1]["originalFilename"], "The private file keeps its name too")

		return
	}

	s.Require().Len(rows, 1, "Only the public key survives an ACL that denies private reads")
	s.Equal(publicKey, rows[0]["key"], "The surviving row should be the public one")
	s.Equal("公开通知.pdf", rows[0]["originalFilename"],
		"A public key resolves without ever consulting the ACL")
}

func TestFileResolve(t *testing.T) {
	suite.Run(t, new(FileResolveTestSuite))
}

func TestFileResolveWithApplicationACL(t *testing.T) {
	suite.Run(t, &FileResolveTestSuite{permissive: true})
}

package storage_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
)

const (
	// testMaxUploadSize bounds the size cap for init_upload so the
	// oversized-file cases can be exercised without allocating a
	// gigabyte buffer. Picked to be larger than the chunked test payload
	// (80 KiB) but small enough that 1 MiB+1 still trips the limit.
	testMaxUploadSize int64 = 1024 * 1024
	// memoryPartSize mirrors the memory backend's hard-coded 64 KiB part
	// size; chunked test payloads use this to land on a partCount > 1
	// without depending on the public part-size constant.
	memoryPartSize int64 = 64 * 1024
	// chunkedSize is the total size for multi-part upload scenarios. It
	// is memoryPartSize * 1.25 so the upload splits into exactly two
	// parts.
	chunkedSize int64 = 80 * 1024
	// singleShotSize is the total size for the N=1 happy path — a file
	// small enough that ceil(size/partSize) == 1, exercising the unified
	// protocol's degenerate single-part case.
	singleShotSize int64 = 1024
)

// StorageResourceTestSuite exercises the sys/storage RPC actions
// against the in-memory storage backend with a SQLite-backed claim and
// upload-part store. Each test starts from a fresh init_upload so the
// claims do not leak between cases; the storage.Service handle is used
// to seed objects directly when the test needs to bypass the upload
// flow (e.g. ACL checks against pre-existing keys).
type StorageResourceTestSuite struct {
	apptest.Suite

	ctx     context.Context
	service storage.Service

	// ownerToken belongs to the principal that drives every chunked
	// upload in the suite; otherToken is a different principal used
	// solely to assert ownership rejection on upload_part.
	ownerToken string
	otherToken string
}

func (s *StorageResourceTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.SetupApp(
		fx.Replace(
			&config.DataSourceConfig{Kind: config.SQLite},
			&config.StorageConfig{
				Provider:      config.StorageMemory,
				AutoMigrate:   true,
				MaxUploadSize: testMaxUploadSize,
			},
			&security.JWTConfig{
				Secret:   security.DefaultJWTSecret,
				Audience: "test_app",
			},
		),
		fx.Populate(&s.service),
	)

	s.ownerToken = s.GenerateToken(&security.Principal{ID: "test-owner", Name: "owner"})
	s.otherToken = s.GenerateToken(&security.Principal{ID: "test-other", Name: "other"})
}

func (s *StorageResourceTestSuite) TearDownSuite() {
	s.TearDownApp()
}

// makeMultipartRequest sends a multipart/form-data POST to /api with
// the given form fields and an optional file attached as the "file"
// part. Used for the upload and upload_part actions, which both reject
// JSON bodies at the handler level.
func (s *StorageResourceTestSuite) makeMultipartRequest(token string, fields map[string]string, fileName string, fileContent []byte) *http.Response {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for k, v := range fields {
		s.Require().NoError(writer.WriteField(k, v), "Should write form field %s", k)
	}

	if fileName != "" {
		part, err := writer.CreateFormFile("file", fileName)
		s.Require().NoError(err, "Should create form file")

		_, err = part.Write(fileContent)
		s.Require().NoError(err, "Should write file content")
	}

	s.Require().NoError(writer.Close(), "Should close multipart writer")

	req := httptest.NewRequestWithContext(s.ctx, fiber.MethodPost, "/api", body)
	req.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())

	if token != "" {
		req.Header.Set(fiber.HeaderAuthorization, security.AuthSchemeBearer+" "+token)
	}

	resp, err := s.App.Test(req)
	s.Require().NoError(err, "API request should not fail at the transport layer")

	return resp
}

// uploadPart drives the upload_part action with the given claim ID and
// part position. The claimId / partNumber pair is encoded into the
// "params" form field because the framework's multipart binder pulls
// non-file params from there (file headers go in their own form parts).
func (s *StorageResourceTestSuite) uploadPart(token, claimID string, partNumber int, content []byte) *http.Response {
	paramsJSON, err := json.Marshal(map[string]any{
		"claimId":    claimID,
		"partNumber": partNumber,
	})
	s.Require().NoError(err, "Should marshal upload_part params")

	return s.makeMultipartRequest(token, map[string]string{
		"resource": "sys/storage",
		"action":   "upload_part",
		"version":  "v1",
		"params":   string(paramsJSON),
	}, "part.bin", content)
}

// initUpload drives init_upload and returns the parsed result data
// (when the call succeeds) plus the raw Result so callers can inspect
// failure cases. Tests that need only the success case dereference
// data; tests asserting the failure case ignore data.
func (s *StorageResourceTestSuite) initUpload(filename string, size int64) (map[string]any, result.Result) {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "init_upload", Version: "v1"},
		Params: map[string]any{
			"filename": filename,
			"size":     size,
		},
	}, s.ownerToken)

	body := s.ReadResult(resp)
	if !body.IsOk() {
		return nil, body
	}

	return s.ReadDataAsMap(body.Data), body
}

// ── init_upload ─────────────────────────────────────────────────────────

func (s *StorageResourceTestSuite) TestInitUploadHappyPath() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "init_upload should succeed: %s", body.Message)

	s.NotEmpty(data["claimId"], "Response must include the new claim ID")
	s.NotEmpty(data["uploadId"], "Response must include the backend upload ID")
	s.True(strings.HasPrefix(data["key"].(string), storage.PrivatePrefix), "Default visibility is private")
	s.Equal("video.mp4", data["originalFilename"], "Original filename should be echoed back")
	s.Equal(float64(memoryPartSize), data["partSize"], "Part size should mirror the backend's authoritative value")
	s.Equal(float64(2), data["partCount"], "80 KiB / 64 KiB → 2 parts")
}

func (s *StorageResourceTestSuite) TestInitUploadRejectsOversizedFile() {
	_, body := s.initUpload("oversized.bin", testMaxUploadSize+1)
	s.False(body.IsOk(), "init_upload exceeding the configured cap must fail")
}

func (s *StorageResourceTestSuite) TestInitUploadSinglePartForSmallFile() {
	// Files at or below the backend's PartSize collapse to a single
	// part. The unified protocol still routes them through init_upload
	// — the only difference is partCount=1 instead of partCount>1.
	data, body := s.initUpload("note.txt", singleShotSize)
	s.Require().True(body.IsOk(), "init_upload should accept files smaller than partSize")

	s.Equal(float64(memoryPartSize), data["partSize"], "Part size should mirror the backend's authoritative value")
	s.Equal(float64(1), data["partCount"], "Files <= partSize should yield exactly one part")
}

// ── upload_part ─────────────────────────────────────────────────────────

func (s *StorageResourceTestSuite) TestUploadPartHappyPath() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk())

	claimID := data["claimId"].(string)
	part := bytes.Repeat([]byte{'a'}, int(memoryPartSize))

	resp := s.uploadPart(s.ownerToken, claimID, 1, part)
	s.Equal(http.StatusOK, resp.StatusCode, "Should return 200 OK")

	body = s.ReadResult(resp)
	s.Require().True(body.IsOk(), "upload_part should succeed: %s", body.Message)

	m := s.ReadDataAsMap(body.Data)
	s.Equal(float64(1), m["partNumber"], "Response should echo the requested part number")
	s.Equal(float64(memoryPartSize), m["size"], "Response should report the recorded part size")
}

func (s *StorageResourceTestSuite) TestUploadPartRejectsWrongOwner() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk())

	claimID := data["claimId"].(string)
	part := bytes.Repeat([]byte{'a'}, int(memoryPartSize))

	resp := s.uploadPart(s.otherToken, claimID, 1, part)
	body = s.ReadResult(resp)
	s.False(body.IsOk(), "upload_part from a non-owner principal must fail")
}

func (s *StorageResourceTestSuite) TestUploadPartRejectsOutOfRangePartNumber() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk())

	claimID := data["claimId"].(string)
	part := bytes.Repeat([]byte{'a'}, int(memoryPartSize))

	// partCount is 2; 99 is well outside the [1, 2] range.
	resp := s.uploadPart(s.ownerToken, claimID, 99, part)
	body = s.ReadResult(resp)
	s.False(body.IsOk(), "upload_part with an out-of-range partNumber must fail")
}

// ── complete_upload ─────────────────────────────────────────────────────

func (s *StorageResourceTestSuite) TestCompleteUploadHappyPath() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk())

	claimID := data["claimId"].(string)

	// Upload both parts: a 64 KiB chunk and a 16 KiB tail.
	part1 := bytes.Repeat([]byte{'a'}, int(memoryPartSize))
	part2 := bytes.Repeat([]byte{'b'}, int(chunkedSize-memoryPartSize))

	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 1, part1)).IsOk(), "Should upload part 1")
	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 2, part2)).IsOk(), "Should upload part 2")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)

	body = s.ReadResult(resp)
	s.Require().True(body.IsOk(), "complete_upload should succeed: %s", body.Message)

	m := s.ReadDataAsMap(body.Data)
	s.Equal(data["key"], m["key"], "Final key should match the planned key")
	s.Equal(float64(chunkedSize), m["size"], "Assembled object size should match the declared size")
	s.Equal("video.mp4", m["originalFilename"], "Original filename should round-trip through complete_upload")
}

func (s *StorageResourceTestSuite) TestCompleteUploadHappyPathSinglePart() {
	// End-to-end N=1 path: small file flows through the same protocol
	// (init → upload_part(1) → complete) without any single-shot
	// shortcut.
	data, body := s.initUpload("note.txt", singleShotSize)
	s.Require().True(body.IsOk())
	s.Equal(float64(1), data["partCount"], "Small file should yield exactly one part")

	claimID := data["claimId"].(string)
	payload := bytes.Repeat([]byte{'z'}, int(singleShotSize))

	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 1, payload)).IsOk(), "Should upload the only part")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)

	body = s.ReadResult(resp)
	s.Require().True(body.IsOk(), "complete_upload should succeed for the single-part case: %s", body.Message)

	m := s.ReadDataAsMap(body.Data)
	s.Equal(data["key"], m["key"], "Final key should match the planned key")
	s.Equal(float64(singleShotSize), m["size"], "Assembled object size should match the declared size")
	s.Equal("note.txt", m["originalFilename"], "Original filename should round-trip through complete_upload")
}

func (s *StorageResourceTestSuite) TestCompleteUploadDeletesObjectOnSizeMismatch() {
	// Declare size=1024 but upload only 50 bytes as the single part.
	// CompleteMultipart assembles a 50-byte object; the handler detects
	// info.Size != claim.Size and must delete the orphan object before
	// returning the error.
	declaredSize := int64(1024)
	actualPayload := bytes.Repeat([]byte{'m'}, 50)

	data, body := s.initUpload("mismatch.bin", declaredSize)
	s.Require().True(body.IsOk())

	claimID := data["claimId"].(string)
	key := data["key"].(string)

	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 1, actualPayload)).IsOk(), "Should upload the part")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)

	body = s.ReadResult(resp)
	s.False(body.IsOk(), "complete_upload must fail when assembled size != declared size")

	// Verify the orphan object was cleaned up from the backend.
	_, err := s.service.StatObject(s.ctx, storage.StatObjectOptions{Key: key})
	s.ErrorIs(err, storage.ErrObjectNotFound, "Object must be deleted after size mismatch")
}

func (s *StorageResourceTestSuite) TestCompleteUploadRejectsIncompleteParts() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk())

	claimID := data["claimId"].(string)

	// Upload only the first part, then try to complete the upload.
	part1 := bytes.Repeat([]byte{'a'}, int(memoryPartSize))
	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 1, part1)).IsOk())

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)

	body = s.ReadResult(resp)
	s.False(body.IsOk(), "complete_upload with missing parts must fail")
}

// ── abort_upload ────────────────────────────────────────────────────────

func (s *StorageResourceTestSuite) TestAbortUploadHappyPath() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk())

	claimID := data["claimId"].(string)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "abort_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)

	body = s.ReadResult(resp)
	s.True(body.IsOk(), "abort_upload should succeed: %s", body.Message)
}

func (s *StorageResourceTestSuite) TestAbortUploadIdempotentOnMissingClaim() {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "abort_upload", Version: "v1"},
		Params:     map[string]any{"claimId": "non-existent-claim"},
	}, s.ownerToken)

	body := s.ReadResult(resp)
	s.True(body.IsOk(), "abort_upload on a missing claim must be a silent no-op")
}

// ── validation edge cases ───────────────────────────────────────────────

func (s *StorageResourceTestSuite) TestUploadPartRejectsNonFinalPartTooSmall() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk())

	claimID := data["claimId"].(string)
	// Part 1 is non-final (partCount=2) but smaller than partSize.
	smallPart := bytes.Repeat([]byte{'s'}, int(memoryPartSize)/2)

	resp := s.uploadPart(s.ownerToken, claimID, 1, smallPart)
	partBody := s.ReadResult(resp)
	s.False(partBody.IsOk(), "Non-final part smaller than partSize must be rejected")
}

func (s *StorageResourceTestSuite) TestInitUploadRejectsFilenameTooLong() {
	longName := strings.Repeat("a", 256) + ".txt"
	_, body := s.initUpload(longName, singleShotSize)
	s.False(body.IsOk(), "Filename exceeding 255 chars must be rejected by validation")
}

func TestStorageResource(t *testing.T) {
	suite.Run(t, new(StorageResourceTestSuite))
}

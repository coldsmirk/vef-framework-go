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
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/orm"
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
	db      orm.DB
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
		fx.Populate(&s.db),
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
		s.Require().NoError(writer.WriteField(k, v), "Multipart request should write form field %s", k)
	}

	if fileName != "" {
		part, err := writer.CreateFormFile("file", fileName)
		s.Require().NoError(err, "Multipart request should create the file part")

		_, err = part.Write(fileContent)
		s.Require().NoError(err, "Multipart request should write file content")
	}

	s.Require().NoError(writer.Close(), "Multipart writer should close successfully")

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
	s.Require().NoError(err, "Upload part params should marshal to JSON")

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

func (s *StorageResourceTestSuite) requireString(data map[string]any, key string) string {
	s.T().Helper()

	value, ok := data[key].(string)
	s.Require().True(ok, "Response must include string field %s", key)

	return value
}

// completeChunkedUpload drives the whole init → upload_part ×2 →
// complete protocol for a two-part payload and returns the claim ID and
// the final object key.
func (s *StorageResourceTestSuite) completeChunkedUpload(filename string) (claimID, key string) {
	s.T().Helper()

	data, body := s.initUpload(filename, chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should succeed: %s", body.Message)

	claimID = s.requireString(data, "claimId")
	key = s.requireString(data, "key")

	part1 := bytes.Repeat([]byte{'a'}, int(memoryPartSize))
	part2 := bytes.Repeat([]byte{'b'}, int(chunkedSize-memoryPartSize))

	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 1, part1)).IsOk(),
		"Upload part 1 should succeed")
	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 2, part2)).IsOk(),
		"Upload part 2 should succeed")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)
	s.Require().True(s.ReadResult(resp).IsOk(), "Complete upload should succeed")

	return claimID, key
}

// ── init_upload ─────────────────────────────────────────────────────────

func (s *StorageResourceTestSuite) TestInitUploadHappyPath() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should succeed: %s", body.Message)

	s.NotEmpty(data["claimId"], "Response must include the new claim ID")
	s.NotContains(data, "uploadId", "Backend upload ID must NOT leak to the client surface")
	key := s.requireString(data, "key")
	s.True(strings.HasPrefix(key, storage.PrivatePrefix), "Default visibility is private")
	s.Equal("video.mp4", data["originalFilename"], "Original filename should be echoed back")
	s.Equal(float64(memoryPartSize), data["partSize"], "Part size should mirror the backend's authoritative value")
	s.Equal(float64(2), data["partCount"], "80 KiB / 64 KiB → 2 parts")
}

// TestInitUploadSanitizesTheFilename covers the single validation point
// for a value that ends up in the registry, in business UIs, and in a
// response header. Driven through the RPC rather than against the helper
// so it also proves the sanitizer is actually wired into the flow.
func (s *StorageResourceTestSuite) TestInitUploadSanitizesTheFilename() {
	accepted := []struct {
		name     string
		filename string
		expected string
	}{
		{"KeepsAPlainName", "report.pdf", "report.pdf"},
		{"KeepsNonASCII", "季度报告 v2.pdf", "季度报告 v2.pdf"},
		{"TrimsSurroundingWhitespace", "  report.pdf  ", "report.pdf"},
		{"StripsAWindowsPath", `C:\Users\me\Desktop\report.pdf`, "report.pdf"},
		{"StripsAPosixPath", "/home/me/report.pdf", "report.pdf"},
	}

	for _, tc := range accepted {
		s.Run(tc.name, func() {
			data, body := s.initUpload(tc.filename, singleShotSize)
			s.Require().True(body.IsOk(), "Init upload should accept %q: %s", tc.filename, body.Message)
			s.Equal(tc.expected, data["originalFilename"], "Stored filename should be the sanitized form")
		})
	}

	rejected := []struct {
		name     string
		filename string
	}{
		{"RejectsCRLF", "evil\r\nX-Injected: yes.pdf"},
		{"RejectsControlCharacters", "report\x00.pdf"},
		{"RejectsWhitespaceOnly", "   "},
		{"RejectsBareDot", "."},
		{"RejectsParentDirectory", ".."},
		{"RejectsAPathWithNoFinalSegment", "some/directory/"},
	}

	for _, tc := range rejected {
		s.Run(tc.name, func() {
			_, body := s.initUpload(tc.filename, singleShotSize)
			s.False(body.IsOk(), "Init upload must reject %q", tc.filename)
			s.Equal(storage.ErrCodeInvalidFilename, body.Code, "Rejection should carry the invalid-filename code")
		})
	}
}

func (s *StorageResourceTestSuite) TestInitUploadRejectsOversizedFile() {
	_, body := s.initUpload("oversized.bin", testMaxUploadSize+1)
	s.False(body.IsOk(), "Init upload exceeding the configured cap must fail")
}

func (s *StorageResourceTestSuite) TestInitUploadSinglePartForSmallFile() {
	// Files at or below the backend's PartSize collapse to a single
	// part. The unified protocol still routes them through init_upload
	// — the only difference is partCount=1 instead of partCount>1.
	data, body := s.initUpload("note.txt", singleShotSize)
	s.Require().True(body.IsOk(), "Init upload should accept files smaller than partSize")

	s.Equal(float64(memoryPartSize), data["partSize"], "Part size should mirror the backend's authoritative value")
	s.Equal(float64(1), data["partCount"], "Files <= partSize should yield exactly one part")
}

// ── upload_part ─────────────────────────────────────────────────────────

func (s *StorageResourceTestSuite) TestUploadPartHappyPath() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before upload_part: %s", body.Message)

	claimID := s.requireString(data, "claimId")
	part := bytes.Repeat([]byte{'a'}, int(memoryPartSize))

	resp := s.uploadPart(s.ownerToken, claimID, 1, part)
	s.Equal(http.StatusOK, resp.StatusCode, "Upload part should return HTTP 200")

	body = s.ReadResult(resp)
	s.Require().True(body.IsOk(), "Upload part should succeed: %s", body.Message)

	m := s.ReadDataAsMap(body.Data)
	s.Equal(float64(1), m["partNumber"], "Response should echo the requested part number")
	s.Equal(float64(memoryPartSize), m["size"], "Response should report the recorded part size")
}

func (s *StorageResourceTestSuite) TestUploadPartRejectsWrongOwner() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before wrong-owner upload_part: %s", body.Message)

	claimID := s.requireString(data, "claimId")
	part := bytes.Repeat([]byte{'a'}, int(memoryPartSize))

	resp := s.uploadPart(s.otherToken, claimID, 1, part)
	body = s.ReadResult(resp)
	s.False(body.IsOk(), "Upload part from a non-owner principal must fail")
}

func (s *StorageResourceTestSuite) TestUploadPartRejectsOutOfRangePartNumber() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before out-of-range upload_part: %s", body.Message)

	claimID := s.requireString(data, "claimId")
	part := bytes.Repeat([]byte{'a'}, int(memoryPartSize))

	// partCount is 2; 99 is well outside the [1, 2] range.
	resp := s.uploadPart(s.ownerToken, claimID, 99, part)
	body = s.ReadResult(resp)
	s.False(body.IsOk(), "Upload part with an out-of-range partNumber must fail")
}

// ── complete_upload ─────────────────────────────────────────────────────

func (s *StorageResourceTestSuite) TestCompleteUploadHappyPath() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before complete_upload: %s", body.Message)

	claimID := s.requireString(data, "claimId")

	// Upload both parts: a 64 KiB chunk and a 16 KiB tail.
	part1 := bytes.Repeat([]byte{'a'}, int(memoryPartSize))
	part2 := bytes.Repeat([]byte{'b'}, int(chunkedSize-memoryPartSize))

	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 1, part1)).IsOk(), "Upload part should accept part 1")
	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 2, part2)).IsOk(), "Upload part should accept part 2")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)

	body = s.ReadResult(resp)
	s.Require().True(body.IsOk(), "Complete upload should succeed: %s", body.Message)

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
	s.Require().True(body.IsOk(), "Init upload should prepare a single-part claim: %s", body.Message)
	s.Equal(float64(1), data["partCount"], "Small file should yield exactly one part")

	claimID := s.requireString(data, "claimId")
	payload := bytes.Repeat([]byte{'z'}, int(singleShotSize))

	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 1, payload)).IsOk(), "Upload part should accept the only part")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)

	body = s.ReadResult(resp)
	s.Require().True(body.IsOk(), "Complete upload should succeed for the single-part case: %s", body.Message)

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
	s.Require().True(body.IsOk(), "Init upload should prepare a claim for size mismatch cleanup: %s", body.Message)

	claimID := s.requireString(data, "claimId")
	key := s.requireString(data, "key")

	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 1, actualPayload)).IsOk(), "Upload part should accept the undersized payload")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)

	body = s.ReadResult(resp)
	s.False(body.IsOk(), "Complete upload must fail when assembled size != declared size")

	// Verify the orphan object was cleaned up from the backend.
	_, err := s.service.StatObject(s.ctx, storage.StatObjectOptions{Key: key})
	s.ErrorIs(err, storage.ErrObjectNotFound, "Object must be deleted after size mismatch")
}

func (s *StorageResourceTestSuite) TestCompleteUploadRejectsIncompleteParts() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before incomplete complete_upload: %s", body.Message)

	claimID := s.requireString(data, "claimId")

	// Upload only the first part, then try to complete the upload.
	part1 := bytes.Repeat([]byte{'a'}, int(memoryPartSize))
	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 1, part1)).IsOk(),
		"Upload part should accept the first chunk before incomplete complete_upload")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)

	body = s.ReadResult(resp)
	s.False(body.IsOk(), "Complete upload with missing parts must fail")
}

// ── abort_upload ────────────────────────────────────────────────────────

func (s *StorageResourceTestSuite) TestAbortUploadHappyPath() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before abort_upload: %s", body.Message)

	claimID := s.requireString(data, "claimId")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "abort_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)

	body = s.ReadResult(resp)
	s.True(body.IsOk(), "Abort upload should succeed: %s", body.Message)
}

// TestAbortUploadSchedulesBackendCleanup pins the crash-safety property
// of the abort flow: the handler never touches the backend itself, it
// hands the object (and its multipart session) to the durable delete
// queue inside the same transaction that removes the claim. A process
// death right after the commit therefore cannot strand object bytes
// that nothing remembers.
func (s *StorageResourceTestSuite) TestAbortUploadSchedulesBackendCleanup() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before abort_upload: %s", body.Message)

	claimID := s.requireString(data, "claimId")
	key := s.requireString(data, "key")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "abort_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)
	s.Require().True(s.ReadResult(resp).IsOk(), "Abort upload should succeed before the queue check")

	queued := s.pendingDeletes(key)
	s.Require().Len(queued, 1, "Abort must enqueue exactly one pending delete for the aborted key")
	s.Equal(storage.DeleteReasonAborted, queued[0].Reason, "Queue row should carry the aborted reason")
	s.NotEmpty(queued[0].UploadID, "Multipart session must ride along so the worker aborts it before deleting")
}

// TestAbortUploadOnCompletedClaimLeavesObjectAlone is the regression
// fence for the abort-versus-complete race: once a claim reaches
// 'uploaded' the object is finalized business state, so abort must stay
// a silent no-op instead of scheduling the finalized object's deletion.
func (s *StorageResourceTestSuite) TestAbortUploadOnCompletedClaimLeavesObjectAlone() {
	claimID, key := s.completeChunkedUpload("report.pdf")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "abort_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)
	s.Require().True(s.ReadResult(resp).IsOk(), "Abort on a completed claim must be a silent no-op")

	s.Empty(s.pendingDeletes(key), "A completed upload must never be scheduled for deletion by abort")

	_, err := s.service.StatObject(s.ctx, storage.StatObjectOptions{Key: key})
	s.NoError(err, "The finalized object must still exist after a losing abort")
}

// ── file registry ───────────────────────────────────────────────────────

// TestCompleteUploadRecordsTheFile is the end-to-end proof of the
// feature: the original filename must survive the claim row, which
// Consume deletes the moment a business transaction adopts the file.
func (s *StorageResourceTestSuite) TestCompleteUploadRecordsTheFile() {
	claimID, key := s.completeChunkedUpload("季度报告 v2.mp4")

	records := s.fileRecords(key)
	s.Require().Len(records, 1, "A finalized upload must produce exactly one registry record")

	record := records[0]
	s.Equal(claimID, record.ID, "The record should be keyed by the originating claim ID")
	s.Equal("季度报告 v2.mp4", record.OriginalFilename, "The original filename must be recorded verbatim")
	s.Equal(chunkedSize, record.Size, "The recorded size should match the uploaded object")
	s.Equal(storage.FileStatusUploaded, record.Status, "A finalized but unadopted file is uploaded, not claimed")
	s.Equal("test-owner", record.CreatedBy, "The record should attribute the file to the uploading principal")
	s.False(record.Public, "A default-visibility upload should be recorded as private")
}

// TestCompleteUploadIdempotentRetryRecordsOnce fences the fast path at
// the top of complete_upload: it opens no transaction, so a retry must
// not produce a second record.
func (s *StorageResourceTestSuite) TestCompleteUploadIdempotentRetryRecordsOnce() {
	claimID, key := s.completeChunkedUpload("retried.mp4")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)
	s.Require().True(s.ReadResult(resp).IsOk(), "The idempotent retry should succeed")

	s.Len(s.fileRecords(key), 1, "An idempotent complete_upload retry must not duplicate the registry record")
}

// TestInitUploadRecordsNothing fences the other end: nothing is
// materialized until complete_upload, so an open session must leave the
// registry empty.
func (s *StorageResourceTestSuite) TestInitUploadRecordsNothing() {
	data, body := s.initUpload("never-finished.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should succeed: %s", body.Message)

	s.Empty(s.fileRecords(s.requireString(data, "key")), "An unfinished upload must not be recorded")
}

// fileRecords returns the registry rows for key.
func (s *StorageResourceTestSuite) fileRecords(key string) []storage.FileRecord {
	s.T().Helper()

	var records []storage.FileRecord

	err := s.db.NewSelect().Model(&records).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("object_key", key)
	}).Scan(s.ctx)
	s.Require().NoError(err, "File registry lookup should succeed")

	return records
}

// pendingDeletes returns the delete-queue rows targeting key.
func (s *StorageResourceTestSuite) pendingDeletes(key string) []store.PendingDelete {
	s.T().Helper()

	var queued []store.PendingDelete

	err := s.db.NewSelect().Model(&queued).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("object_key", key)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Pending-delete lookup should succeed")

	return queued
}

func (s *StorageResourceTestSuite) TestAbortUploadIdempotentOnMissingClaim() {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "abort_upload", Version: "v1"},
		Params:     map[string]any{"claimId": "non-existent-claim"},
	}, s.ownerToken)

	body := s.ReadResult(resp)
	s.True(body.IsOk(), "Abort upload on a missing claim must be a silent no-op")
}

// ── list_parts ──────────────────────────────────────────────────────────

// listParts is a JSON RPC unlike upload_part — no multipart form
// encoding is needed even though the action lives on the chunked
// upload pipeline. The helper exists so the test suite uses the same
// dispatch path as the production client (MakeRPCRequestWithToken).
func (s *StorageResourceTestSuite) listParts(token, claimID string) result.Result {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "list_parts", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, token)

	return s.ReadResult(resp)
}

func (s *StorageResourceTestSuite) TestListPartsEmptyBeforeAnyUpload() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before list_parts: %s", body.Message)

	listed := s.listParts(s.ownerToken, s.requireString(data, "claimId"))
	s.Require().True(listed.IsOk(), "List parts on a fresh claim should succeed: %s", listed.Message)

	m := s.ReadDataAsMap(listed.Data)
	parts, ok := m["parts"].([]any)
	s.Require().True(ok, "Response must include a parts array")
	s.Empty(parts, "A claim without any uploaded parts should yield an empty list")
}

func (s *StorageResourceTestSuite) TestListPartsReportsUploadedParts() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before uploaded part listing: %s", body.Message)

	claimID := s.requireString(data, "claimId")
	part1 := bytes.Repeat([]byte{'a'}, int(memoryPartSize))
	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 1, part1)).IsOk(),
		"Upload part should accept the part before list_parts")

	listed := s.listParts(s.ownerToken, claimID)
	s.Require().True(listed.IsOk(), "List parts after one uploaded part should succeed: %s", listed.Message)

	m := s.ReadDataAsMap(listed.Data)
	parts, ok := m["parts"].([]any)
	s.Require().True(ok, "Response must include a parts array")
	s.Require().Len(parts, 1, "Exactly one part should be reported")

	first, ok := parts[0].(map[string]any)
	s.Require().True(ok, "Reported part entry must be an object")
	s.Equal(float64(1), first["partNumber"], "Reported part number should match the upload")
	s.Equal(float64(memoryPartSize), first["size"], "Reported part size should match the upload")
}

func (s *StorageResourceTestSuite) TestListPartsRejectsWrongOwner() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before wrong-owner list_parts: %s", body.Message)

	listed := s.listParts(s.otherToken, s.requireString(data, "claimId"))
	s.False(listed.IsOk(), "List parts from a non-owner principal must fail")
}

func (s *StorageResourceTestSuite) TestListPartsRejectsCompletedClaim() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before completed list_parts rejection: %s", body.Message)

	claimID := s.requireString(data, "claimId")
	part1 := bytes.Repeat([]byte{'a'}, int(memoryPartSize))
	part2 := bytes.Repeat([]byte{'b'}, int(chunkedSize-memoryPartSize))

	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 1, part1)).IsOk(),
		"Upload part should accept part 1 before completing the claim")
	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 2, part2)).IsOk(),
		"Upload part should accept part 2 before completing the claim")

	completeResp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)
	s.Require().True(s.ReadResult(completeResp).IsOk(), "Complete upload should succeed before the rejection check")

	listed := s.listParts(s.ownerToken, claimID)
	s.False(listed.IsOk(), "List parts on an already-completed claim must fail (claim is no longer pending)")
}

func (s *StorageResourceTestSuite) TestListPartsAfterAbortReturnsClaimNotFound() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before aborted list_parts rejection: %s", body.Message)

	claimID := s.requireString(data, "claimId")

	abortResp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "abort_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)
	s.Require().True(s.ReadResult(abortResp).IsOk(), "Abort upload should succeed before the lookup check")

	listed := s.listParts(s.ownerToken, claimID)
	s.False(listed.IsOk(), "List parts on an aborted (deleted) claim must fail")
}

// ── validation edge cases ───────────────────────────────────────────────

func (s *StorageResourceTestSuite) TestUploadPartRejectsNonFinalPartTooSmall() {
	data, body := s.initUpload("video.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload should prepare a claim before non-final size validation: %s", body.Message)

	claimID := s.requireString(data, "claimId")
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

// ── public upload rejection ─────────────────────────────────────────────

// initUploadPublic is like initUpload but sets public=true on the
// init_upload params. Used only to exercise the AllowPublicUploads gate.
func (s *StorageResourceTestSuite) initUploadPublic(filename string, size int64) (map[string]any, result.Result) {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "init_upload", Version: "v1"},
		Params: map[string]any{
			"filename": filename,
			"size":     size,
			"public":   true,
		},
	}, s.ownerToken)

	body := s.ReadResult(resp)
	if !body.IsOk() {
		return nil, body
	}

	return s.ReadDataAsMap(body.Data), body
}

func (s *StorageResourceTestSuite) TestInitUploadRejectsPublicWhenNotAllowed() {
	// The suite fixture sets AllowPublicUploads to its zero value (false),
	// so any public=true request must be rejected with the
	// storage_public_uploads_not_allowed business error.
	_, body := s.initUploadPublic("photo.jpg", singleShotSize)
	s.False(body.IsOk(), "InitUpload with public=true must fail when AllowPublicUploads=false")
	s.Equal(storage.ErrCodePublicUploadsNotAllowed, body.Code, "Error code must be the public-uploads-not-allowed business code")
}

// ── complete_upload idempotent retry ────────────────────────────────────

func (s *StorageResourceTestSuite) TestCompleteUploadIdempotentOnAlreadyCompleted() {
	// Drive the full upload to completion, then call complete_upload a
	// second time. The handler detects ClaimStatusUploaded, re-stats the
	// object, and returns the same success response instead of erroring.
	data, body := s.initUpload("idempotent.mp4", chunkedSize)
	s.Require().True(body.IsOk(), "Init upload must succeed before idempotent complete test: %s", body.Message)

	claimID := s.requireString(data, "claimId")
	expectedKey := s.requireString(data, "key")

	part1 := bytes.Repeat([]byte{'a'}, int(memoryPartSize))
	part2 := bytes.Repeat([]byte{'b'}, int(chunkedSize-memoryPartSize))

	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 1, part1)).IsOk(),
		"Upload part 1 must succeed before idempotent complete test")
	s.Require().True(s.ReadResult(s.uploadPart(s.ownerToken, claimID, 2, part2)).IsOk(),
		"Upload part 2 must succeed before idempotent complete test")

	// First complete: normal path.
	firstResp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)

	firstBody := s.ReadResult(firstResp)
	s.Require().True(firstBody.IsOk(), "First complete_upload must succeed: %s", firstBody.Message)

	// Second complete: idempotent fast-path via ClaimStatusUploaded branch.
	secondResp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "sys/storage", Action: "complete_upload", Version: "v1"},
		Params:     map[string]any{"claimId": claimID},
	}, s.ownerToken)

	secondBody := s.ReadResult(secondResp)
	s.True(secondBody.IsOk(), "Second complete_upload on an already-completed claim must succeed idempotently: %s", secondBody.Message)

	// Both responses must agree on the final key and original filename.
	first := s.ReadDataAsMap(firstBody.Data)
	second := s.ReadDataAsMap(secondBody.Data)

	s.Equal(expectedKey, second["key"], "Idempotent retry must return the same object key as the first complete")
	s.Equal(first["originalFilename"], second["originalFilename"],
		"Idempotent retry must echo the same originalFilename")
	s.Equal(first["size"], second["size"],
		"Idempotent retry must report the same assembled object size")
}

func TestStorageResource(t *testing.T) {
	suite.Run(t, new(StorageResourceTestSuite))
}

package storage_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	istorage "github.com/coldsmirk/vef-framework-go/internal/storage"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// StorageResourceTestSuite tests the storage API resource functionality.
// Tests cover file upload, download, presigned URLs, object metadata, and listing operations.
type StorageResourceTestSuite struct {
	apptest.Suite

	ctx            context.Context
	minioContainer *testx.MinIOContainer
	service        storage.Service
	token          string

	testBucketName  string
	testObjectKey   string
	testObjectData  []byte
	testContentType string
}

// SetupSuite runs once before all tests in the suite.
func (s *StorageResourceTestSuite) SetupSuite() {
	s.T().Log("Setting up StorageResourceTestSuite - starting MinIO container and test app")

	s.ctx = context.Background()
	s.testBucketName = testx.TestMinIOBucket
	s.testObjectKey = "test-upload.txt"
	s.testObjectData = []byte("Hello, Storage API Test!")
	s.testContentType = "text/plain"

	s.minioContainer = testx.NewMinIOContainer(s.ctx, s.T())

	s.setupTestApp()

	reader := bytes.NewReader(s.testObjectData)
	_, err := s.service.PutObject(s.ctx, storage.PutObjectOptions{
		Key:         s.testObjectKey,
		Reader:      reader,
		Size:        int64(len(s.testObjectData)),
		ContentType: s.testContentType,
		Metadata: map[string]string{
			storage.MetadataKeyOriginalFilename: "test.txt",
		},
	})
	s.Require().NoError(err, "Should upload test object for read operations")

	s.T().Log("StorageResourceTestSuite setup complete - MinIO and test app ready")
}

// TearDownSuite runs once after all tests in the suite.
func (s *StorageResourceTestSuite) TearDownSuite() {
	s.T().Log("Tearing down StorageResourceTestSuite")
	s.TearDownApp()
	s.T().Log("StorageResourceTestSuite teardown complete")
}

func (s *StorageResourceTestSuite) setupTestApp() {
	// Create MinIO config with bucket
	minioConfig := *s.minioContainer.MinIO

	s.SetupApp(
		// Replace storage config with test values
		fx.Replace(
			&config.DataSourceConfig{
				Kind: "sqlite",
			},
			&config.StorageConfig{
				Provider:    "minio",
				AutoMigrate: true,
				MinIO:       minioConfig,
			},
			&security.JWTConfig{
				Secret:   security.DefaultJWTSecret,
				Audience: "test_app",
			},
		),
		fx.Populate(&s.service),
	)

	s.token = s.GenerateToken(&security.Principal{
		ID:   "test-admin",
		Name: "admin",
	})
}

// Helper methods for making API requests and reading responses

func (s *StorageResourceTestSuite) makeMultipartRequest(params map[string]string, fieldName, fileName string, fileContent []byte) *http.Response {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for key, value := range params {
		_ = writer.WriteField(key, value)
	}

	if fieldName != "" && fileName != "" {
		part, err := writer.CreateFormFile(fieldName, fileName)
		s.Require().NoError(err, "Should create form file")
		_, err = part.Write(fileContent)
		s.Require().NoError(err, "Should write file content")
	}

	err := writer.Close()
	s.Require().NoError(err, "Should close multipart writer")

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api", body)
	req.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())
	req.Header.Set(fiber.HeaderAuthorization, security.AuthSchemeBearer+" "+s.token)

	resp, err := s.App.Test(req)
	s.Require().NoError(err, "API request should not fail")

	return resp
}

// Test Cases

func (s *StorageResourceTestSuite) TestUpload() {
	s.Run("Success", func() {
		uploadData := []byte("Uploaded via API")

		params := map[string]string{
			"resource": "sys/storage",
			"action":   "upload",
			"version":  "v1",
		}

		resp := s.makeMultipartRequest(params, "file", "test.txt", uploadData)

		s.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.True(body.IsOk(), "Upload should succeed")
		s.Equal(i18n.T(result.OkMessage), body.Message, "Should return success message")

		data := s.ReadDataAsMap(body.Data)
		s.Equal(s.testBucketName, data["bucket"], "Bucket should match test bucket")
		s.NotEmpty(data["key"], "Key should not be empty")
		s.Contains(data["key"], ".txt", "Key should preserve file extension")
		s.NotEmpty(data["eTag"], "ETag should not be empty")
		s.NotZero(data["size"], "Size should not be zero")

		key := data["key"].(string)
		parts := strings.Split(key, "/")
		s.GreaterOrEqual(len(parts), 4, "Key should have date-based path structure")
		s.True(strings.HasSuffix(key, ".txt"), "Key should end with .txt")

		reader, err := s.service.GetObject(s.ctx, storage.GetObjectOptions{
			Key: key,
		})
		s.Require().NoError(err, "Should retrieve uploaded file")

		defer reader.Close()

		content, err := io.ReadAll(reader)
		s.Require().NoError(err, "Should read file content")
		s.Equal(uploadData, content, "File content should match uploaded data")

		info, err := s.service.StatObject(s.ctx, storage.StatObjectOptions{
			Key: key,
		})
		s.Require().NoError(err, "Should get file metadata")
		s.NotNil(info.Metadata, "Metadata should not be nil")
		s.Equal("test.txt", info.Metadata[storage.MetadataKeyOriginalFilename], "Original filename should be preserved in metadata")
	})

	s.Run("MissingFile", func() {
		params := map[string]string{
			"resource": "sys/storage",
			"action":   "upload",
			"version":  "v1",
		}

		resp := s.makeMultipartRequest(params, "", "", nil)

		s.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.False(body.IsOk(), "Upload should fail without file")
	})

	s.Run("WithJSON", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/storage",
				Action:   "upload",
				Version:  "v1",
			},
			Params: map[string]any{
				"key": "test.txt",
			},
		}, s.token)

		s.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.False(body.IsOk(), "Upload should fail with JSON request")
	})
}

func (s *StorageResourceTestSuite) TestGetPresignedURL() {
	s.Run("ForDownload", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/storage",
				Action:   "get_presigned_url",
				Version:  "v1",
			},
			Params: map[string]any{
				"key":     s.testObjectKey,
				"expires": 3600,
				"method":  "GET",
			},
		}, s.token)

		s.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.True(body.IsOk(), "Presigned URL generation should succeed")

		data := s.ReadDataAsMap(body.Data)
		url, ok := data["url"].(string)
		s.True(ok, "URL should be a string")
		s.NotEmpty(url, "URL should not be empty")
		s.Contains(url, s.testBucketName, "URL should contain bucket name")
		s.Contains(url, s.testObjectKey, "URL should contain object key")

		downloadReq, err := http.NewRequestWithContext(s.ctx, http.MethodGet, url, nil)
		s.Require().NoError(err, "Should create download request")

		downloadResp, err := http.DefaultClient.Do(downloadReq)
		s.Require().NoError(err, "Should execute download request")

		defer downloadResp.Body.Close()

		s.Equal(http.StatusOK, downloadResp.StatusCode, "Download should succeed")
		content, err := io.ReadAll(downloadResp.Body)
		s.Require().NoError(err, "Should read downloaded content")
		s.Equal(s.testObjectData, content, "Downloaded content should match original data")
	})

	s.Run("ForUpload", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/storage",
				Action:   "get_presigned_url",
				Version:  "v1",
			},
			Params: map[string]any{
				"key":     "presigned-upload.txt",
				"expires": 3600,
				"method":  "PUT",
			},
		}, s.token)

		s.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.True(body.IsOk(), "Presigned URL generation should succeed")

		data := s.ReadDataAsMap(body.Data)
		url, ok := data["url"].(string)
		s.True(ok, "URL should be a string")
		s.NotEmpty(url, "URL should not be empty")
		s.Contains(url, s.testBucketName, "URL should contain bucket name")

		uploadData := []byte("Uploaded via presigned URL")
		uploadReq, err := http.NewRequestWithContext(s.ctx, http.MethodPut, url, bytes.NewReader(uploadData))
		s.Require().NoError(err, "Should create upload request")

		uploadResp, err := http.DefaultClient.Do(uploadReq)
		s.Require().NoError(err, "Should execute upload request")

		defer uploadResp.Body.Close()

		s.Equal(http.StatusOK, uploadResp.StatusCode, "Upload should succeed")
	})

	s.Run("DefaultExpires", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/storage",
				Action:   "get_presigned_url",
				Version:  "v1",
			},
			Params: map[string]any{
				"key": s.testObjectKey,
			},
		}, s.token)

		s.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.True(body.IsOk(), "Presigned URL generation should succeed with default expiration")

		data := s.ReadDataAsMap(body.Data)
		s.Contains(data, "url", "Response should contain URL")
		s.NotEmpty(data["url"], "URL should not be empty")
	})

	s.Run("CustomExpiration", func() {
		customExpires := 7200

		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/storage",
				Action:   "get_presigned_url",
				Version:  "v1",
			},
			Params: map[string]any{
				"key":     s.testObjectKey,
				"expires": float64(customExpires),
			},
		}, s.token)

		s.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.True(body.IsOk(), "Presigned URL generation with custom expiration should succeed")

		data := s.ReadDataAsMap(body.Data)
		url := data["url"].(string)
		s.NotEmpty(url, "URL should not be empty")
		s.Contains(url, "X-Amz-Expires", "URL should contain expiration parameter")
	})
}

func (s *StorageResourceTestSuite) TestStatObject() {
	s.Run("Success", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/storage",
				Action:   "stat",
				Version:  "v1",
			},
			Params: map[string]any{
				"key": s.testObjectKey,
			},
		}, s.token)

		s.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.True(body.IsOk(), "Stat should succeed")

		data := s.ReadDataAsMap(body.Data)
		s.Equal(s.testBucketName, data["bucket"], "Bucket should match")
		s.Equal(s.testObjectKey, data["key"], "Key should match")
		s.NotEmpty(data["eTag"], "ETag should not be empty")
		s.NotZero(data["size"], "Size should not be zero")
		s.Equal(s.testContentType, data["contentType"], "Content type should match")
		s.NotZero(data["lastModified"], "Last modified should not be zero")
		s.Equal("test.txt", s.ReadDataAsMap(data["metadata"])[storage.MetadataKeyOriginalFilename], "Original filename should be in metadata")
	})

	s.Run("NotFound", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/storage",
				Action:   "stat",
				Version:  "v1",
			},
			Params: map[string]any{
				"key": "non-existent-key.txt",
			},
		}, s.token)

		s.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.False(body.IsOk(), "Stat should fail for non-existent object")
	})
}

func (s *StorageResourceTestSuite) TestListObjects() {
	s.Run("Success", func() {
		objects := map[string][]byte{
			"folder1/file1.txt": []byte("content1"),
			"folder1/file2.txt": []byte("content2"),
			"folder2/file3.txt": []byte("content3"),
		}

		for key, content := range objects {
			reader := bytes.NewReader(content)
			_, err := s.service.PutObject(s.ctx, storage.PutObjectOptions{
				Key:         key,
				Reader:      reader,
				Size:        int64(len(content)),
				ContentType: "text/plain",
			})
			s.Require().NoError(err, "Should upload test object")
		}

		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/storage",
				Action:   "list",
				Version:  "v1",
			},
			Params: map[string]any{
				"recursive": true,
			},
		}, s.token)

		s.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.True(body.IsOk(), "List should succeed")

		dataSlice, ok := body.Data.([]any)
		s.True(ok, "Data should be a slice")
		s.GreaterOrEqual(len(dataSlice), 4, "Should have at least 4 objects")
	})

	s.Run("WithPrefix", func() {
		objects := map[string][]byte{
			"prefix-test/file1.txt": []byte("content1"),
			"prefix-test/file2.txt": []byte("content2"),
			"other/file3.txt":       []byte("content3"),
		}

		for key, content := range objects {
			reader := bytes.NewReader(content)
			_, err := s.service.PutObject(s.ctx, storage.PutObjectOptions{
				Key:         key,
				Reader:      reader,
				Size:        int64(len(content)),
				ContentType: "text/plain",
			})
			s.Require().NoError(err, "Should upload test object")
		}

		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/storage",
				Action:   "list",
				Version:  "v1",
			},
			Params: map[string]any{
				"prefix":    "prefix-test/",
				"recursive": true,
			},
		}, s.token)

		s.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.True(body.IsOk(), "List with prefix should succeed")

		dataSlice, ok := body.Data.([]any)
		s.True(ok, "Data should be a slice")
		s.GreaterOrEqual(len(dataSlice), 2, "Should have at least 2 objects with prefix")

		for _, item := range dataSlice {
			obj := item.(map[string]any)
			key := obj["key"].(string)
			s.Contains(key, "prefix-test/", "All keys should contain prefix")
		}
	})

	s.Run("WithMaxKeys", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/storage",
				Action:   "list",
				Version:  "v1",
			},
			Params: map[string]any{
				"recursive": true,
				"maxKeys":   1,
			},
		}, s.token)

		s.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.True(body.IsOk(), "List with maxKeys should succeed")

		dataSlice, ok := body.Data.([]any)
		s.True(ok, "Data should be a slice")
		s.Equal(1, len(dataSlice), "MaxKeys should limit results to 1")
	})
}

func (s *StorageResourceTestSuite) TestUploadWithMetadata() {
	uploadData := []byte("Test with metadata")

	params := map[string]string{
		"resource": "sys/storage",
		"action":   "upload",
		"version":  "v1",
		"params":   `{"metadata":{"author":"test-suite","version":"1.0"}}`,
	}

	resp := s.makeMultipartRequest(params, "file", "test-metadata.txt", uploadData)

	s.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := s.ReadResult(resp)
	s.True(body.IsOk(), "Upload with metadata should succeed")

	data := s.ReadDataAsMap(body.Data)
	uploadKey := data["key"].(string)

	info, err := s.service.StatObject(s.ctx, storage.StatObjectOptions{
		Key: uploadKey,
	})
	s.Require().NoError(err, "Should get object metadata")
	s.NotNil(info.Metadata, "Metadata should not be nil")

	s.Equal("test-suite", info.Metadata["Author"], "Author metadata should match")
	s.Equal("1.0", info.Metadata["Version"], "Version metadata should match")
	s.Equal("test-metadata.txt", info.Metadata[storage.MetadataKeyOriginalFilename], "Original filename should be preserved")
}

func (s *StorageResourceTestSuite) TestUploadWithContentType() {
	uploadData := []byte(`{"test": "data"}`)

	params := map[string]string{
		"resource": "sys/storage",
		"action":   "upload",
		"version":  "v1",
		"params":   `{"contentType":"application/json"}`,
	}

	resp := s.makeMultipartRequest(params, "file", "test.json", uploadData)

	s.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := s.ReadResult(resp)
	s.True(body.IsOk(), "Upload with content type should succeed")

	data := s.ReadDataAsMap(body.Data)
	uploadKey := data["key"].(string)

	info, err := s.service.StatObject(s.ctx, storage.StatObjectOptions{
		Key: uploadKey,
	})
	s.Require().NoError(err, "Should get object metadata")
	s.Equal("application/json", info.ContentType, "Content type should match")
}

func (s *StorageResourceTestSuite) TestConcurrentUploads() {
	numUploads := 5
	done := make(chan bool, numUploads)

	for i := range numUploads {
		go func(index int) {
			defer func() { done <- true }()

			uploadData := fmt.Appendf(nil, "Concurrent upload %d", index)
			params := map[string]string{
				"resource": "sys/storage",
				"action":   "upload",
				"version":  "v1",
			}

			resp := s.makeMultipartRequest(params, "file", fmt.Sprintf("test%d.txt", index), uploadData)
			s.Equal(200, resp.StatusCode, "Concurrent upload should return 200 OK")

			body := s.ReadResult(resp)
			s.True(body.IsOk(), "Concurrent upload should succeed")
		}(i)
	}

	timeout := time.After(30 * time.Second)

	for range numUploads {
		select {
		case <-done:
		case <-timeout:
			s.Fail("Concurrent upload test timed out", "Should fail for expected reason")

			return
		}
	}
}

// ── New upload-claim API tests ──────────────────────────────────────────

// initUpload is a convenience helper for tests that need to drive the new
// init_upload action and assert against the returned shape.
func (s *StorageResourceTestSuite) initUpload(params istorage.InitUploadParams) (istorage.InitUploadResult, *http.Response) {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "sys/storage",
			Action:   "init_upload",
			Version:  "v1",
		},
		Params: map[string]any{
			"filename":    params.Filename,
			"size":        params.Size,
			"contentType": params.ContentType,
			"public":      params.Public,
			"metadata":    params.Metadata,
		},
	}, s.token)

	body := s.ReadResult(resp)
	if !body.IsOk() {
		return istorage.InitUploadResult{}, resp
	}

	m := s.ReadDataAsMap(body.Data)
	out := istorage.InitUploadResult{
		Mode:      asString(m["mode"]),
		Key:       asString(m["key"]),
		ClaimID:   asString(m["claimId"]),
		UploadURL: asString(m["uploadUrl"]),
		UploadID:  asString(m["uploadId"]),
		PartSize:  asInt64(m["partSize"]),
		PartCount: asInt(m["partCount"]),
	}

	if h, ok := m["requiredHeaders"].(map[string]any); ok {
		out.RequiredHeaders = make(map[string]string, len(h))
		for k, v := range h {
			out.RequiredHeaders[k] = asString(v)
		}
	}

	return out, resp
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}

	return ""
}

func asInt64(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	}

	return 0
}

func asInt(v any) int {
	return int(asInt64(v))
}

func (s *StorageResourceTestSuite) TestInitUploadDirectMode() {
	out, _ := s.initUpload(istorage.InitUploadParams{
		Filename:    "small.txt",
		Size:        1024,
		ContentType: "text/plain",
	})

	s.Equal("direct", out.Mode, "small files should pick the direct protocol")
	s.True(strings.HasPrefix(out.Key, "priv/"), "default visibility is private")
	s.True(strings.HasSuffix(out.Key, ".txt"), "extension preserved")
	s.NotEmpty(out.ClaimID)
	s.NotEmpty(out.UploadURL, "direct mode must return a presigned PUT URL")
	s.Equal("", out.UploadID, "direct mode does not open a multipart session")
}

func (s *StorageResourceTestSuite) TestInitUploadPublicPrefix() {
	out, _ := s.initUpload(istorage.InitUploadParams{
		Filename: "hero.png",
		Size:     1024,
		Public:   true,
	})

	s.True(strings.HasPrefix(out.Key, "pub/"), "public flag should produce pub/ prefix")
}

func (s *StorageResourceTestSuite) TestInitUploadMultipartMode() {
	// 10 MiB > default 5 MiB threshold, MinIO has Multipart+PresignedPart caps.
	out, _ := s.initUpload(istorage.InitUploadParams{
		Filename:    "video.mp4",
		Size:        10 * 1024 * 1024,
		ContentType: "video/mp4",
	})

	s.Equal("multipart", out.Mode, "size above threshold should pick multipart")
	s.NotEmpty(out.UploadID, "multipart mode opens a backend session")
	s.NotEmpty(out.ClaimID)
	s.GreaterOrEqual(out.PartSize, int64(5*1024*1024), "part size respects S3 minimum")
	s.GreaterOrEqual(out.PartCount, 1, "must compute at least one part")
}

func (s *StorageResourceTestSuite) TestSignPart() {
	out, _ := s.initUpload(istorage.InitUploadParams{
		Filename: "video.mp4",
		Size:     10 * 1024 * 1024,
	})
	s.Require().Equal("multipart", out.Mode)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "sys/storage",
			Action:   "sign_part",
			Version:  "v1",
		},
		Params: map[string]any{
			"claimId":    out.ClaimID,
			"partNumber": 1,
		},
	}, s.token)

	body := s.ReadResult(resp)
	s.Require().True(body.IsOk(), "sign_part should succeed: %s", body.Message)

	m := s.ReadDataAsMap(body.Data)
	s.NotEmpty(m["url"], "sign_part must return a non-empty URL")
}

func (s *StorageResourceTestSuite) TestSignPartRejectsNonMultipartClaim() {
	out, _ := s.initUpload(istorage.InitUploadParams{
		Filename: "small.txt",
		Size:     1024,
	})
	s.Require().Equal("direct", out.Mode)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "sys/storage",
			Action:   "sign_part",
			Version:  "v1",
		},
		Params: map[string]any{
			"claimId":    out.ClaimID,
			"partNumber": 1,
		},
	}, s.token)

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "sign_part on a direct-mode claim should fail")
}

func (s *StorageResourceTestSuite) TestCompleteUploadDirectMode() {
	out, _ := s.initUpload(istorage.InitUploadParams{
		Filename:    "doc.txt",
		Size:        12,
		ContentType: "text/plain",
	})
	s.Require().Equal("direct", out.Mode)

	// Simulate the client PUT by writing the object directly via the service
	// (the test environment doesn't push bytes through the presigned URL).
	_, err := s.service.PutObject(s.ctx, storage.PutObjectOptions{
		Key:    out.Key,
		Reader: bytes.NewReader([]byte("hello, world")),
		Size:   12,
	})
	s.Require().NoError(err)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "sys/storage",
			Action:   "complete_upload",
			Version:  "v1",
		},
		Params: map[string]any{
			"claimId": out.ClaimID,
		},
	}, s.token)

	body := s.ReadResult(resp)
	s.Require().True(body.IsOk(), "complete_upload should succeed: %s", body.Message)

	m := s.ReadDataAsMap(body.Data)
	s.Equal(out.Key, m["key"], "result should echo the final key")
	s.NotEmpty(m["eTag"])
}

func (s *StorageResourceTestSuite) TestAbortUploadHappyPath() {
	out, _ := s.initUpload(istorage.InitUploadParams{
		Filename: "video.mp4",
		Size:     10 * 1024 * 1024,
	})
	s.Require().Equal("multipart", out.Mode)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "sys/storage",
			Action:   "abort_upload",
			Version:  "v1",
		},
		Params: map[string]any{
			"claimId": out.ClaimID,
		},
	}, s.token)

	body := s.ReadResult(resp)
	s.True(body.IsOk(), "abort_upload should succeed: %s", body.Message)

	// A second abort on the same (now-deleted) claim is idempotent.
	resp = s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "sys/storage",
			Action:   "abort_upload",
			Version:  "v1",
		},
		Params: map[string]any{
			"claimId": out.ClaimID,
		},
	}, s.token)
	body = s.ReadResult(resp)
	s.True(body.IsOk(), "abort_upload is idempotent on missing claim")
}

// TestStorageResourceTestSuite runs the test suite.
func TestStorageResource(t *testing.T) {
	suite.Run(t, new(StorageResourceTestSuite))
}

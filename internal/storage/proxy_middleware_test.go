package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/coldsmirk/vef-framework-go/storage"
)

// MockStorageService is a mock implementation of storage.Service for testing.
type MockStorageService struct {
	mock.Mock
}

func (*MockStorageService) PutObject(context.Context, storage.PutObjectOptions) (*storage.ObjectInfo, error) {
	return nil, nil
}

func (m *MockStorageService) GetObject(_ context.Context, opts storage.GetObjectOptions) (io.ReadCloser, *storage.ObjectInfo, error) {
	args := m.Called(opts)

	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}

	var info *storage.ObjectInfo
	if args.Get(1) != nil {
		info = args.Get(1).(*storage.ObjectInfo)
	}

	return args.Get(0).(io.ReadCloser), info, args.Error(2)
}

func (*MockStorageService) DeleteObject(context.Context, storage.DeleteObjectOptions) error {
	return nil
}

func (*MockStorageService) DeleteObjects(context.Context, storage.DeleteObjectsOptions) error {
	return nil
}

func (*MockStorageService) CopyObject(context.Context, storage.CopyObjectOptions) (*storage.ObjectInfo, error) {
	return nil, nil
}

func (m *MockStorageService) StatObject(_ context.Context, opts storage.StatObjectOptions) (*storage.ObjectInfo, error) {
	args := m.Called(opts)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*storage.ObjectInfo), args.Error(1)
}

func (*MockStorageService) PartSize() int64   { return 0 }
func (*MockStorageService) MaxPartCount() int { return 0 }

func (*MockStorageService) InitMultipart(context.Context, storage.InitMultipartOptions) (*storage.MultipartSession, error) {
	return nil, nil
}

func (*MockStorageService) PutPart(context.Context, storage.PutPartOptions) (*storage.PartInfo, error) {
	return nil, nil
}

func (*MockStorageService) CompleteMultipart(context.Context, storage.CompleteMultipartOptions) (*storage.ObjectInfo, error) {
	return nil, nil
}

func (*MockStorageService) AbortMultipart(context.Context, storage.AbortMultipartOptions) error {
	return nil
}

// StubFileRegistry serves canned registry records, or fails outright so
// the proxy's best-effort filename resolution can be exercised.
type StubFileRegistry struct {
	records map[string]storage.FileRecord
	err     error
}

func (s *StubFileRegistry) Lookup(_ context.Context, keys []string) (map[string]storage.FileRecord, error) {
	if s.err != nil {
		return nil, s.err
	}

	found := make(map[string]storage.FileRecord, len(keys))

	for _, key := range keys {
		if record, ok := s.records[key]; ok {
			found[key] = record
		}
	}

	return found, nil
}

// registryWith returns a stub holding one record for key.
func registryWith(key, filename string) *StubFileRegistry {
	return &StubFileRegistry{
		records: map[string]storage.FileRecord{key: {Key: key, OriginalFilename: filename}},
	}
}

func TestProxyMiddleware(t *testing.T) {
	// Helper function to create a configured Fiber app with error handler
	createApp := func() *fiber.App {
		return fiber.New(fiber.Config{
			ErrorHandler: func(fiber.Ctx, error) error {
				// Return 200 for business errors (matching framework behavior)
				return nil
			},
		})
	}

	t.Run("SuccessfulFileDownload", func(t *testing.T) {
		mockService := new(MockStorageService)
		fileContent := []byte("test file content")

		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "pub/2025/01/15/test.jpg",
		}).Return(io.NopCloser(bytes.NewReader(fileContent)), &storage.ObjectInfo{
			ContentType: "image/jpeg",
			ETag:        "etag123",
			Size:        17,
		}, nil)

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL), new(StubFileRegistry))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/pub/2025/01/15/test.jpg", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "TestProxyMiddleware should complete without error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Public file should be accessible")
		assert.Equal(t, "image/jpeg", resp.Header.Get("Content-Type"), "Content-Type should match")
		assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"), "Must send nosniff to prevent MIME sniffing")
		assert.Equal(t, "public, max-age=3600, immutable", resp.Header.Get("Cache-Control"), "Public files get public cache")

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, fileContent, body, "Body should match uploaded content")

		mockService.AssertExpectations(t)
	})

	t.Run("AdvertisesTheOriginalFilename", func(t *testing.T) {
		mockService := new(MockStorageService)

		const key = "pub/2026/08/04/9f3c.pdf"

		mockService.On("GetObject", storage.GetObjectOptions{Key: key}).
			Return(io.NopCloser(bytes.NewReader([]byte("pdf"))), &storage.ObjectInfo{ContentType: "application/pdf"}, nil)

		app := createApp()
		NewProxyMiddleware(mockService, new(storage.DefaultFileACL), registryWith(key, "季度报告.pdf")).Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/"+key, nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "TestProxyMiddleware should complete without error")
		assert.Equal(t, `inline; filename*=utf-8''%E5%AD%A3%E5%BA%A6%E6%8A%A5%E5%91%8A.pdf`,
			resp.Header.Get(fiber.HeaderContentDisposition),
			"A recorded upload should be served under its original filename")

		mockService.AssertExpectations(t)
	})

	// Filename resolution is decoration; losing it must never cost the
	// caller their download.
	t.Run("RegistryFailureStillServesTheFile", func(t *testing.T) {
		mockService := new(MockStorageService)

		const key = "pub/2026/08/04/degraded.jpg"

		fileContent := []byte("jpeg bytes")

		mockService.On("GetObject", storage.GetObjectOptions{Key: key}).
			Return(io.NopCloser(bytes.NewReader(fileContent)), &storage.ObjectInfo{ContentType: "image/jpeg"}, nil)

		registry := &StubFileRegistry{err: errors.New("registry unavailable")}

		app := createApp()
		NewProxyMiddleware(mockService, new(storage.DefaultFileACL), registry).Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/"+key, nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "TestProxyMiddleware should complete without error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "A registry failure must not fail the download")
		assert.Empty(t, resp.Header.Get(fiber.HeaderContentDisposition),
			"Without a resolved name the header is omitted rather than guessed")

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, fileContent, body, "The body should still stream in full")

		mockService.AssertExpectations(t)
	})

	t.Run("FileNotFound", func(t *testing.T) {
		mockService := new(MockStorageService)

		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "pub/nonexistent.jpg",
		}).Return(nil, nil, storage.ErrObjectNotFound)

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL), new(StubFileRegistry))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/pub/nonexistent.jpg", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "TestProxyMiddleware should complete without error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Framework returns 200 with error body")

		mockService.AssertExpectations(t)
	})

	t.Run("PrivateKeyDeniedByDefaultACL", func(t *testing.T) {
		mockService := new(MockStorageService)

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL), new(StubFileRegistry))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/priv/2025/01/15/secret.bin", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "TestProxyMiddleware should complete without error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Framework returns 200 with error body for access denied")

		// GetObject should NOT be called — ACL rejects before reaching backend
		mockService.AssertNotCalled(t, "GetObject")
	})

	t.Run("URLEncodedFileKey", func(t *testing.T) {
		mockService := new(MockStorageService)
		fileContent := []byte("test content")

		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "pub/测试文件.jpg",
		}).Return(io.NopCloser(bytes.NewReader(fileContent)), &storage.ObjectInfo{
			ContentType: "image/jpeg",
		}, nil)

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL), new(StubFileRegistry))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/pub/%E6%B5%8B%E8%AF%95%E6%96%87%E4%BB%B6.jpg", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "TestProxyMiddleware should complete without error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Public file proxy should return 200 OK")

		mockService.AssertExpectations(t)
	})

	t.Run("StorageError", func(t *testing.T) {
		mockService := new(MockStorageService)

		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "pub/error.jpg",
		}).Return(nil, nil, errors.New("storage error"))

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL), new(StubFileRegistry))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/pub/error.jpg", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "TestProxyMiddleware should complete without error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Framework returns 200 with error body")

		mockService.AssertExpectations(t)
	})

	t.Run("ContentTypeFallbackWhenInfoNil", func(t *testing.T) {
		mockService := new(MockStorageService)
		fileContent := []byte("test content")

		// A nil ObjectInfo models a backend that opened the body but could
		// not resolve metadata (best-effort contract). The proxy must still
		// stream the body and fall back to the extension for Content-Type.
		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "pub/test.png",
		}).Return(io.NopCloser(bytes.NewReader(fileContent)), nil, nil)

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL), new(StubFileRegistry))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/pub/test.png", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "TestProxyMiddleware should complete without error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Proxy should return 200 OK when object metadata is unavailable")
		assert.Equal(t, "image/png", resp.Header.Get("Content-Type"), "Should fallback to extension")

		mockService.AssertExpectations(t)
	})

	t.Run("ContentTypeFallbackWhenEmpty", func(t *testing.T) {
		mockService := new(MockStorageService)
		fileContent := []byte("test content")

		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "pub/document.pdf",
		}).Return(io.NopCloser(bytes.NewReader(fileContent)), &storage.ObjectInfo{
			ContentType: "",
			ETag:        "etag456",
		}, nil)

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL), new(StubFileRegistry))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/pub/document.pdf", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "TestProxyMiddleware should complete without error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Proxy should return 200 OK when content type is empty")
		assert.Equal(t, "application/pdf", resp.Header.Get("Content-Type"), "Should fallback to extension")
		assert.Equal(t, "etag456", resp.Header.Get("ETag"), "ETag should be set")

		mockService.AssertExpectations(t)
	})
}

// TestIsValidObjectKey covers the security-relevant behavior of the
// isValidObjectKey helper. The function guards the proxy handler against
// path-traversal and other filesystem-level exploits, so rejection cases
// are emphasized.
func TestIsValidObjectKey(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		valid bool
	}{
		// ── rejection cases (security boundary) ──────────────────────
		{
			name:  "EmptyKey",
			key:   "",
			valid: false,
		},
		{
			name:  "AbsolutePath",
			key:   "/etc/passwd",
			valid: false,
		},
		{
			name:  "DotDotSegmentTraversal",
			key:   "../secret",
			valid: false,
		},
		{
			name:  "DotDotInMiddle",
			key:   "pub/../priv/secret.bin",
			valid: false,
		},
		{
			name:  "DotDotAtEnd",
			key:   "pub/2026/..",
			valid: false,
		},
		{
			name:  "DotDotOnly",
			key:   "..",
			valid: false,
		},
		{
			name:  "NULByte",
			key:   "pub/fi\x00le.jpg",
			valid: false,
		},
		{
			name:  "Backslash",
			key:   "pub\\windows\\path",
			valid: false,
		},
		{
			name:  "TrailingSlash",
			key:   "pub/2026/01/15/",
			valid: false,
		},
		{
			name:  "DoubleSlash",
			key:   "pub//file.jpg",
			valid: false,
		},
		{
			name:  "LeadingDotDotAbsolute",
			key:   "/../escape",
			valid: false,
		},

		// ── acceptance cases ─────────────────────────────────────────
		{
			name:  "SimplePublicKey",
			key:   "pub/2026/01/15/photo.jpg",
			valid: true,
		},
		{
			name:  "SimplePrivateKey",
			key:   "priv/2026/01/15/report.pdf",
			valid: true,
		},
		{
			name:  "SingleSegment",
			key:   "file.bin",
			valid: true,
		},
		{
			name:  "DeepPath",
			key:   "pub/a/b/c/d/e/f.png",
			valid: true,
		},
		{
			name:  "KeyWithDotInName",
			key:   "pub/my.file.v2.jpg",
			valid: true,
		},
		{
			name:  "KeyWithUnicode",
			key:   "pub/测试文件.jpg",
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidObjectKey(tt.key)
			if tt.valid {
				assert.True(t, got, "isValidObjectKey(%q) should return true for a valid key", tt.key)
			} else {
				assert.False(t, got, "isValidObjectKey(%q) should return false to reject unsafe key", tt.key)
			}
		})
	}
}

// TestDetectContentType covers the detectContentType helper which picks
// a content type from the ObjectInfo stat (when available) and always
// runs the result through sanitizeContentType to prevent stored-XSS.
func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name     string
		stat     *storage.ObjectInfo
		key      string
		expected string
	}{
		{
			name:     "StatContentTypePreferred",
			stat:     &storage.ObjectInfo{ContentType: "image/jpeg"},
			key:      "pub/file.jpg",
			expected: "image/jpeg",
		},
		{
			name:     "StatContentTypeEmptyFallsBackToExtension",
			stat:     &storage.ObjectInfo{ContentType: ""},
			key:      "pub/file.png",
			expected: "image/png",
		},
		{
			name:     "NilStatFallsBackToExtension",
			stat:     nil,
			key:      "pub/file.pdf",
			expected: "application/pdf",
		},
		{
			name:     "UnsafeContentTypeSanitizedToOctetStream",
			stat:     &storage.ObjectInfo{ContentType: "text/html"},
			key:      "pub/page.html",
			expected: "application/octet-stream",
		},
		{
			name:     "JavaScriptSanitizedToOctetStream",
			stat:     &storage.ObjectInfo{ContentType: "application/javascript"},
			key:      "pub/script.js",
			expected: "application/octet-stream",
		},
		{
			name:     "UnknownExtensionNoStatFallsToOctetStream",
			stat:     nil,
			key:      "pub/file.xyz123",
			expected: "application/octet-stream",
		},
		{
			name:     "VideoMimePassesThrough",
			stat:     &storage.ObjectInfo{ContentType: "video/mp4"},
			key:      "pub/clip.mp4",
			expected: "video/mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectContentType(tt.stat, tt.key)
			assert.Equal(t, tt.expected, got,
				"detectContentType(stat, %q) should return %q", tt.key, tt.expected)
		})
	}
}

// TestContentDisposition covers the header the download proxy builds
// from the registry's original filename — the whole reason the registry
// is consulted on the read path.
func TestContentDisposition(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		filename    string
		expected    string
	}{
		{
			name:        "NoFilenameOmitsTheHeader",
			contentType: "image/jpeg",
			filename:    "",
			expected:    "",
		},
		{
			name:        "RenderableMediaStaysInline",
			contentType: "image/jpeg",
			filename:    "photo.jpg",
			expected:    `inline; filename=photo.jpg`,
		},
		{
			name:        "OctetStreamIsAnAttachment",
			contentType: "application/octet-stream",
			filename:    "report.pdf",
			expected:    `attachment; filename=report.pdf`,
		},
		{
			// The upload sanitizer collapses anything unsafe to
			// octet-stream, but the disposition must not depend on that.
			name:        "UnsafeTypeIsAnAttachment",
			contentType: "text/html",
			filename:    "page.html",
			expected:    `attachment; filename=page.html`,
		},
		{
			name:        "PDFStaysInlineSoPreviewsKeepWorking",
			contentType: "application/pdf",
			filename:    "report.pdf",
			expected:    `inline; filename=report.pdf`,
		},
		{
			name:        "ArchiveIsAnAttachment",
			contentType: "application/zip",
			filename:    "bundle.zip",
			expected:    `attachment; filename=bundle.zip`,
		},
		{
			name:        "NonASCIIUsesTheExtendedForm",
			contentType: "application/octet-stream",
			filename:    "季度报告.pdf",
			expected:    `attachment; filename*=utf-8''%E5%AD%A3%E5%BA%A6%E6%8A%A5%E5%91%8A.pdf`,
		},
		{
			name:        "SpacesAreQuoted",
			contentType: "application/octet-stream",
			filename:    "my report (1).pdf",
			expected:    `attachment; filename="my report (1).pdf"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, contentDisposition(tt.contentType, tt.filename),
				"contentDisposition(%q, %q) should render the RFC 6266 header", tt.contentType, tt.filename)
		})
	}

	// A filename can never inject a header: sanitizeFilename rejects
	// control characters at upload, and the encoder percent-escapes
	// anything left over.
	t.Run("NeverEmitsRawCRLF", func(t *testing.T) {
		got := contentDisposition("application/octet-stream", "evil\r\nX-Injected: yes.pdf")
		assert.NotContains(t, got, "\r", "The rendered header must not carry a raw CR")
		assert.NotContains(t, got, "\n", "The rendered header must not carry a raw LF")
	})
}

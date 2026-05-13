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

func (m *MockStorageService) GetObject(_ context.Context, opts storage.GetObjectOptions) (io.ReadCloser, error) {
	args := m.Called(opts)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (*MockStorageService) DeleteObject(context.Context, storage.DeleteObjectOptions) error {
	return nil
}

func (*MockStorageService) DeleteObjects(context.Context, storage.DeleteObjectsOptions) error {
	return nil
}

func (*MockStorageService) ListObjects(context.Context, storage.ListObjectsOptions) ([]storage.ObjectInfo, error) {
	return nil, nil
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

// TestProxyMiddleware tests proxy middleware functionality.
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
		}).Return(io.NopCloser(bytes.NewReader(fileContent)), nil)

		mockService.On("StatObject", storage.StatObjectOptions{
			Key: "pub/2025/01/15/test.jpg",
		}).Return(&storage.ObjectInfo{
			ContentType: "image/jpeg",
			ETag:        "etag123",
			Size:        17,
		}, nil)

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/pub/2025/01/15/test.jpg", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Public file should be accessible")
		assert.Equal(t, "image/jpeg", resp.Header.Get("Content-Type"), "Content-Type should match")
		assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"), "Must send nosniff to prevent MIME sniffing")
		assert.Equal(t, "public, max-age=3600, immutable", resp.Header.Get("Cache-Control"), "Public files get public cache")

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, fileContent, body, "Body should match uploaded content")

		mockService.AssertExpectations(t)
	})

	t.Run("FileNotFound", func(t *testing.T) {
		mockService := new(MockStorageService)

		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "pub/nonexistent.jpg",
		}).Return(nil, storage.ErrObjectNotFound)

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/pub/nonexistent.jpg", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Framework returns 200 with error body")

		mockService.AssertExpectations(t)
	})

	t.Run("PrivateKeyDeniedByDefaultACL", func(t *testing.T) {
		mockService := new(MockStorageService)

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/priv/2025/01/15/secret.bin", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Framework returns 200 with error body for access denied")

		// GetObject should NOT be called — ACL rejects before reaching backend
		mockService.AssertNotCalled(t, "GetObject")
	})

	t.Run("URLEncodedFileKey", func(t *testing.T) {
		mockService := new(MockStorageService)
		fileContent := []byte("test content")

		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "pub/测试文件.jpg",
		}).Return(io.NopCloser(bytes.NewReader(fileContent)), nil)

		mockService.On("StatObject", storage.StatObjectOptions{
			Key: "pub/测试文件.jpg",
		}).Return(&storage.ObjectInfo{
			ContentType: "image/jpeg",
		}, nil)

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/pub/%E6%B5%8B%E8%AF%95%E6%96%87%E4%BB%B6.jpg", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should succeed for pub/ key")

		mockService.AssertExpectations(t)
	})

	t.Run("StorageError", func(t *testing.T) {
		mockService := new(MockStorageService)

		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "pub/error.jpg",
		}).Return(nil, errors.New("storage error"))

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/pub/error.jpg", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Framework returns 200 with error body")

		mockService.AssertExpectations(t)
	})

	t.Run("ContentTypeFallbackWhenStatFails", func(t *testing.T) {
		mockService := new(MockStorageService)
		fileContent := []byte("test content")

		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "pub/test.png",
		}).Return(io.NopCloser(bytes.NewReader(fileContent)), nil)

		mockService.On("StatObject", storage.StatObjectOptions{
			Key: "pub/test.png",
		}).Return(nil, errors.New("stat failed"))

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/pub/test.png", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should succeed")
		assert.Equal(t, "image/png", resp.Header.Get("Content-Type"), "Should fallback to extension")

		mockService.AssertExpectations(t)
	})

	t.Run("ContentTypeFallbackWhenEmpty", func(t *testing.T) {
		mockService := new(MockStorageService)
		fileContent := []byte("test content")

		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "pub/document.pdf",
		}).Return(io.NopCloser(bytes.NewReader(fileContent)), nil)

		mockService.On("StatObject", storage.StatObjectOptions{
			Key: "pub/document.pdf",
		}).Return(&storage.ObjectInfo{
			ContentType: "",
			ETag:        "etag456",
		}, nil)

		app := createApp()
		middleware := NewProxyMiddleware(mockService, new(storage.DefaultFileACL))
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/pub/document.pdf", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should succeed")
		assert.Equal(t, "application/pdf", resp.Header.Get("Content-Type"), "Should fallback to extension")
		assert.Equal(t, "etag456", resp.Header.Get("ETag"), "ETag should be set")

		mockService.AssertExpectations(t)
	})
}

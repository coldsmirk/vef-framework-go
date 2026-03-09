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

func (*MockStorageService) PutObject(_ context.Context, _ storage.PutObjectOptions) (*storage.ObjectInfo, error) {
	return nil, nil
}

func (m *MockStorageService) GetObject(_ context.Context, opts storage.GetObjectOptions) (io.ReadCloser, error) {
	args := m.Called(opts)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (*MockStorageService) DeleteObject(_ context.Context, _ storage.DeleteObjectOptions) error {
	return nil
}

func (*MockStorageService) DeleteObjects(_ context.Context, _ storage.DeleteObjectsOptions) error {
	return nil
}

func (*MockStorageService) ListObjects(_ context.Context, _ storage.ListObjectsOptions) ([]storage.ObjectInfo, error) {
	return nil, nil
}

func (*MockStorageService) GetPresignedURL(_ context.Context, _ storage.PresignedURLOptions) (string, error) {
	return "", nil
}

func (*MockStorageService) CopyObject(_ context.Context, _ storage.CopyObjectOptions) (*storage.ObjectInfo, error) {
	return nil, nil
}

func (*MockStorageService) MoveObject(_ context.Context, _ storage.MoveObjectOptions) (*storage.ObjectInfo, error) {
	return nil, nil
}

func (m *MockStorageService) StatObject(_ context.Context, opts storage.StatObjectOptions) (*storage.ObjectInfo, error) {
	args := m.Called(opts)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*storage.ObjectInfo), args.Error(1)
}

func (*MockStorageService) PromoteObject(_ context.Context, _ string) (*storage.ObjectInfo, error) {
	return nil, nil
}

// TestProxyMiddleware tests proxy middleware functionality.
func TestProxyMiddleware(t *testing.T) {
	// Helper function to create a configured Fiber app with error handler
	createApp := func() *fiber.App {
		return fiber.New(fiber.Config{
			ErrorHandler: func(_ fiber.Ctx, _ error) error {
				// Return 200 for business errors (matching framework behavior)
				return nil
			},
		})
	}

	t.Run("SuccessfulFileDownload", func(t *testing.T) {
		mockService := new(MockStorageService)
		fileContent := []byte("test file content")

		// Setup expectations
		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "temp/2025/01/15/test.jpg",
		}).Return(io.NopCloser(bytes.NewReader(fileContent)), nil)

		mockService.On("StatObject", storage.StatObjectOptions{
			Key: "temp/2025/01/15/test.jpg",
		}).Return(&storage.ObjectInfo{
			ContentType: "image/jpeg",
			ETag:        "etag123",
		}, nil)

		app := createApp()
		middleware := NewProxyMiddleware(mockService)
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/temp/2025/01/15/test.jpg", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should equal expected value")
		assert.Equal(t, "image/jpeg", resp.Header.Get("Content-Type"), "Should equal expected value")

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, fileContent, body, "Should equal expected value")

		mockService.AssertExpectations(t)
	})

	t.Run("FileNotFound", func(t *testing.T) {
		mockService := new(MockStorageService)

		// Setup expectations
		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "nonexistent.jpg",
		}).Return(nil, storage.ErrObjectNotFound)

		app := createApp()
		middleware := NewProxyMiddleware(mockService)
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/nonexistent.jpg", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should equal expected value")

		mockService.AssertExpectations(t)
	})

	t.Run("EmptyFileKey", func(t *testing.T) {
		app := createApp()
		middleware := NewProxyMiddleware(nil)
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should equal expected value")
	})

	t.Run("URLEncodedFileKey", func(t *testing.T) {
		mockService := new(MockStorageService)
		fileContent := []byte("test content")

		// Setup expectations - the key should be decoded
		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "temp/测试文件.jpg",
		}).Return(io.NopCloser(bytes.NewReader(fileContent)), nil)

		mockService.On("StatObject", storage.StatObjectOptions{
			Key: "temp/测试文件.jpg",
		}).Return(&storage.ObjectInfo{
			ContentType: "image/jpeg",
		}, nil)

		app := createApp()
		middleware := NewProxyMiddleware(mockService)
		middleware.Apply(app)

		// URL encode the Chinese characters
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/temp/%E6%B5%8B%E8%AF%95%E6%96%87%E4%BB%B6.jpg", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should equal expected value")

		mockService.AssertExpectations(t)
	})

	t.Run("StorageError", func(t *testing.T) {
		mockService := new(MockStorageService)

		// Setup expectations
		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "error.jpg",
		}).Return(nil, errors.New("storage error"))

		app := createApp()
		middleware := NewProxyMiddleware(mockService)
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/error.jpg", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should equal expected value")

		mockService.AssertExpectations(t)
	})

	t.Run("ContentTypeFallbackWhenStatFails", func(t *testing.T) {
		mockService := new(MockStorageService)
		fileContent := []byte("test content")

		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "test.png",
		}).Return(io.NopCloser(bytes.NewReader(fileContent)), nil)

		// StatObject fails - should fallback to extension-based detection
		mockService.On("StatObject", storage.StatObjectOptions{
			Key: "test.png",
		}).Return(nil, errors.New("stat failed"))

		app := createApp()
		middleware := NewProxyMiddleware(mockService)
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/test.png", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should equal expected value")
		assert.Equal(t, "image/png", resp.Header.Get("Content-Type"), "Should equal expected value")

		mockService.AssertExpectations(t)
	})

	t.Run("ContentTypeFallbackWhenEmpty", func(t *testing.T) {
		mockService := new(MockStorageService)
		fileContent := []byte("test content")

		mockService.On("GetObject", storage.GetObjectOptions{
			Key: "document.pdf",
		}).Return(io.NopCloser(bytes.NewReader(fileContent)), nil)

		// StatObject succeeds but ContentType is empty
		mockService.On("StatObject", storage.StatObjectOptions{
			Key: "document.pdf",
		}).Return(&storage.ObjectInfo{
			ContentType: "",
			ETag:        "etag456",
		}, nil)

		app := createApp()
		middleware := NewProxyMiddleware(mockService)
		middleware.Apply(app)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/storage/files/document.pdf", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err, "Should not return error")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should equal expected value")
		assert.Equal(t, "application/pdf", resp.Header.Get("Content-Type"), "Should equal expected value")
		assert.Equal(t, "etag456", resp.Header.Get("ETag"), "Should equal expected value")

		mockService.AssertExpectations(t)
	})
}

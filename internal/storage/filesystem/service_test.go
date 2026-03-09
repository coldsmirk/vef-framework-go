package filesystem

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/storage"
)

func setupTestService(t *testing.T) (storage.Service, func()) {
	tempDir := t.TempDir()

	service, err := New(config.FilesystemConfig{Root: tempDir})
	require.NoError(t, err, "Should not return error")

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	return service, cleanup
}

// TestFilesystemService tests filesystem service functionality.
func TestFilesystemService(t *testing.T) {
	ctx := context.Background()

	service, cleanup := setupTestService(t)
	defer cleanup()

	t.Run("PutObject", func(t *testing.T) {
		data := []byte("Hello, Filesystem Storage!")
		reader := bytes.NewReader(data)

		info, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:         "test.txt",
			Reader:      reader,
			Size:        int64(len(data)),
			ContentType: "text/plain",
			Metadata: map[string]string{
				"author": "test",
			},
		})

		require.NoError(t, err, "Should not return error")
		assert.NotNil(t, info, "Should not be nil")
		assert.Equal(t, "test.txt", info.Key, "Should equal expected value")
		assert.Equal(t, int64(len(data)), info.Size, "Should equal expected value")
		assert.Equal(t, "text/plain", info.ContentType, "Should equal expected value")
	})

	t.Run("GetObjectSuccess", func(t *testing.T) {
		expectedData := []byte("Hello, Filesystem Storage!")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test.txt",
		})

		require.NoError(t, err, "Should not return error")

		require.NotNil(t, reader, "Should not be nil")
		defer reader.Close()

		data, err := io.ReadAll(reader)
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, expectedData, data, "Should equal expected value")
	})

	t.Run("GetObjectNotFound", func(t *testing.T) {
		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "nonexistent.txt",
		})

		assert.Error(t, err, "Should return error")
		assert.Nil(t, reader, "Should be nil")
		assert.Equal(t, storage.ErrObjectNotFound, err, "Should equal expected value")
	})

	t.Run("StatObject", func(t *testing.T) {
		info, err := service.StatObject(ctx, storage.StatObjectOptions{
			Key: "test.txt",
		})

		require.NoError(t, err, "Should not return error")
		assert.NotNil(t, info, "Should not be nil")
		assert.Equal(t, "test.txt", info.Key, "Should equal expected value")
		assert.Greater(t, info.Size, int64(0), "Should be greater")
	})

	t.Run("CopyObject", func(t *testing.T) {
		info, err := service.CopyObject(ctx, storage.CopyObjectOptions{
			SourceKey: "test.txt",
			DestKey:   "test-copy.txt",
		})

		require.NoError(t, err, "Should not return error")
		assert.NotNil(t, info, "Should not be nil")
		assert.Equal(t, "test-copy.txt", info.Key, "Should equal expected value")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test-copy.txt",
		})
		require.NoError(t, err, "Should not return error")

		defer reader.Close()

		data, err := io.ReadAll(reader)
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, []byte("Hello, Filesystem Storage!"), data, "Should equal expected value")
	})

	t.Run("MoveObject", func(t *testing.T) {
		info, err := service.MoveObject(ctx, storage.MoveObjectOptions{
			CopyObjectOptions: storage.CopyObjectOptions{
				SourceKey: "test-copy.txt",
				DestKey:   "test-moved.txt",
			},
		})

		require.NoError(t, err, "Should not return error")
		assert.NotNil(t, info, "Should not be nil")
		assert.Equal(t, "test-moved.txt", info.Key, "Should equal expected value")

		_, err = service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test-copy.txt",
		})
		assert.Error(t, err, "Should return error")
		assert.Equal(t, storage.ErrObjectNotFound, err, "Should equal expected value")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test-moved.txt",
		})
		require.NoError(t, err, "Should not return error")

		defer reader.Close()
	})

	t.Run("ListObjects", func(t *testing.T) {
		_, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    "folder/file1.txt",
			Reader: bytes.NewReader([]byte("file1")),
			Size:   5,
		})
		require.NoError(t, err, "Should not return error")

		_, err = service.PutObject(ctx, storage.PutObjectOptions{
			Key:    "folder/file2.txt",
			Reader: bytes.NewReader([]byte("file2")),
			Size:   5,
		})
		require.NoError(t, err, "Should not return error")

		t.Run("ListAllObjects", func(t *testing.T) {
			objects, err := service.ListObjects(ctx, storage.ListObjectsOptions{
				Recursive: true,
			})

			require.NoError(t, err, "Should not return error")
			assert.GreaterOrEqual(t, len(objects), 3, "Should be greater or equal")
		})

		t.Run("ListWithPrefix", func(t *testing.T) {
			objects, err := service.ListObjects(ctx, storage.ListObjectsOptions{
				Prefix:    "folder/",
				Recursive: true,
			})

			require.NoError(t, err, "Should not return error")
			assert.Equal(t, 2, len(objects), "Should equal expected value")
		})

		t.Run("ListNonRecursive", func(t *testing.T) {
			objects, err := service.ListObjects(ctx, storage.ListObjectsOptions{
				Recursive: false,
			})

			require.NoError(t, err, "Should not return error")

			for _, obj := range objects {
				assert.NotContains(t, obj.Key, "folder/", "Should not contain value")
			}
		})
	})

	t.Run("PromoteObject", func(t *testing.T) {
		tempKey := storage.TempPrefix + "2025/01/15/test.txt"
		_, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    tempKey,
			Reader: bytes.NewReader([]byte("temp content")),
			Size:   12,
		})
		require.NoError(t, err, "Should not return error")

		info, err := service.PromoteObject(ctx, tempKey)
		require.NoError(t, err, "Should not return error")
		assert.NotNil(t, info, "Should not be nil")
		assert.Equal(t, "2025/01/15/test.txt", info.Key, "Should equal expected value")

		_, err = service.GetObject(ctx, storage.GetObjectOptions{Key: tempKey})
		assert.Error(t, err, "Should return error")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "2025/01/15/test.txt",
		})
		require.NoError(t, err, "Should not return error")

		defer reader.Close()
	})

	t.Run("DeleteObject", func(t *testing.T) {
		err := service.DeleteObject(ctx, storage.DeleteObjectOptions{
			Key: "test.txt",
		})

		assert.NoError(t, err, "Should not return error")

		_, err = service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test.txt",
		})
		assert.Error(t, err, "Should return error")
	})

	t.Run("DeleteObjects", func(t *testing.T) {
		keys := []string{"delete1.txt", "delete2.txt", "delete3.txt"}
		for _, key := range keys {
			_, err := service.PutObject(ctx, storage.PutObjectOptions{
				Key:    key,
				Reader: bytes.NewReader([]byte("content")),
				Size:   7,
			})
			require.NoError(t, err, "Should not return error")
		}

		err := service.DeleteObjects(ctx, storage.DeleteObjectsOptions{
			Keys: keys,
		})
		assert.NoError(t, err, "Should not return error")

		for _, key := range keys {
			_, err := service.GetObject(ctx, storage.GetObjectOptions{Key: key})
			assert.Error(t, err, "Should return error")
		}
	})

	t.Run("NestedDirectories", func(t *testing.T) {
		nestedKey := "level1/level2/level3/nested.txt"
		data := []byte("nested content")

		_, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    nestedKey,
			Reader: bytes.NewReader(data),
			Size:   int64(len(data)),
		})
		require.NoError(t, err, "Should not return error")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: nestedKey,
		})
		require.NoError(t, err, "Should not return error")

		defer reader.Close()

		readData, err := io.ReadAll(reader)
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, data, readData, "Should equal expected value")
	})

	t.Run("GetPresignedUrl", func(t *testing.T) {
		url, err := service.GetPresignedURL(ctx, storage.PresignedURLOptions{
			Key: "test-moved.txt",
		})

		require.NoError(t, err, "Should not return error")
		assert.Contains(t, url, "file://", "Should contain expected value")
		assert.Contains(t, url, "test-moved.txt", "Should contain expected value")
	})
}

// TestCleanupEmptyDirs tests cleanup empty dirs functionality.
func TestCleanupEmptyDirs(t *testing.T) {
	tempDir := t.TempDir()
	service := &Service{root: tempDir}

	nestedPath := filepath.Join(tempDir, "a", "b", "c", "test.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(nestedPath), 0o755), "Should not return error")
	require.NoError(t, os.WriteFile(nestedPath, []byte("test"), 0o644), "Should not return error")

	require.NoError(t, os.Remove(nestedPath), "Should not return error")

	service.cleanupEmptyDirs(filepath.Dir(nestedPath))

	_, err := os.Stat(filepath.Join(tempDir, "a"))
	assert.True(t, os.IsNotExist(err), "Should be true")
}

// TestEdgeCases tests edge cases functionality.
func TestEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("EmptyFile", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		info, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    "empty.txt",
			Reader: bytes.NewReader([]byte{}),
			Size:   0,
		})

		require.NoError(t, err, "Should not return error")
		assert.Equal(t, int64(0), info.Size, "Should equal expected value")
		assert.NotEmpty(t, info.ETag, "Should not be empty")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{Key: "empty.txt"})
		require.NoError(t, err, "Should not return error")

		defer reader.Close()

		data, err := io.ReadAll(reader)
		require.NoError(t, err, "Should not return error")
		assert.Empty(t, data, "Should be empty")
	})

	t.Run("SpecialCharactersInKey", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		keys := []string{
			"file with spaces.txt",
			"文件中文名.txt",
			"file-with-dashes.txt",
			"file_with_underscores.txt",
			"file.multiple.dots.txt",
		}

		for _, key := range keys {
			data := []byte("test content")
			_, err := service.PutObject(ctx, storage.PutObjectOptions{
				Key:    key,
				Reader: bytes.NewReader(data),
				Size:   int64(len(data)),
			})
			require.NoError(t, err, "Failed to put object with key: %s", key)

			reader, err := service.GetObject(ctx, storage.GetObjectOptions{Key: key})
			require.NoError(t, err, "Failed to get object with key: %s", key)

			defer reader.Close()

			readData, err := io.ReadAll(reader)
			require.NoError(t, err, "Should not return error")
			assert.Equal(t, data, readData, "Should equal expected value")
		}
	})

	t.Run("OverwriteExistingFile", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		key := "overwrite.txt"
		originalData := []byte("original content")
		newData := []byte("new content that is longer")

		_, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    key,
			Reader: bytes.NewReader(originalData),
			Size:   int64(len(originalData)),
		})
		require.NoError(t, err, "Should not return error")

		info, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    key,
			Reader: bytes.NewReader(newData),
			Size:   int64(len(newData)),
		})
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, int64(len(newData)), info.Size, "Should equal expected value")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{Key: key})
		require.NoError(t, err, "Should not return error")

		defer reader.Close()

		readData, err := io.ReadAll(reader)
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, newData, readData, "Should equal expected value")
	})

	t.Run("DeleteNonExistentFile", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		err := service.DeleteObject(ctx, storage.DeleteObjectOptions{
			Key: "nonexistent.txt",
		})
		assert.NoError(t, err, "Should not return error")
	})

	t.Run("CopyNonExistentFile", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		_, err := service.CopyObject(ctx, storage.CopyObjectOptions{
			SourceKey: "nonexistent.txt",
			DestKey:   "dest.txt",
		})
		assert.Error(t, err, "Should return error")
		assert.Equal(t, storage.ErrObjectNotFound, err, "Should equal expected value")
	})

	t.Run("MoveNonExistentFile", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		_, err := service.MoveObject(ctx, storage.MoveObjectOptions{
			CopyObjectOptions: storage.CopyObjectOptions{
				SourceKey: "nonexistent.txt",
				DestKey:   "dest.txt",
			},
		})
		assert.Error(t, err, "Should return error")
	})

	t.Run("StatNonExistentFile", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		_, err := service.StatObject(ctx, storage.StatObjectOptions{
			Key: "nonexistent.txt",
		})
		assert.Error(t, err, "Should return error")
		assert.Equal(t, storage.ErrObjectNotFound, err, "Should equal expected value")
	})

	t.Run("ListEmptyDirectory", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		objects, err := service.ListObjects(ctx, storage.ListObjectsOptions{
			Recursive: true,
		})
		require.NoError(t, err, "Should not return error")
		assert.Empty(t, objects, "Should be empty")
	})

	t.Run("ListWithNonExistentPrefix", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		_, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    "exists.txt",
			Reader: bytes.NewReader([]byte("test")),
			Size:   4,
		})
		require.NoError(t, err, "Should not return error")

		objects, err := service.ListObjects(ctx, storage.ListObjectsOptions{
			Prefix:    "nonexistent/",
			Recursive: true,
		})
		require.NoError(t, err, "Should not return error")
		assert.Empty(t, objects, "Should be empty")
	})

	t.Run("ListWithMaxKeys", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		for i := range 10 {
			_, err := service.PutObject(ctx, storage.PutObjectOptions{
				Key:    filepath.Join("test", "file"+string(rune('0'+i))+".txt"),
				Reader: bytes.NewReader([]byte("content")),
				Size:   7,
			})
			require.NoError(t, err, "Should not return error")
		}

		objects, err := service.ListObjects(ctx, storage.ListObjectsOptions{
			Prefix:    "test/",
			Recursive: true,
			MaxKeys:   5,
		})
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, 5, len(objects), "Should equal expected value")
	})

	t.Run("ListWithZeroMaxKeys", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		_, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    "test.txt",
			Reader: bytes.NewReader([]byte("test")),
			Size:   4,
		})
		require.NoError(t, err, "Should not return error")

		objects, err := service.ListObjects(ctx, storage.ListObjectsOptions{
			Recursive: true,
			MaxKeys:   0,
		})
		require.NoError(t, err, "Should not return error")
		assert.NotEmpty(t, objects, "Should not be empty")
	})

	t.Run("PromoteNonTempFile", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		key := "regular/file.txt"
		_, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    key,
			Reader: bytes.NewReader([]byte("content")),
			Size:   7,
		})
		require.NoError(t, err, "Should not return error")

		info, err := service.PromoteObject(ctx, key)
		require.NoError(t, err, "Should not return error")
		assert.Nil(t, info, "Should be nil")

		_, err = service.GetObject(ctx, storage.GetObjectOptions{Key: key})
		assert.NoError(t, err, "Should not return error")
	})

	t.Run("PromoteNonExistentTempFile", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		_, err := service.PromoteObject(ctx, storage.TempPrefix+"nonexistent.txt")
		assert.Error(t, err, "Should return error")
	})

	t.Run("VeryLongPath", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		longPath := ""
		for range 20 {
			longPath += "verylongdirectoryname/"
		}

		longPath += "file.txt"

		data := []byte("test")
		_, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    longPath,
			Reader: bytes.NewReader(data),
			Size:   int64(len(data)),
		})
		require.NoError(t, err, "Should not return error")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{Key: longPath})
		require.NoError(t, err, "Should not return error")

		defer reader.Close()
	})

	t.Run("InvalidRootDirectory", func(t *testing.T) {
		_, err := New(config.FilesystemConfig{Root: "/invalid/readonly/path/that/should/not/exist"})
		assert.Error(t, err, "Should return error")
	})

	t.Run("DefaultRootDirectory", func(t *testing.T) {
		originalWd, err := os.Getwd()
		require.NoError(t, err, "Should not return error")

		tempDir := t.TempDir()
		err = os.Chdir(tempDir)
		require.NoError(t, err, "Should not return error")

		defer os.Chdir(originalWd)

		service, err := New(config.FilesystemConfig{})
		require.NoError(t, err, "Should not return error")
		assert.NotNil(t, service, "Should not be nil")

		_, err = os.Stat(filepath.Join(tempDir, "storage"))
		assert.NoError(t, err, "Should not return error")
	})

	t.Run("MD5ConsistencyCheck", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		data := []byte("test data for md5 check")
		key := "md5test.txt"

		info1, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    key,
			Reader: bytes.NewReader(data),
			Size:   int64(len(data)),
		})
		require.NoError(t, err, "Should not return error")

		info2, err := service.StatObject(ctx, storage.StatObjectOptions{Key: key})
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, info1.ETag, info2.ETag, "Should equal expected value")
		assert.NotEmpty(t, info1.ETag, "Should not be empty")
	})

	t.Run("ContentTypeDetection", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		testCases := []struct {
			key         string
			contentType string
		}{
			{"test.txt", "text/plain; charset=utf-8"},
			{"test.json", "application/json"},
			{"test.html", "text/html; charset=utf-8"},
			{"test.pdf", "application/pdf"},
			{"test.jpg", "image/jpeg"},
			{"test.png", "image/png"},
		}

		for _, tc := range testCases {
			_, err := service.PutObject(ctx, storage.PutObjectOptions{
				Key:    tc.key,
				Reader: bytes.NewReader([]byte("test")),
				Size:   4,
			})
			require.NoError(t, err, "Should not return error")

			info, err := service.StatObject(ctx, storage.StatObjectOptions{Key: tc.key})
			require.NoError(t, err, "Should not return error")
			assert.Equal(t, tc.contentType, info.ContentType, "Key: %s", tc.key)
		}
	})
}

// TestConcurrency tests concurrency functionality.
func TestConcurrency(t *testing.T) {
	ctx := context.Background()

	service, cleanup := setupTestService(t)
	defer cleanup()

	t.Run("ConcurrentPutObject", func(t *testing.T) {
		concurrency := 10
		done := make(chan bool, concurrency)

		for i := range concurrency {
			go func(id int) {
				key := filepath.Join("concurrent", "put", "file"+string(rune('0'+id))+".txt")
				data := []byte("concurrent content " + string(rune('0'+id)))
				_, err := service.PutObject(ctx, storage.PutObjectOptions{
					Key:    key,
					Reader: bytes.NewReader(data),
					Size:   int64(len(data)),
				})
				assert.NoError(t, err, "Should not return error")

				done <- true
			}(i)
		}

		for range concurrency {
			<-done
		}

		objects, err := service.ListObjects(ctx, storage.ListObjectsOptions{
			Prefix:    "concurrent/put/",
			Recursive: true,
		})
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, concurrency, len(objects), "Should equal expected value")
	})

	t.Run("ConcurrentReadSameFile", func(t *testing.T) {
		key := "concurrent/read/shared.txt"
		expectedData := []byte("shared content")

		_, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    key,
			Reader: bytes.NewReader(expectedData),
			Size:   int64(len(expectedData)),
		})
		require.NoError(t, err, "Should not return error")

		concurrency := 20
		done := make(chan bool, concurrency)

		for range concurrency {
			go func() {
				reader, err := service.GetObject(ctx, storage.GetObjectOptions{Key: key})
				assert.NoError(t, err, "Should not return error")

				if reader != nil {
					defer reader.Close()

					data, err := io.ReadAll(reader)
					assert.NoError(t, err, "Should not return error")
					assert.Equal(t, expectedData, data, "Should equal expected value")
				}

				done <- true
			}()
		}

		for range concurrency {
			<-done
		}
	})

	t.Run("ConcurrentDeleteDifferentFiles", func(t *testing.T) {
		concurrency := 10

		for i := range concurrency {
			key := filepath.Join("concurrent", "delete", "file"+string(rune('0'+i))+".txt")
			_, err := service.PutObject(ctx, storage.PutObjectOptions{
				Key:    key,
				Reader: bytes.NewReader([]byte("content")),
				Size:   7,
			})
			require.NoError(t, err, "Should not return error")
		}

		done := make(chan bool, concurrency)
		for i := range concurrency {
			go func(id int) {
				key := filepath.Join("concurrent", "delete", "file"+string(rune('0'+id))+".txt")
				err := service.DeleteObject(ctx, storage.DeleteObjectOptions{Key: key})
				assert.NoError(t, err, "Should not return error")

				done <- true
			}(i)
		}

		for range concurrency {
			<-done
		}

		objects, err := service.ListObjects(ctx, storage.ListObjectsOptions{
			Prefix:    "concurrent/delete/",
			Recursive: true,
		})
		require.NoError(t, err, "Should not return error")
		assert.Empty(t, objects, "Should be empty")
	})
}

// TestLargeFile tests large file functionality.
func TestLargeFile(t *testing.T) {
	ctx := context.Background()

	service, cleanup := setupTestService(t)
	defer cleanup()

	t.Run("LargeFileUploadAndDownload", func(t *testing.T) {
		size := 10 * 1024 * 1024 // 10MB

		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}

		key := "large/file.bin"
		info, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    key,
			Reader: bytes.NewReader(data),
			Size:   int64(size),
		})
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, int64(size), info.Size, "Should equal expected value")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{Key: key})
		require.NoError(t, err, "Should not return error")

		defer reader.Close()

		readData, err := io.ReadAll(reader)
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, data, readData, "Should equal expected value")
	})
}

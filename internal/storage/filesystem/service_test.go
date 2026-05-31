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
	require.NoError(t, err, "Service should complete without error")

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

		require.NoError(t, err, "TestFilesystemService should complete without error")
		assert.NotNil(t, info, "TestFilesystemService should return a non-nil value")
		assert.Equal(t, "test.txt", info.Key, "TestFilesystemService should match expected value")
		assert.Equal(t, int64(len(data)), info.Size, "TestFilesystemService should match expected value")
		assert.Equal(t, "text/plain", info.ContentType, "TestFilesystemService should match expected value")
	})

	t.Run("GetObjectSuccess", func(t *testing.T) {
		expectedData := []byte("Hello, Filesystem Storage!")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test.txt",
		})

		require.NoError(t, err, "TestFilesystemService should complete without error")

		require.NotNil(t, reader, "TestFilesystemService should return a non-nil value")
		defer reader.Close()

		data, err := io.ReadAll(reader)
		require.NoError(t, err, "TestFilesystemService should complete without error")
		assert.Equal(t, expectedData, data, "TestFilesystemService should match expected value")
	})

	t.Run("GetObjectNotFound", func(t *testing.T) {
		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "nonexistent.txt",
		})

		assert.Error(t, err, "Missing object reads should return an error")
		assert.Nil(t, reader, "TestFilesystemService should return nil")
		assert.Equal(t, storage.ErrObjectNotFound, err, "TestFilesystemService should match expected value")
	})

	t.Run("StatObject", func(t *testing.T) {
		info, err := service.StatObject(ctx, storage.StatObjectOptions{
			Key: "test.txt",
		})

		require.NoError(t, err, "TestFilesystemService should complete without error")
		assert.NotNil(t, info, "TestFilesystemService should return a non-nil value")
		assert.Equal(t, "test.txt", info.Key, "TestFilesystemService should match expected value")
		assert.Greater(t, info.Size, int64(0), "Should be greater")
	})

	t.Run("CopyObject", func(t *testing.T) {
		info, err := service.CopyObject(ctx, storage.CopyObjectOptions{
			SourceKey: "test.txt",
			DestKey:   "test-copy.txt",
		})

		require.NoError(t, err, "TestFilesystemService should complete without error")
		assert.NotNil(t, info, "TestFilesystemService should return a non-nil value")
		assert.Equal(t, "test-copy.txt", info.Key, "TestFilesystemService should match expected value")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test-copy.txt",
		})
		require.NoError(t, err, "TestFilesystemService should complete without error")

		defer reader.Close()

		data, err := io.ReadAll(reader)
		require.NoError(t, err, "TestFilesystemService should complete without error")
		assert.Equal(t, []byte("Hello, Filesystem Storage!"), data, "TestFilesystemService should match expected value")
	})

	t.Run("DeleteObject", func(t *testing.T) {
		err := service.DeleteObject(ctx, storage.DeleteObjectOptions{
			Key: "test.txt",
		})

		assert.NoError(t, err, "TestFilesystemService should complete without error")

		_, err = service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test.txt",
		})
		assert.Error(t, err, "Deleted object reads should return an error")
	})

	t.Run("DeleteObjects", func(t *testing.T) {
		keys := []string{"delete1.txt", "delete2.txt", "delete3.txt"}
		for _, key := range keys {
			_, err := service.PutObject(ctx, storage.PutObjectOptions{
				Key:    key,
				Reader: bytes.NewReader([]byte("content")),
				Size:   7,
			})
			require.NoError(t, err, "TestFilesystemService should complete without error")
		}

		err := service.DeleteObjects(ctx, storage.DeleteObjectsOptions{
			Keys: keys,
		})
		assert.NoError(t, err, "TestFilesystemService should complete without error")

		for _, key := range keys {
			_, err := service.GetObject(ctx, storage.GetObjectOptions{Key: key})
			assert.Error(t, err, "Deleted batch object reads should return an error")
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
		require.NoError(t, err, "TestFilesystemService should complete without error")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: nestedKey,
		})
		require.NoError(t, err, "TestFilesystemService should complete without error")

		defer reader.Close()

		readData, err := io.ReadAll(reader)
		require.NoError(t, err, "TestFilesystemService should complete without error")
		assert.Equal(t, data, readData, "TestFilesystemService should match expected value")
	})
}

// TestCleanupEmptyDirs tests cleanup empty dirs functionality.
func TestCleanupEmptyDirs(t *testing.T) {
	tempDir := t.TempDir()
	service := &Service{root: tempDir}

	nestedPath := filepath.Join(tempDir, "a", "b", "c", "test.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(nestedPath), 0o755), "TestCleanupEmptyDirs should complete without error")
	require.NoError(t, os.WriteFile(nestedPath, []byte("test"), 0o644), "TestCleanupEmptyDirs should complete without error")

	require.NoError(t, os.Remove(nestedPath), "TestCleanupEmptyDirs should complete without error")

	service.cleanupEmptyDirs(filepath.Dir(nestedPath))

	_, err := os.Stat(filepath.Join(tempDir, "a"))
	assert.True(t, os.IsNotExist(err), "TestCleanupEmptyDirs condition should be true")
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

		require.NoError(t, err, "TestEdgeCases should complete without error")
		assert.Equal(t, int64(0), info.Size, "TestEdgeCases should match expected value")
		assert.NotEmpty(t, info.ETag, "TestEdgeCases should return non-empty value")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{Key: "empty.txt"})
		require.NoError(t, err, "TestEdgeCases should complete without error")

		defer reader.Close()

		data, err := io.ReadAll(reader)
		require.NoError(t, err, "TestEdgeCases should complete without error")
		assert.Empty(t, data, "TestEdgeCases should return empty value")
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
			require.NoError(t, err, "TestEdgeCases should complete without error")
			assert.Equal(t, data, readData, "TestEdgeCases should match expected value")
		}
	})

	t.Run("RejectsInvalidObjectKeys", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		testCases := []struct {
			name string
			key  string
		}{
			{name: "ParentDirectory", key: "../escape.txt"},
			{name: "NestedParentDirectory", key: "safe/../../escape.txt"},
			{name: "AbsolutePath", key: "/tmp/escape.txt"},
			{name: "RedundantSlash", key: "safe//file.txt"},
			{name: "CurrentDirectory", key: "safe/./file.txt"},
			{name: "Backslash", key: `safe\file.txt`},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := service.PutObject(ctx, storage.PutObjectOptions{
					Key:    tc.key,
					Reader: bytes.NewReader([]byte("payload")),
					Size:   7,
				})
				assert.Error(t, err, "PutObject should reject invalid key %q", tc.key)

				_, err = service.GetObject(ctx, storage.GetObjectOptions{Key: tc.key})
				assert.Error(t, err, "GetObject should reject invalid key %q", tc.key)

				err = service.DeleteObject(ctx, storage.DeleteObjectOptions{Key: tc.key})
				assert.Error(t, err, "DeleteObject should reject invalid key %q", tc.key)

				_, err = service.CopyObject(ctx, storage.CopyObjectOptions{
					SourceKey: "file-with-dashes.txt",
					DestKey:   tc.key,
				})
				assert.Error(t, err, "CopyObject should reject invalid destination key %q", tc.key)
			})
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
		require.NoError(t, err, "TestEdgeCases should complete without error")

		info, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    key,
			Reader: bytes.NewReader(newData),
			Size:   int64(len(newData)),
		})
		require.NoError(t, err, "TestEdgeCases should complete without error")
		assert.Equal(t, int64(len(newData)), info.Size, "TestEdgeCases should match expected value")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{Key: key})
		require.NoError(t, err, "TestEdgeCases should complete without error")

		defer reader.Close()

		readData, err := io.ReadAll(reader)
		require.NoError(t, err, "TestEdgeCases should complete without error")
		assert.Equal(t, newData, readData, "TestEdgeCases should match expected value")
	})

	t.Run("DeleteNonExistentFile", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		err := service.DeleteObject(ctx, storage.DeleteObjectOptions{
			Key: "nonexistent.txt",
		})
		assert.NoError(t, err, "TestEdgeCases should complete without error")
	})

	t.Run("CopyNonExistentFile", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		_, err := service.CopyObject(ctx, storage.CopyObjectOptions{
			SourceKey: "nonexistent.txt",
			DestKey:   "dest.txt",
		})
		assert.Error(t, err, "Copying a missing source object should return an error")
		assert.Equal(t, storage.ErrObjectNotFound, err, "TestEdgeCases should match expected value")
	})

	t.Run("StatNonExistentFile", func(t *testing.T) {
		service, cleanup := setupTestService(t)
		defer cleanup()

		_, err := service.StatObject(ctx, storage.StatObjectOptions{
			Key: "nonexistent.txt",
		})
		assert.Error(t, err, "Statting a missing object should return an error")
		assert.Equal(t, storage.ErrObjectNotFound, err, "TestEdgeCases should match expected value")
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
		require.NoError(t, err, "TestEdgeCases should complete without error")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{Key: longPath})
		require.NoError(t, err, "TestEdgeCases should complete without error")

		defer reader.Close()
	})

	t.Run("InvalidRootDirectory", func(t *testing.T) {
		_, err := New(config.FilesystemConfig{Root: "/invalid/readonly/path/that/should/not/exist"})
		assert.Error(t, err, "Invalid root directory should return an error")
	})

	t.Run("DefaultRootDirectory", func(t *testing.T) {
		originalWd, err := os.Getwd()
		require.NoError(t, err, "TestEdgeCases should complete without error")

		tempDir := t.TempDir()
		err = os.Chdir(tempDir)
		require.NoError(t, err, "TestEdgeCases should complete without error")

		defer os.Chdir(originalWd)

		service, err := New(config.FilesystemConfig{})
		require.NoError(t, err, "TestEdgeCases should complete without error")
		assert.NotNil(t, service, "TestEdgeCases should return a non-nil value")

		_, err = os.Stat(filepath.Join(tempDir, "storage"))
		assert.NoError(t, err, "TestEdgeCases should complete without error")
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
		require.NoError(t, err, "TestEdgeCases should complete without error")

		info2, err := service.StatObject(ctx, storage.StatObjectOptions{Key: key})
		require.NoError(t, err, "TestEdgeCases should complete without error")

		assert.Equal(t, info1.ETag, info2.ETag, "TestEdgeCases should match expected value")
		assert.NotEmpty(t, info1.ETag, "TestEdgeCases should return non-empty value")
	})

	t.Run("ETagSidecarCreatedOnPutObject", func(t *testing.T) {
		// PutObject must persist the MD5 ETag to the .etags sidecar tree
		// so subsequent StatObject calls can read it without re-hashing
		// the object body.
		tempDir := t.TempDir()
		service, err := New(config.FilesystemConfig{Root: tempDir})
		require.NoError(t, err, "Service construction should succeed")

		key := "priv/sidecar/created.bin"
		data := []byte("etag sidecar payload")

		_, err = service.PutObject(ctx, storage.PutObjectOptions{
			Key:    key,
			Reader: bytes.NewReader(data),
			Size:   int64(len(data)),
		})
		require.NoError(t, err, "PutObject should succeed")

		sidecarPath := filepath.Join(tempDir, ".etags", filepath.FromSlash(key))
		contents, err := os.ReadFile(sidecarPath)
		require.NoError(t, err, "ETag sidecar must exist after PutObject")
		assert.NotEmpty(t, contents, "ETag sidecar must contain the MD5 hex string")
	})

	t.Run("ETagSidecarRemovedOnDeleteObject", func(t *testing.T) {
		// DeleteObject must clean up the sidecar; otherwise a future
		// PutObject to the same key briefly observes a stale ETag, and
		// abandoned keys leak disk space under .etags.
		tempDir := t.TempDir()
		service, err := New(config.FilesystemConfig{Root: tempDir})
		require.NoError(t, err, "Service construction should succeed")

		key := "priv/sidecar/removed.bin"

		_, err = service.PutObject(ctx, storage.PutObjectOptions{
			Key:    key,
			Reader: bytes.NewReader([]byte("payload")),
			Size:   7,
		})
		require.NoError(t, err, "PutObject should succeed")

		require.NoError(t, service.DeleteObject(ctx, storage.DeleteObjectOptions{Key: key}), "DeleteObject should succeed")

		sidecarPath := filepath.Join(tempDir, ".etags", filepath.FromSlash(key))
		_, statErr := os.Stat(sidecarPath)
		assert.True(t, os.IsNotExist(statErr), "ETag sidecar must be removed after DeleteObject")
	})

	t.Run("StatObjectFallsBackForLegacyDataWithoutSidecar", func(t *testing.T) {
		// Objects written before the sidecar mechanism (or whose sidecar
		// was lost) must still be statable. The contract is: empty ETag,
		// no error — the proxy then serves without a validator.
		tempDir := t.TempDir()
		service, err := New(config.FilesystemConfig{Root: tempDir})
		require.NoError(t, err, "Service construction should succeed")

		key := "priv/legacy.bin"
		objectPath := filepath.Join(tempDir, filepath.FromSlash(key))
		require.NoError(t, os.MkdirAll(filepath.Dir(objectPath), 0o755), "Should create legacy object directory")
		require.NoError(t, os.WriteFile(objectPath, []byte("legacy"), 0o644), "Should write legacy object directly")

		info, err := service.StatObject(ctx, storage.StatObjectOptions{Key: key})
		require.NoError(t, err, "StatObject must succeed for legacy objects without sidecar")
		assert.Equal(t, key, info.Key, "StatObject must echo the queried key")
		assert.Equal(t, int64(6), info.Size, "StatObject must report the object size")
		assert.Empty(t, info.ETag, "Legacy object without sidecar must yield empty ETag")
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
			require.NoError(t, err, "TestEdgeCases should complete without error")

			info, err := service.StatObject(ctx, storage.StatObjectOptions{Key: tc.key})
			require.NoError(t, err, "TestEdgeCases should complete without error")
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
				assert.NoError(t, err, "TestConcurrency should complete without error")

				done <- true
			}(i)
		}

		for range concurrency {
			<-done
		}

		// Verify each concurrent put landed via StatObject; ListObjects
		// used to play this role before it was removed.
		for i := range concurrency {
			key := filepath.Join("concurrent", "put", "file"+string(rune('0'+i))+".txt")
			_, err := service.StatObject(ctx, storage.StatObjectOptions{Key: key})
			require.NoError(t, err, "Concurrent put %q should be visible", key)
		}
	})

	t.Run("ConcurrentReadSameFile", func(t *testing.T) {
		key := "concurrent/read/shared.txt"
		expectedData := []byte("shared content")

		_, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    key,
			Reader: bytes.NewReader(expectedData),
			Size:   int64(len(expectedData)),
		})
		require.NoError(t, err, "TestConcurrency should complete without error")

		concurrency := 20
		done := make(chan bool, concurrency)

		for range concurrency {
			go func() {
				reader, err := service.GetObject(ctx, storage.GetObjectOptions{Key: key})
				assert.NoError(t, err, "TestConcurrency should complete without error")

				if reader != nil {
					defer reader.Close()

					data, err := io.ReadAll(reader)
					assert.NoError(t, err, "TestConcurrency should complete without error")
					assert.Equal(t, expectedData, data, "TestConcurrency should match expected value")
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
			require.NoError(t, err, "TestConcurrency should complete without error")
		}

		done := make(chan bool, concurrency)
		for i := range concurrency {
			go func(id int) {
				key := filepath.Join("concurrent", "delete", "file"+string(rune('0'+id))+".txt")
				err := service.DeleteObject(ctx, storage.DeleteObjectOptions{Key: key})
				assert.NoError(t, err, "TestConcurrency should complete without error")

				done <- true
			}(i)
		}

		for range concurrency {
			<-done
		}

		// Each concurrent delete should have removed its key; verify
		// via StatObject (ListObjects no longer exists).
		for i := range concurrency {
			key := filepath.Join("concurrent", "delete", "file"+string(rune('0'+i))+".txt")
			_, err := service.StatObject(ctx, storage.StatObjectOptions{Key: key})
			assert.ErrorIs(t, err, storage.ErrObjectNotFound, "Concurrent delete of %q should leave it gone", key)
		}
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
		require.NoError(t, err, "TestLargeFile should complete without error")
		assert.Equal(t, int64(size), info.Size, "TestLargeFile should match expected value")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{Key: key})
		require.NoError(t, err, "TestLargeFile should complete without error")

		defer reader.Close()

		readData, err := io.ReadAll(reader)
		require.NoError(t, err, "TestLargeFile should complete without error")
		assert.Equal(t, data, readData, "TestLargeFile should match expected value")
	})

	t.Run("ImplementsMultipart", func(t *testing.T) {
		_, isMultipart := any(service).(storage.Multipart)
		assert.True(t, isMultipart, "Filesystem backend must implement storage.Multipart")
	})

	t.Run("RejectsInvalidMultipartInputs", func(t *testing.T) {
		mp := service.(storage.Multipart)

		_, err := mp.InitMultipart(ctx, storage.InitMultipartOptions{
			Key:         "../escape.bin",
			ContentType: "application/octet-stream",
		})
		assert.Error(t, err, "InitMultipart should reject invalid object keys")

		_, err = mp.PutPart(ctx, storage.PutPartOptions{
			Key:        "large/file.bin",
			UploadID:   "../session",
			PartNumber: 1,
			Reader:     bytes.NewReader([]byte("payload")),
			Size:       7,
		})
		assert.Error(t, err, "PutPart should reject invalid upload IDs")

		err = mp.AbortMultipart(ctx, storage.AbortMultipartOptions{
			Key:      "large/file.bin",
			UploadID: "../session",
		})
		assert.Error(t, err, "AbortMultipart should reject invalid upload IDs")
	})
}

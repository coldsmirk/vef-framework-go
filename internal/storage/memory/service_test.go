package memory

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/storage"
)

// TestMemoryService tests memory service functionality.
func TestMemoryService(t *testing.T) {
	ctx := context.Background()
	service := New()

	t.Run("PutObject", func(t *testing.T) {
		data := []byte("Hello, Memory Storage!")
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

		require.NoError(t, err, "PutObject should succeed")
		assert.NotNil(t, info, "ObjectInfo should not be nil")
		assert.Equal(t, "test.txt", info.Key, "Key should match")
		assert.Equal(t, int64(len(data)), info.Size, "Size should match")
		assert.Equal(t, "text/plain", info.ContentType, "ContentType should match")
	})

	t.Run("GetObjectSuccess", func(t *testing.T) {
		expectedData := []byte("Hello, Memory Storage!")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test.txt",
		})

		require.NoError(t, err, "GetObject should succeed")

		require.NotNil(t, reader, "Reader should not be nil")
		defer reader.Close()

		data, err := io.ReadAll(reader)
		require.NoError(t, err, "Reading data should succeed")
		assert.Equal(t, expectedData, data, "Data should match uploaded content")
	})

	t.Run("GetObjectNotFound", func(t *testing.T) {
		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "nonexistent.txt",
		})

		assert.Error(t, err, "GetObject should return error for non-existent key")
		assert.Nil(t, reader, "Reader should be nil for non-existent key")
		assert.Equal(t, storage.ErrObjectNotFound, err, "Error should be ErrObjectNotFound")
	})

	t.Run("StatObject", func(t *testing.T) {
		info, err := service.StatObject(ctx, storage.StatObjectOptions{
			Key: "test.txt",
		})

		require.NoError(t, err, "StatObject should succeed")
		assert.NotNil(t, info, "ObjectInfo should not be nil")
		assert.Equal(t, "test.txt", info.Key, "Key should match")
		assert.Equal(t, "text/plain", info.ContentType, "ContentType should match")
	})

	t.Run("CopyObject", func(t *testing.T) {
		info, err := service.CopyObject(ctx, storage.CopyObjectOptions{
			SourceKey: "test.txt",
			DestKey:   "test-copy.txt",
		})

		require.NoError(t, err, "CopyObject should succeed")
		assert.NotNil(t, info, "ObjectInfo should not be nil")
		assert.Equal(t, "test-copy.txt", info.Key, "Destination key should match")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test-copy.txt",
		})
		require.NoError(t, err, "Should be able to get copied object")

		defer reader.Close()

		data, err := io.ReadAll(reader)
		require.NoError(t, err, "Reading copied data should succeed")
		assert.Equal(t, []byte("Hello, Memory Storage!"), data, "Copied data should match original")
	})

	t.Run("MoveObject", func(t *testing.T) {
		info, err := service.MoveObject(ctx, storage.MoveObjectOptions{
			CopyObjectOptions: storage.CopyObjectOptions{
				SourceKey: "test-copy.txt",
				DestKey:   "test-moved.txt",
			},
		})

		require.NoError(t, err, "MoveObject should succeed")
		assert.NotNil(t, info, "ObjectInfo should not be nil")
		assert.Equal(t, "test-moved.txt", info.Key, "Destination key should match")

		_, err = service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test-copy.txt",
		})
		assert.Error(t, err, "Source object should be deleted after move")
		assert.Equal(t, storage.ErrObjectNotFound, err, "Error should be ErrObjectNotFound")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test-moved.txt",
		})
		require.NoError(t, err, "Should be able to get moved object")

		defer reader.Close()
	})

	t.Run("ListObjects", func(t *testing.T) {
		_, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    "folder/file1.txt",
			Reader: bytes.NewReader([]byte("file1")),
			Size:   5,
		})
		require.NoError(t, err, "PutObject should succeed for file1")

		_, err = service.PutObject(ctx, storage.PutObjectOptions{
			Key:    "folder/file2.txt",
			Reader: bytes.NewReader([]byte("file2")),
			Size:   5,
		})
		require.NoError(t, err, "PutObject should succeed for file2")

		t.Run("ListAllObjects", func(t *testing.T) {
			objects, err := service.ListObjects(ctx, storage.ListObjectsOptions{
				Recursive: true,
			})

			require.NoError(t, err, "ListObjects should succeed")
			assert.GreaterOrEqual(t, len(objects), 3, "Should have at least 3 objects")
		})

		t.Run("ListWithPrefix", func(t *testing.T) {
			objects, err := service.ListObjects(ctx, storage.ListObjectsOptions{
				Prefix:    "folder/",
				Recursive: true,
			})

			require.NoError(t, err, "ListObjects with prefix should succeed")
			assert.Equal(t, 2, len(objects), "Should have exactly 2 objects in folder")
		})
	})

	t.Run("PromoteObject", func(t *testing.T) {
		tempKey := storage.TempPrefix + "2025/01/15/test.txt"
		_, err := service.PutObject(ctx, storage.PutObjectOptions{
			Key:    tempKey,
			Reader: bytes.NewReader([]byte("temp content")),
			Size:   12,
		})
		require.NoError(t, err, "PutObject should succeed for temp file")

		info, err := service.PromoteObject(ctx, tempKey)
		require.NoError(t, err, "PromoteObject should succeed")
		assert.NotNil(t, info, "ObjectInfo should not be nil")
		assert.Equal(t, "2025/01/15/test.txt", info.Key, "Promoted key should not have temp prefix")

		_, err = service.GetObject(ctx, storage.GetObjectOptions{Key: tempKey})
		assert.Error(t, err, "Temp file should be deleted after promotion")

		reader, err := service.GetObject(ctx, storage.GetObjectOptions{
			Key: "2025/01/15/test.txt",
		})
		require.NoError(t, err, "Should be able to get promoted object")

		defer reader.Close()
	})

	t.Run("DeleteObject", func(t *testing.T) {
		err := service.DeleteObject(ctx, storage.DeleteObjectOptions{
			Key: "test.txt",
		})

		assert.NoError(t, err, "DeleteObject should succeed")

		_, err = service.GetObject(ctx, storage.GetObjectOptions{
			Key: "test.txt",
		})
		assert.Error(t, err, "Deleted object should not be retrievable")
	})

	t.Run("DeleteObjects", func(t *testing.T) {
		keys := []string{"delete1.txt", "delete2.txt", "delete3.txt"}
		for _, key := range keys {
			_, err := service.PutObject(ctx, storage.PutObjectOptions{
				Key:    key,
				Reader: bytes.NewReader([]byte("content")),
				Size:   7,
			})
			require.NoError(t, err, "PutObject should succeed for "+key)
		}

		err := service.DeleteObjects(ctx, storage.DeleteObjectsOptions{
			Keys: keys,
		})
		assert.NoError(t, err, "DeleteObjects should succeed")

		for _, key := range keys {
			_, err := service.GetObject(ctx, storage.GetObjectOptions{Key: key})
			assert.Error(t, err, "Deleted object "+key+" should not be retrievable")
		}
	})
}

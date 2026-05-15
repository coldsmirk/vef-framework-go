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

	t.Run("ImplementsMultipart", func(t *testing.T) {
		_, isMultipart := service.(storage.Multipart)
		assert.True(t, isMultipart, "Memory backend must implement storage.Multipart")
	})
}

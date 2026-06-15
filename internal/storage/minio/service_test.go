package minio

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/contract"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// MinIOServiceTestSuite tests MinIO storage service implementation.
type MinIOServiceTestSuite struct {
	suite.Suite

	ctx            context.Context
	minioContainer *testx.MinIOContainer
	service        storage.Service
	multipart      storage.Multipart
	minioClient    *minio.Client

	testBucketName  string
	testObjectKey   string
	testObjectData  []byte
	testContentType string
}

func (suite *MinIOServiceTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.testBucketName = testx.TestMinIOBucket
	suite.testObjectKey = "test-file.txt"
	suite.testObjectData = []byte("Hello, MinIO Test!")
	suite.testContentType = "text/plain"

	suite.minioContainer = testx.NewMinIOContainer(suite.ctx, suite.T())

	provider, err := New(*suite.minioContainer.MinIO, &config.AppConfig{})
	suite.Require().NoError(err, "NewMinIOService should succeed")
	suite.service = provider

	multipart, ok := suite.service.(storage.Multipart)
	suite.Require().True(ok, "MinIO Service should implement storage.Multipart")
	suite.multipart = multipart

	suite.minioClient = suite.service.(*Service).client

	initializer, ok := suite.service.(contract.Initializer)
	suite.Require().True(ok, "MinIO provider must implement contract.Initializer")
	err = initializer.Init(suite.ctx)
	suite.Require().NoError(err, "Initializer.Init should succeed")
}

func (suite *MinIOServiceTestSuite) TearDownSuite() {
	objectsCh := suite.minioClient.ListObjects(suite.ctx, suite.testBucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectsCh {
		if object.Err != nil {
			continue
		}

		_ = suite.minioClient.RemoveObject(suite.ctx, suite.testBucketName, object.Key, minio.RemoveObjectOptions{})
	}

	_ = suite.minioClient.RemoveBucket(suite.ctx, suite.testBucketName)
}

func (suite *MinIOServiceTestSuite) SetupTest() {
	objectsCh := suite.minioClient.ListObjects(suite.ctx, suite.testBucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectsCh {
		if object.Err != nil {
			continue
		}

		_ = suite.minioClient.RemoveObject(suite.ctx, suite.testBucketName, object.Key, minio.RemoveObjectOptions{})
	}
}

func (suite *MinIOServiceTestSuite) TestPutObject() {
	suite.Run("Success", func() {
		reader := bytes.NewReader(suite.testObjectData)

		info, err := suite.service.PutObject(suite.ctx, storage.PutObjectOptions{
			Key:         suite.testObjectKey,
			Reader:      reader,
			Size:        int64(len(suite.testObjectData)),
			ContentType: suite.testContentType,
			Metadata: map[string]string{
				"author": "test-suite",
			},
		})

		suite.Require().NoError(err, "PutObject should succeed")
		suite.NotNil(info, "ObjectInfo should not be nil")
		suite.Equal(suite.testBucketName, info.Bucket, "Bucket should match")
		suite.Equal(suite.testObjectKey, info.Key, "Key should match")
		suite.NotEmpty(info.ETag, "ETag should not be empty")
		suite.Equal(int64(len(suite.testObjectData)), info.Size, "Size should match")
		suite.Equal(suite.testContentType, info.ContentType, "ContentType should match")
	})
}

func (suite *MinIOServiceTestSuite) TestGetObject() {
	suite.Run("Success", func() {
		suite.uploadTestObject()

		reader, info, err := suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
			Key: suite.testObjectKey,
		})

		suite.Require().NoError(err, "GetObject should succeed")

		suite.NotNil(reader, "Reader should not be nil")
		defer reader.Close()

		// GetObject returns the object metadata from its single fetch.
		suite.Require().NotNil(info, "GetObject should return ObjectInfo alongside the reader")
		suite.Equal(suite.testObjectKey, info.Key, "Info key should match")
		suite.Equal(int64(len(suite.testObjectData)), info.Size, "Info size should match")
		suite.Equal(suite.testContentType, info.ContentType, "Info content type should round-trip")
		suite.NotEmpty(info.ETag, "Info ETag should be populated")

		data, err := io.ReadAll(reader)
		suite.Require().NoError(err, "Reading data should succeed")
		suite.Equal(suite.testObjectData, data, "Data should match uploaded content")
	})

	suite.Run("NotFound", func() {
		reader, info, err := suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
			Key: "non-existent-key.txt",
		})

		suite.Error(err, "GetObject should return error for non-existent key")
		suite.Nil(reader, "Reader should be nil for non-existent key")
		suite.Nil(info, "Info should be nil for non-existent key")
		suite.Equal(storage.ErrObjectNotFound, err, "Error should be ErrObjectNotFound")
	})
}

func (suite *MinIOServiceTestSuite) TestDeleteObject() {
	suite.Run("Success", func() {
		suite.uploadTestObject()

		err := suite.service.DeleteObject(suite.ctx, storage.DeleteObjectOptions{
			Key: suite.testObjectKey,
		})

		suite.NoError(err, "DeleteObject should succeed")

		_, _, err = suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
			Key: suite.testObjectKey,
		})
		suite.Error(err, "Deleted object should not be retrievable")
	})

	suite.Run("NotFound", func() {
		err := suite.service.DeleteObject(suite.ctx, storage.DeleteObjectOptions{
			Key: "non-existent-key.txt",
		})

		suite.NoError(err, "DeleteObject should not return error for non-existent key")
	})
}

func (suite *MinIOServiceTestSuite) TestDeleteObjects() {
	suite.Run("Success", func() {
		keys := []string{"file1.txt", "file2.txt", "file3.txt"}
		for _, key := range keys {
			suite.uploadObject(key, []byte("test content"))
		}

		err := suite.service.DeleteObjects(suite.ctx, storage.DeleteObjectsOptions{
			Keys: keys,
		})

		suite.NoError(err, "DeleteObjects should succeed")

		for _, key := range keys {
			_, _, err := suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
				Key: key,
			})
			suite.Error(err, "Deleted object should not be retrievable")
		}
	})
}

func (suite *MinIOServiceTestSuite) TestCopyObject() {
	suite.Run("Success", func() {
		suite.uploadTestObject()

		destKey := "copied-file.txt"
		info, err := suite.service.CopyObject(suite.ctx, storage.CopyObjectOptions{
			SourceKey: suite.testObjectKey,
			DestKey:   destKey,
		})

		suite.NoError(err, "CopyObject should succeed")
		suite.NotNil(info, "ObjectInfo should not be nil")
		suite.Equal(suite.testBucketName, info.Bucket, "Bucket should match")
		suite.Equal(destKey, info.Key, "Destination key should match")
		suite.NotEmpty(info.ETag, "ETag should not be empty")

		reader, _, err := suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
			Key: destKey,
		})
		suite.Require().NoError(err, "Should be able to get copied object")

		defer reader.Close()

		data, err := io.ReadAll(reader)
		suite.Require().NoError(err, "Reading copied data should succeed")
		suite.Equal(suite.testObjectData, data, "Copied data should match original")
	})

	suite.Run("NotFound", func() {
		_, err := suite.service.CopyObject(suite.ctx, storage.CopyObjectOptions{
			SourceKey: "non-existent.txt",
			DestKey:   "destination.txt",
		})

		suite.Error(err, "CopyObject should return error for non-existent source")
		suite.Equal(storage.ErrObjectNotFound, err, "Error should be ErrObjectNotFound")
	})
}

func (suite *MinIOServiceTestSuite) TestStatObject() {
	suite.Run("Success", func() {
		suite.uploadTestObject()

		info, err := suite.service.StatObject(suite.ctx, storage.StatObjectOptions{
			Key: suite.testObjectKey,
		})

		suite.NoError(err, "StatObject should succeed")
		suite.NotNil(info, "ObjectInfo should not be nil")
		suite.Equal(suite.testBucketName, info.Bucket, "Bucket should match")
		suite.Equal(suite.testObjectKey, info.Key, "Key should match")
		suite.NotEmpty(info.ETag, "ETag should not be empty")
		suite.Equal(int64(len(suite.testObjectData)), info.Size, "Size should match")
		suite.Equal(suite.testContentType, info.ContentType, "ContentType should match")
		suite.NotZero(info.LastModified, "LastModified should not be zero")
	})

	suite.Run("NotFound", func() {
		_, err := suite.service.StatObject(suite.ctx, storage.StatObjectOptions{
			Key: "non-existent.txt",
		})

		suite.Error(err, "StatObject should return error for non-existent key")
		suite.Equal(storage.ErrObjectNotFound, err, "Error should be ErrObjectNotFound")
	})
}

// TestMetadataRoundTrip pins the MinIO backend to the provider-neutral metadata
// contract across PutObject -> StatObject/GetObject and CopyObject propagation.
//
// The MinIO/S3 protocol canonicalizes user-metadata keys (minio-go sends them as
// "X-Amz-Meta-<key>" HTTP headers and strips the prefix from the Go-canonical
// response header on read), so "author" -> "Author", "lower" -> "Lower", while
// already-canonical keys survive. Rather than leave this as a backend-specific
// quirk, the contract canonicalizes keys in EVERY backend, so the expectation is
// single-sourced from storage.CanonicalizeMetadataKeys — the exact helper the
// memory and filesystem backends apply at their store boundary. This test and
// the cross-backend contract suite (multipart_contract_test.go) therefore assert
// the identical guarantee.
func (suite *MinIOServiceTestSuite) TestMetadataRoundTrip() {
	input := map[string]string{
		"author":         "alice",    // lowercase -> canonicalized to "Author"
		"lower":          "value",    // lowercase -> canonicalized to "Lower"
		"X-Custom":       "custom",   // already canonical -> survives
		"Mixed-Case-Key": "mixedval", // already canonical -> survives
	}

	expected := storage.CanonicalizeMetadataKeys(input)

	suite.Run("PutObjectReturnIsCanonical", func() {
		key := "meta/put-return.bin"
		info, err := suite.service.PutObject(suite.ctx, storage.PutObjectOptions{
			Key:         key,
			Reader:      bytes.NewReader([]byte("payload")),
			Size:        int64(len("payload")),
			ContentType: suite.testContentType,
			Metadata:    input,
		})
		suite.Require().NoError(err, "PutObject with metadata should succeed")
		suite.Require().NotNil(info, "PutObject should return ObjectInfo")

		// The echoed metadata must already be canonical — identical to every
		// other backend — not the caller's verbatim keys.
		suite.Equal(expected, info.Metadata,
			"PutObject return metadata must be canonicalized to match the cross-backend contract")
		suite.NotContains(info.Metadata, "author",
			"PutObject return must not echo the caller's verbatim lowercase 'author' key")
	})

	suite.Run("PutObjectThenStatObject", func() {
		key := "meta/put-stat.bin"
		_, err := suite.service.PutObject(suite.ctx, storage.PutObjectOptions{
			Key:         key,
			Reader:      bytes.NewReader([]byte("payload")),
			Size:        int64(len("payload")),
			ContentType: suite.testContentType,
			Metadata:    input,
		})
		suite.Require().NoError(err, "PutObject with metadata should succeed")

		info, err := suite.service.StatObject(suite.ctx, storage.StatObjectOptions{Key: key})
		suite.Require().NoError(err, "StatObject should succeed")

		for k, v := range expected {
			suite.Equal(v, info.Metadata[k],
				"StatObject must return metadata under the MinIO-canonicalized key %q", k)
		}

		// Document the divergence explicitly: the lowercase key the caller
		// supplied is NOT what comes back.
		suite.NotContains(info.Metadata, "author",
			"MinIO canonicalizes keys: the verbatim lowercase 'author' must not survive (provider-neutral contract divergence)")
		suite.Contains(info.Metadata, "Author",
			"MinIO returns the canonicalized 'Author' key instead of the caller's 'author'")
	})

	suite.Run("GetObjectReturnsMetadata", func() {
		key := "meta/get.bin"
		_, err := suite.service.PutObject(suite.ctx, storage.PutObjectOptions{
			Key:         key,
			Reader:      bytes.NewReader([]byte("payload")),
			Size:        int64(len("payload")),
			ContentType: suite.testContentType,
			Metadata:    input,
		})
		suite.Require().NoError(err, "PutObject with metadata should succeed")

		reader, info, err := suite.service.GetObject(suite.ctx, storage.GetObjectOptions{Key: key})
		suite.Require().NoError(err, "GetObject should succeed")

		defer reader.Close()

		suite.Require().NotNil(info, "GetObject should return ObjectInfo")

		for k, v := range expected {
			suite.Equal(v, info.Metadata[k],
				"GetObject must surface metadata under the MinIO-canonicalized key %q", k)
		}
	})

	suite.Run("CopyObjectPropagatesMetadata", func() {
		srcKey := "meta/copy-src.bin"
		destKey := "meta/copy-dest.bin"

		_, err := suite.service.PutObject(suite.ctx, storage.PutObjectOptions{
			Key:         srcKey,
			Reader:      bytes.NewReader([]byte("payload")),
			Size:        int64(len("payload")),
			ContentType: suite.testContentType,
			Metadata:    input,
		})
		suite.Require().NoError(err, "PutObject of copy source should succeed")

		info, err := suite.service.CopyObject(suite.ctx, storage.CopyObjectOptions{
			SourceKey: srcKey,
			DestKey:   destKey,
		})
		suite.Require().NoError(err, "CopyObject should succeed")

		for k, v := range expected {
			suite.Equal(v, info.Metadata[k],
				"CopyObject result must propagate the source metadata under canonicalized key %q", k)
		}

		// And the propagation must survive a fresh Stat of the destination.
		statInfo, err := suite.service.StatObject(suite.ctx, storage.StatObjectOptions{Key: destKey})
		suite.Require().NoError(err, "StatObject of copy destination should succeed")

		for k, v := range expected {
			suite.Equal(v, statInfo.Metadata[k],
				"copied object's metadata must persist under canonicalized key %q", k)
		}
	})
}

func (suite *MinIOServiceTestSuite) uploadTestObject() {
	suite.uploadObject(suite.testObjectKey, suite.testObjectData)
}

func (suite *MinIOServiceTestSuite) uploadObject(key string, data []byte) {
	reader := bytes.NewReader(data)
	_, err := suite.service.PutObject(suite.ctx, storage.PutObjectOptions{
		Key:         key,
		Reader:      reader,
		Size:        int64(len(data)),
		ContentType: suite.testContentType,
	})
	suite.Require().NoError(err, "PutObject should succeed for "+key)
}

// anonymousGet performs an unauthenticated HTTP GET against the MinIO
// endpoint, modeling an external client that bypasses the framework
// and tries to read the object directly. The endpoint URL is derived
// from the container config (http://<endpoint>/<bucket>/<key>); no
// signing is applied so only objects covered by an Allow-anonymous
// bucket policy should respond with 200.
func (suite *MinIOServiceTestSuite) anonymousGet(key string) (int, []byte) {
	endpoint := suite.minioContainer.MinIO.Endpoint
	directURL := "http://" + endpoint + "/" + suite.testBucketName + "/" + key

	req, err := http.NewRequestWithContext(suite.ctx, http.MethodGet, directURL, nil)
	suite.Require().NoError(err, "Anonymous GET request construction should succeed for "+key)

	resp, err := http.DefaultClient.Do(req)
	suite.Require().NoError(err, "Anonymous GET should reach the MinIO endpoint for "+key)

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err, "Reading anonymous GET body should succeed for "+key)

	return resp.StatusCode, body
}

// TestBucketPolicy guards the storage-layer half of the FileACL
// boundary: the framework intentionally scopes the bucket's
// anonymous-read policy to the "pub/" prefix so that "priv/" objects
// can only be read through the proxy middleware (which authenticates
// the caller and consults FileACL). A regression that widened the
// policy back to "<bucket>/*" would let any outside caller fetch
// every priv/ object by hitting the MinIO endpoint directly.
func (suite *MinIOServiceTestSuite) TestBucketPolicy() {
	suite.Run("PubPrefixAllowsAnonymousRead", func() {
		key := "pub/anon-readable.txt"
		body := []byte("public payload")
		suite.uploadObject(key, body)

		status, got := suite.anonymousGet(key)
		suite.Equal(http.StatusOK, status, "Anonymous GET on pub/* must succeed (bucket policy grants s3:GetObject)")
		suite.Equal(body, got, "Anonymous GET body must match the uploaded content for pub/*")
	})

	suite.Run("PrivPrefixDeniesAnonymousRead", func() {
		key := "priv/anon-forbidden.txt"
		body := []byte("private payload")
		suite.uploadObject(key, body)

		status, _ := suite.anonymousGet(key)
		suite.Equal(http.StatusForbidden, status, "Anonymous GET on priv/* must be rejected by MinIO; only the proxy + FileACL path may serve these objects")
	})

	suite.Run("RootLevelKeyDeniesAnonymousRead", func() {
		// Keys outside both pub/ and priv/ must also be inaccessible
		// anonymously — the policy grants only pub/* explicitly.
		key := "loose-key.txt"
		body := []byte("ambiguous payload")
		suite.uploadObject(key, body)

		status, _ := suite.anonymousGet(key)
		suite.Equal(http.StatusForbidden, status, "Anonymous GET on root-level keys must be rejected; pub/* is the only public namespace")
	})
}

func TestMinIOService(t *testing.T) {
	suite.Run(t, new(MinIOServiceTestSuite))
}

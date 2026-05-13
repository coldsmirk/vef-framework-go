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
	suite.Require().True(ok, "MinIO service should implement storage.Multipart")
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

		reader, err := suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
			Key: suite.testObjectKey,
		})

		suite.Require().NoError(err, "GetObject should succeed")

		suite.NotNil(reader, "Reader should not be nil")
		defer reader.Close()

		data, err := io.ReadAll(reader)
		suite.Require().NoError(err, "Reading data should succeed")
		suite.Equal(suite.testObjectData, data, "Data should match uploaded content")
	})

	suite.Run("NotFound", func() {
		reader, err := suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
			Key: "non-existent-key.txt",
		})

		suite.Error(err, "GetObject should return error for non-existent key")
		suite.Nil(reader, "Reader should be nil for non-existent key")
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

		_, err = suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
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
			_, err := suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
				Key: key,
			})
			suite.Error(err, "Deleted object should not be retrievable")
		}
	})
}

func (suite *MinIOServiceTestSuite) TestListObjects() {
	objects := map[string][]byte{
		"folder1/file1.txt": []byte("content1"),
		"folder1/file2.txt": []byte("content2"),
		"folder2/file3.txt": []byte("content3"),
		"root.txt":          []byte("root content"),
	}

	for key, data := range objects {
		suite.uploadObject(key, data)
	}

	suite.Run("ListAll", func() {
		result, err := suite.service.ListObjects(suite.ctx, storage.ListObjectsOptions{
			Recursive: true,
		})

		suite.NoError(err, "ListObjects should succeed")
		suite.Len(result, 4, "Should have 4 objects")
	})

	suite.Run("ListWithPrefix", func() {
		result, err := suite.service.ListObjects(suite.ctx, storage.ListObjectsOptions{
			Prefix:    "folder1/",
			Recursive: true,
		})

		suite.NoError(err, "ListObjects with prefix should succeed")
		suite.Len(result, 2, "Should have 2 objects with prefix")

		for _, obj := range result {
			suite.Contains(obj.Key, "folder1/", "Object key should contain prefix")
		}
	})

	suite.Run("ListWithMaxKeys", func() {
		result, err := suite.service.ListObjects(suite.ctx, storage.ListObjectsOptions{
			Recursive: true,
			MaxKeys:   2,
		})

		suite.NoError(err, "ListObjects with max keys should succeed")
		suite.Equal(2, len(result), "Should respect MaxKeys limit")
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

		reader, err := suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
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

// TestMinIOServiceTestSuite tests MinIO service test suite functionality.
func TestMinIOService(t *testing.T) {
	suite.Run(t, new(MinIOServiceTestSuite))
}

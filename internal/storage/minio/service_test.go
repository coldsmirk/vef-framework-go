package minio

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

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

func (suite *MinIOServiceTestSuite) TestGetPresignedURL() {
	suite.Run("GetMethod", func() {
		suite.uploadTestObject()

		url, err := suite.service.GetPresignedURL(suite.ctx, storage.PresignedURLOptions{
			Key:     suite.testObjectKey,
			Expires: 1 * time.Hour,
			Method:  http.MethodGet,
		})

		suite.NoError(err, "GetPresignedURL should succeed")
		suite.NotEmpty(url, "URL should not be empty")
		suite.Contains(url, suite.testBucketName, "URL should contain bucket name")
		suite.Contains(url, suite.testObjectKey, "URL should contain object key")

		downloadReq, err := http.NewRequestWithContext(suite.ctx, http.MethodGet, url, nil)
		suite.Require().NoError(err, "Creating download request should succeed")

		resp, err := http.DefaultClient.Do(downloadReq)
		suite.Require().NoError(err, "Downloading via presigned URL should succeed")

		defer resp.Body.Close()

		suite.Equal(http.StatusOK, resp.StatusCode, "Download should return 200 OK")
		data, err := io.ReadAll(resp.Body)
		suite.Require().NoError(err, "Reading response body should succeed")
		suite.Equal(suite.testObjectData, data, "Downloaded data should match uploaded content")
	})

	suite.Run("PutMethod", func() {
		url, err := suite.service.GetPresignedURL(suite.ctx, storage.PresignedURLOptions{
			Key:     "presigned-upload.txt",
			Expires: 1 * time.Hour,
			Method:  http.MethodPut,
		})

		suite.NoError(err, "GetPresignedURL for PUT should succeed")
		suite.NotEmpty(url, "URL should not be empty")
		suite.Contains(url, suite.testBucketName, "URL should contain bucket name")

		uploadData := []byte("Uploaded via presigned URL")
		req, err := http.NewRequestWithContext(suite.ctx, http.MethodPut, url, bytes.NewReader(uploadData))
		suite.Require().NoError(err, "Creating upload request should succeed")

		resp, err := http.DefaultClient.Do(req)
		suite.Require().NoError(err, "Uploading via presigned URL should succeed")

		defer resp.Body.Close()

		suite.Equal(http.StatusOK, resp.StatusCode, "Upload should return 200 OK")

		reader, err := suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
			Key: "presigned-upload.txt",
		})
		suite.Require().NoError(err, "Should be able to get uploaded object")

		defer reader.Close()

		data, err := io.ReadAll(reader)
		suite.Require().NoError(err, "Reading uploaded data should succeed")
		suite.Equal(uploadData, data, "Uploaded data should match")
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

func (suite *MinIOServiceTestSuite) TestMoveObject() {
	suite.Run("Success", func() {
		suite.uploadTestObject()

		destKey := "moved-file.txt"
		info, err := suite.service.MoveObject(suite.ctx, storage.MoveObjectOptions{
			CopyObjectOptions: storage.CopyObjectOptions{
				SourceKey: suite.testObjectKey,
				DestKey:   destKey,
			},
		})

		suite.NoError(err, "MoveObject should succeed")
		suite.NotNil(info, "ObjectInfo should not be nil")
		suite.Equal(suite.testBucketName, info.Bucket, "Bucket should match")
		suite.Equal(destKey, info.Key, "Destination key should match")

		reader, err := suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
			Key: destKey,
		})
		suite.Require().NoError(err, "Should be able to get moved object")

		defer reader.Close()

		data, err := io.ReadAll(reader)
		suite.Require().NoError(err, "Reading moved data should succeed")
		suite.Equal(suite.testObjectData, data, "Moved data should match original")

		_, err = suite.service.GetObject(suite.ctx, storage.GetObjectOptions{
			Key: suite.testObjectKey,
		})
		suite.Error(err, "Source object should be deleted after move")
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

func (suite *MinIOServiceTestSuite) TestPromoteObject() {
	suite.Run("Success", func() {
		tempKey := storage.TempPrefix + "2025/01/15/test-promote.txt"
		content := []byte("Content to be promoted")
		suite.uploadObject(tempKey, content)

		info, err := suite.service.PromoteObject(suite.ctx, tempKey)
		suite.Require().NoError(err, "PromoteObject should succeed")
		suite.NotNil(info, "ObjectInfo should not be nil")

		expectedKey := "2025/01/15/test-promote.txt"
		suite.Equal(expectedKey, info.Key, "Key should not have temp prefix")
		suite.Equal(suite.testBucketName, info.Bucket, "Bucket should match")

		_, err = suite.service.StatObject(suite.ctx, storage.StatObjectOptions{Key: tempKey})
		suite.Error(err, "Temp file should be deleted after promotion")
		suite.Equal(storage.ErrObjectNotFound, err, "Error should be ErrObjectNotFound")

		permanentInfo, err := suite.service.StatObject(suite.ctx, storage.StatObjectOptions{Key: expectedKey})
		suite.NoError(err, "Should be able to stat promoted object")
		suite.Equal(expectedKey, permanentInfo.Key, "Promoted key should match")
		suite.Equal(int64(len(content)), permanentInfo.Size, "Promoted size should match")
	})

	suite.Run("NonTempKey", func() {
		normalKey := "normal/file.txt"
		content := []byte("Normal file content")
		suite.uploadObject(normalKey, content)

		info, err := suite.service.PromoteObject(suite.ctx, normalKey)
		suite.NoError(err, "PromoteObject should not error for non-temp key")
		suite.Nil(info, "PromoteObject should return nil for non-temp keys")

		originalInfo, err := suite.service.StatObject(suite.ctx, storage.StatObjectOptions{Key: normalKey})
		suite.NoError(err, "Original file should still exist")
		suite.Equal(normalKey, originalInfo.Key, "Original key should be unchanged")
	})

	suite.Run("NotFound", func() {
		tempKey := storage.TempPrefix + "non-existent.txt"

		info, err := suite.service.PromoteObject(suite.ctx, tempKey)
		suite.Error(err, "PromoteObject should return error for non-existent temp file")
		suite.Nil(info, "ObjectInfo should be nil for non-existent file")
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

// TestMinIOServiceTestSuite tests MinIO service test suite functionality.
func TestMinIOService(t *testing.T) {
	suite.Run(t, new(MinIOServiceTestSuite))
}

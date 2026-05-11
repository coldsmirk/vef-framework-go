package minio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// S3 protocol constants for multipart uploads.
const (
	// minPartSize is the smallest non-final part size accepted by S3 / MinIO.
	minPartSize int64 = 5 * 1024 * 1024
	// maxPartCount is the largest number of parts in a single multipart upload.
	maxPartCount = 10000
)

type Service struct {
	client *minio.Client
	core   *minio.Core
	bucket string
}

func New(cfg config.MinIOConfig, appCfg *config.AppConfig) (storage.Service, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	return &Service{
		client: client,
		core:   &minio.Core{Client: client},
		bucket: lo.CoalesceOrEmpty(cfg.Bucket, appCfg.Name, "vef-app"),
	}, nil
}

func (*Service) Capabilities() storage.ServiceCapabilities {
	return storage.ServiceCapabilities{
		Multipart:     true,
		PresignedPut:  true,
		PresignedGet:  true,
		PresignedPart: true,
		MinPartSize:   minPartSize,
		MaxPartCount:  maxPartCount,
	}
}

func (s *Service) Init(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", s.bucket, err)
		}

		// Set public read policy for the bucket
		policy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": {"AWS": ["*"]},
					"Action": ["s3:GetObject"],
					"Resource": ["arn:aws:s3:::%s/*"]
				}
			]
		}`, s.bucket)

		if err := s.client.SetBucketPolicy(ctx, s.bucket, policy); err != nil {
			return fmt.Errorf("failed to set public read policy for bucket %s: %w", s.bucket, err)
		}
	}

	return nil
}

func (s *Service) PutObject(ctx context.Context, opts storage.PutObjectOptions) (*storage.ObjectInfo, error) {
	uploadOpts := minio.PutObjectOptions{
		ContentType:  opts.ContentType,
		UserMetadata: opts.Metadata,
	}

	info, err := s.client.PutObject(ctx, s.bucket, opts.Key, opts.Reader, opts.Size, uploadOpts)
	if err != nil {
		return nil, s.translateError(err)
	}

	return &storage.ObjectInfo{
		Bucket:       info.Bucket,
		Key:          info.Key,
		ETag:         info.ETag,
		Size:         info.Size,
		ContentType:  opts.ContentType,
		LastModified: info.LastModified,
		Metadata:     opts.Metadata,
	}, nil
}

func (s *Service) GetObject(ctx context.Context, opts storage.GetObjectOptions) (io.ReadCloser, error) {
	object, err := s.client.GetObject(ctx, s.bucket, opts.Key, minio.GetObjectOptions{})
	if err != nil {
		return nil, s.translateError(err)
	}

	if _, err = object.Stat(); err != nil {
		_ = object.Close()

		return nil, s.translateError(err)
	}

	return object, nil
}

func (s *Service) DeleteObject(ctx context.Context, opts storage.DeleteObjectOptions) error {
	if err := s.client.RemoveObject(ctx, s.bucket, opts.Key, minio.RemoveObjectOptions{}); err != nil {
		return s.translateError(err)
	}

	return nil
}

func (s *Service) DeleteObjects(ctx context.Context, opts storage.DeleteObjectsOptions) error {
	objectsCh := make(chan minio.ObjectInfo, len(opts.Keys))

	go func() {
		defer close(objectsCh)

		for _, key := range opts.Keys {
			objectsCh <- minio.ObjectInfo{Key: key}
		}
	}()

	errorCh := s.client.RemoveObjects(ctx, s.bucket, objectsCh, minio.RemoveObjectsOptions{})

	for err := range errorCh {
		if err.Err != nil {
			return s.translateError(err.Err)
		}
	}

	return nil
}

func (s *Service) ListObjects(ctx context.Context, opts storage.ListObjectsOptions) ([]storage.ObjectInfo, error) {
	listOpts := minio.ListObjectsOptions{
		Prefix:       opts.Prefix,
		Recursive:    opts.Recursive,
		MaxKeys:      opts.MaxKeys,
		WithMetadata: true,
	}

	var objects []storage.ObjectInfo

	for object := range s.client.ListObjects(ctx, s.bucket, listOpts) {
		if object.Err != nil {
			return nil, s.translateError(object.Err)
		}

		objects = append(objects, storage.ObjectInfo{
			Bucket:       s.bucket,
			Key:          object.Key,
			ETag:         object.ETag,
			Size:         object.Size,
			ContentType:  object.ContentType,
			LastModified: object.LastModified,
			Metadata:     object.UserMetadata,
		})

		if opts.MaxKeys > 0 && len(objects) >= opts.MaxKeys {
			break
		}
	}

	return objects, nil
}

func (s *Service) GetPresignedURL(ctx context.Context, opts storage.PresignedURLOptions) (string, error) {
	var (
		u   *url.URL
		err error
	)

	switch opts.Method {
	case http.MethodGet, "":
		u, err = s.client.PresignedGetObject(ctx, s.bucket, opts.Key, opts.Expires, nil)
	case http.MethodPut:
		u, err = s.client.PresignedPutObject(ctx, s.bucket, opts.Key, opts.Expires)
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedHTTPMethod, opts.Method)
	}

	if err != nil {
		return "", s.translateError(err)
	}

	return u.String(), nil
}

func (s *Service) CopyObject(ctx context.Context, opts storage.CopyObjectOptions) (*storage.ObjectInfo, error) {
	src := minio.CopySrcOptions{
		Bucket: s.bucket,
		Object: opts.SourceKey,
	}

	dst := minio.CopyDestOptions{
		Bucket: s.bucket,
		Object: opts.DestKey,
	}

	info, err := s.client.CopyObject(ctx, dst, src)
	if err != nil {
		return nil, s.translateError(err)
	}

	return &storage.ObjectInfo{
		Bucket:       info.Bucket,
		Key:          info.Key,
		ETag:         info.ETag,
		Size:         info.Size,
		LastModified: info.LastModified,
	}, nil
}

func (s *Service) StatObject(ctx context.Context, opts storage.StatObjectOptions) (*storage.ObjectInfo, error) {
	info, err := s.client.StatObject(ctx, s.bucket, opts.Key, minio.StatObjectOptions{})
	if err != nil {
		return nil, s.translateError(err)
	}

	return &storage.ObjectInfo{
		Bucket:       s.bucket,
		Key:          info.Key,
		ETag:         info.ETag,
		Size:         info.Size,
		ContentType:  info.ContentType,
		LastModified: info.LastModified,
		Metadata:     info.UserMetadata,
	}, nil
}

func (s *Service) PresignPutObject(ctx context.Context, opts storage.PresignPutOptions) (*storage.PresignedURL, error) {
	u, err := s.client.PresignedPutObject(ctx, s.bucket, opts.Key, opts.Expires)
	if err != nil {
		return nil, s.translateError(err)
	}

	headers := map[string]string{}
	if opts.ContentType != "" {
		headers["Content-Type"] = opts.ContentType
	}

	return &storage.PresignedURL{
		URL:       u.String(),
		Method:    http.MethodPut,
		Headers:   headers,
		ExpiresAt: time.Now().Add(opts.Expires),
	}, nil
}

func (s *Service) InitMultipart(ctx context.Context, opts storage.InitMultipartOptions) (*storage.MultipartSession, error) {
	uploadID, err := s.core.NewMultipartUpload(ctx, s.bucket, opts.Key, minio.PutObjectOptions{
		ContentType:  opts.ContentType,
		UserMetadata: opts.Metadata,
	})
	if err != nil {
		return nil, s.translateError(err)
	}

	return &storage.MultipartSession{
		Key:      opts.Key,
		UploadID: uploadID,
	}, nil
}

func (s *Service) PresignPart(ctx context.Context, opts storage.PresignPartOptions) (*storage.PresignedURL, error) {
	if opts.PartNumber < 1 {
		return nil, fmt.Errorf("storage(minio): partNumber must be >= 1, got %d", opts.PartNumber)
	}

	params := url.Values{
		"partNumber": []string{strconv.Itoa(opts.PartNumber)},
		"uploadId":   []string{opts.UploadID},
	}

	u, err := s.client.Presign(ctx, http.MethodPut, s.bucket, opts.Key, opts.Expires, params)
	if err != nil {
		return nil, s.translateError(err)
	}

	return &storage.PresignedURL{
		URL:       u.String(),
		Method:    http.MethodPut,
		ExpiresAt: time.Now().Add(opts.Expires),
	}, nil
}

func (s *Service) CompleteMultipart(ctx context.Context, opts storage.CompleteMultipartOptions) (*storage.ObjectInfo, error) {
	parts := make([]minio.CompletePart, len(opts.Parts))
	for i, p := range opts.Parts {
		parts[i] = minio.CompletePart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
	}

	info, err := s.core.CompleteMultipartUpload(ctx, s.bucket, opts.Key, opts.UploadID, parts, minio.PutObjectOptions{})
	if err != nil {
		return nil, s.translateError(err)
	}

	return &storage.ObjectInfo{
		Bucket:       info.Bucket,
		Key:          info.Key,
		ETag:         info.ETag,
		Size:         info.Size,
		LastModified: info.LastModified,
	}, nil
}

func (s *Service) AbortMultipart(ctx context.Context, opts storage.AbortMultipartOptions) error {
	if err := s.core.AbortMultipartUpload(ctx, s.bucket, opts.Key, opts.UploadID); err != nil {
		return s.translateError(err)
	}

	return nil
}

func (*Service) translateError(err error) error {
	if err == nil {
		return nil
	}

	var minioErr minio.ErrorResponse
	if ok := errors.As(err, &minioErr); !ok {
		return err
	}

	switch minioErr.Code {
	case "NoSuchBucket":
		return storage.ErrBucketNotFound
	case "NoSuchKey":
		return storage.ErrObjectNotFound
	case "InvalidBucketName":
		return storage.ErrInvalidBucketName
	case "AccessDenied":
		return storage.ErrAccessDenied
	default:
		return err
	}
}

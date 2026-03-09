package minio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/storage"
)

type Service struct {
	client *minio.Client
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
		bucket: lo.CoalesceOrEmpty(cfg.Bucket, appCfg.Name, "vef-app"),
	}, nil
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

func (s *Service) MoveObject(ctx context.Context, opts storage.MoveObjectOptions) (info *storage.ObjectInfo, err error) {
	if info, err = s.CopyObject(ctx, opts.CopyObjectOptions); err != nil {
		return info, err
	}

	if err = s.DeleteObject(ctx, storage.DeleteObjectOptions{
		Key: opts.SourceKey,
	}); err != nil {
		return nil, fmt.Errorf("copied successfully but failed to delete source: %w", err)
	}

	return info, err
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

func (s *Service) PromoteObject(ctx context.Context, tempKey string) (*storage.ObjectInfo, error) {
	if !strings.HasPrefix(tempKey, storage.TempPrefix) {
		return nil, nil
	}

	permanentKey := strings.TrimPrefix(tempKey, storage.TempPrefix)

	return s.MoveObject(ctx, storage.MoveObjectOptions{
		CopyObjectOptions: storage.CopyObjectOptions{
			SourceKey: tempKey,
			DestKey:   permanentKey,
		},
	})
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

package minio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// S3 protocol constants for multipart uploads.
const (
	// partSize is the authoritative part size for chunked uploads via
	// this backend. Chosen to be comfortably above the S3 minimum
	// (5 MiB) while keeping per-part overhead low.
	partSize int64 = 16 * 1024 * 1024
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

func (*Service) PartSize() int64   { return partSize }
func (*Service) MaxPartCount() int { return maxPartCount }

// buildPublicPrefixPolicy constructs the bucket policy JSON that grants
// anonymous s3:GetObject only on objects whose key starts with "pub/".
// Uses json.Marshal to avoid string-interpolation injection risks.
func buildPublicPrefixPolicy(bucket string) (string, error) {
	policy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect":    "Allow",
				"Principal": map[string]any{"AWS": []string{"*"}},
				"Action":    []string{"s3:GetObject"},
				"Resource":  []string{"arn:aws:s3:::" + bucket + "/pub/*"},
			},
		},
	}

	data, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (s *Service) Init(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if exists {
		return nil
	}

	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("failed to create bucket %s: %w", s.bucket, err)
	}

	policy, err := buildPublicPrefixPolicy(s.bucket)
	if err != nil {
		return fmt.Errorf("failed to build pub/* policy for bucket %s: %w", s.bucket, err)
	}

	if err := s.client.SetBucketPolicy(ctx, s.bucket, policy); err != nil {
		return fmt.Errorf("failed to set pub/* public-read policy for bucket %s: %w", s.bucket, err)
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

// ── Multipart ───────────────────────────────────────────────────────────

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

func (s *Service) PutPart(ctx context.Context, opts storage.PutPartOptions) (*storage.PartInfo, error) {
	part, err := s.core.PutObjectPart(ctx, s.bucket, opts.Key, opts.UploadID, opts.PartNumber, opts.Reader, opts.Size, minio.PutObjectPartOptions{})
	if err != nil {
		return nil, s.translateError(err)
	}

	return &storage.PartInfo{
		PartNumber: part.PartNumber,
		ETag:       part.ETag,
		Size:       part.Size,
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
		translated := s.translateError(err)
		// AbortMultipart is idempotent: unknown session → nil.
		if errors.Is(translated, storage.ErrUploadSessionNotFound) {
			return nil
		}

		return translated
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
	case "NoSuchUpload":
		return storage.ErrUploadSessionNotFound
	case "InvalidPart":
		return storage.ErrPartETagMismatch
	case "EntityTooSmall":
		return storage.ErrPartTooSmall
	default:
		return err
	}
}

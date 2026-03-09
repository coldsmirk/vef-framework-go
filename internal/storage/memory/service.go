package memory

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/storage"
)

// Service is intended for testing purposes only.
type Service struct {
	mu      sync.RWMutex
	objects map[string]*objectData
}

type objectData struct {
	data         []byte
	contentType  string
	metadata     map[string]string
	lastModified time.Time
}

func New() storage.Service {
	return &Service{
		objects: make(map[string]*objectData),
	}
}

func (s *Service) PutObject(_ context.Context, opts storage.PutObjectOptions) (*storage.ObjectInfo, error) {
	data, err := io.ReadAll(opts.Reader)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.objects[opts.Key] = &objectData{
		data:         data,
		contentType:  opts.ContentType,
		metadata:     opts.Metadata,
		lastModified: now,
	}

	return &storage.ObjectInfo{
		Bucket:       "memory",
		Key:          opts.Key,
		ETag:         cast.ToString(now.UnixNano()),
		Size:         int64(len(data)),
		ContentType:  opts.ContentType,
		LastModified: now,
		Metadata:     opts.Metadata,
	}, nil
}

func (s *Service) GetObject(_ context.Context, opts storage.GetObjectOptions) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	obj, exists := s.objects[opts.Key]
	if !exists {
		return nil, storage.ErrObjectNotFound
	}

	return io.NopCloser(bytes.NewReader(obj.data)), nil
}

func (s *Service) DeleteObject(_ context.Context, opts storage.DeleteObjectOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.objects, opts.Key)

	return nil
}

func (s *Service) DeleteObjects(_ context.Context, opts storage.DeleteObjectsOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, key := range opts.Keys {
		delete(s.objects, key)
	}

	return nil
}

func (s *Service) ListObjects(_ context.Context, opts storage.ListObjectsOptions) ([]storage.ObjectInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var objects []storage.ObjectInfo

	for key, obj := range s.objects {
		if opts.Prefix != "" && !strings.HasPrefix(key, opts.Prefix) {
			continue
		}

		if !opts.Recursive {
			relativeKey := strings.TrimPrefix(key, opts.Prefix)
			if strings.Contains(relativeKey, "/") {
				continue
			}
		}

		objects = append(objects, storage.ObjectInfo{
			Bucket:       "memory",
			Key:          key,
			ETag:         cast.ToString(obj.lastModified.UnixNano()),
			Size:         int64(len(obj.data)),
			ContentType:  obj.contentType,
			LastModified: obj.lastModified,
			Metadata:     obj.metadata,
		})

		if opts.MaxKeys > 0 && len(objects) >= opts.MaxKeys {
			break
		}
	}

	return objects, nil
}

func (*Service) GetPresignedURL(_ context.Context, opts storage.PresignedURLOptions) (string, error) {
	return fmt.Sprintf("memory://%s?method=%s&expires=%d", opts.Key, opts.Method, opts.Expires), nil
}

func (s *Service) CopyObject(_ context.Context, opts storage.CopyObjectOptions) (*storage.ObjectInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	source, exists := s.objects[opts.SourceKey]
	if !exists {
		return nil, storage.ErrObjectNotFound
	}

	dataCopy := make([]byte, len(source.data))
	copy(dataCopy, source.data)

	metadataCopy := make(map[string]string, len(source.metadata))
	maps.Copy(metadataCopy, source.metadata)

	now := time.Now()
	s.objects[opts.DestKey] = &objectData{
		data:         dataCopy,
		contentType:  source.contentType,
		metadata:     metadataCopy,
		lastModified: now,
	}

	return &storage.ObjectInfo{
		Bucket:       "memory",
		Key:          opts.DestKey,
		ETag:         cast.ToString(now.UnixNano()),
		Size:         int64(len(dataCopy)),
		ContentType:  source.contentType,
		LastModified: now,
		Metadata:     metadataCopy,
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

func (s *Service) StatObject(_ context.Context, opts storage.StatObjectOptions) (*storage.ObjectInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	obj, exists := s.objects[opts.Key]
	if !exists {
		return nil, storage.ErrObjectNotFound
	}

	return &storage.ObjectInfo{
		Bucket:       "memory",
		Key:          opts.Key,
		ETag:         cast.ToString(obj.lastModified.UnixNano()),
		Size:         int64(len(obj.data)),
		ContentType:  obj.contentType,
		LastModified: obj.lastModified,
		Metadata:     obj.metadata,
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

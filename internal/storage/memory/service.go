package memory

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/storage"
)

const partSize int64 = 64 * 1024 // 64 KiB — small for fast multi-part test coverage

// Service is intended for testing purposes only.
type Service struct {
	mu       sync.RWMutex
	objects  map[string]*objectData
	sessions map[string]*multipartSession
}

type objectData struct {
	data         []byte
	etag         string
	contentType  string
	metadata     map[string]string
	lastModified time.Time
}

type multipartSession struct {
	key         string
	contentType string
	metadata    map[string]string
	parts       map[int]*memoryPart
}

type memoryPart struct {
	data []byte
	etag string
	size int64
}

func New() storage.Service {
	return &Service{
		objects:  make(map[string]*objectData),
		sessions: make(map[string]*multipartSession),
	}
}

func (s *Service) PutObject(_ context.Context, opts storage.PutObjectOptions) (*storage.ObjectInfo, error) {
	data, err := io.ReadAll(opts.Reader)
	if err != nil {
		return nil, err
	}

	h := md5.Sum(data)
	etag := hex.EncodeToString(h[:])

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.objects[opts.Key] = &objectData{
		data:         data,
		etag:         etag,
		contentType:  opts.ContentType,
		metadata:     opts.Metadata,
		lastModified: now,
	}

	return &storage.ObjectInfo{
		Bucket:       "memory",
		Key:          opts.Key,
		ETag:         etag,
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
			ETag:         obj.etag,
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

	copyHash := md5.Sum(dataCopy)
	copyEtag := hex.EncodeToString(copyHash[:])

	now := time.Now()
	s.objects[opts.DestKey] = &objectData{
		data:         dataCopy,
		etag:         copyEtag,
		contentType:  source.contentType,
		metadata:     metadataCopy,
		lastModified: now,
	}

	return &storage.ObjectInfo{
		Bucket:       "memory",
		Key:          opts.DestKey,
		ETag:         copyEtag,
		Size:         int64(len(dataCopy)),
		ContentType:  source.contentType,
		LastModified: now,
		Metadata:     metadataCopy,
	}, nil
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
		ETag:         obj.etag,
		Size:         int64(len(obj.data)),
		ContentType:  obj.contentType,
		LastModified: obj.lastModified,
		Metadata:     obj.metadata,
	}, nil
}

func (*Service) PartSize() int64   { return partSize }
func (*Service) MaxPartCount() int { return 0 }

// ── Multipart ───────────────────────────────────────────────────────────

func (s *Service) InitMultipart(_ context.Context, opts storage.InitMultipartOptions) (*storage.MultipartSession, error) {
	uploadID := id.GenerateUUID()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[uploadID] = &multipartSession{
		key:         opts.Key,
		contentType: opts.ContentType,
		metadata:    opts.Metadata,
		parts:       make(map[int]*memoryPart),
	}

	return &storage.MultipartSession{
		Key:      opts.Key,
		UploadID: uploadID,
	}, nil
}

func (s *Service) PutPart(_ context.Context, opts storage.PutPartOptions) (*storage.PartInfo, error) {
	data, err := io.ReadAll(opts.Reader)
	if err != nil {
		return nil, err
	}

	h := md5.Sum(data)
	etag := hex.EncodeToString(h[:])

	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[opts.UploadID]
	if !exists {
		return nil, storage.ErrUploadSessionNotFound
	}

	session.parts[opts.PartNumber] = &memoryPart{
		data: data,
		etag: etag,
		size: int64(len(data)),
	}

	return &storage.PartInfo{
		PartNumber: opts.PartNumber,
		ETag:       etag,
		Size:       int64(len(data)),
	}, nil
}

func (s *Service) CompleteMultipart(_ context.Context, opts storage.CompleteMultipartOptions) (*storage.ObjectInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(opts.Parts) == 0 {
		return nil, storage.ErrPartNumberOutOfRange
	}

	session, exists := s.sessions[opts.UploadID]
	if !exists {
		return nil, storage.ErrUploadSessionNotFound
	}

	// Verify parts: contiguous 1..N, ETags match, non-final size check.
	for i, cp := range opts.Parts {
		expected := i + 1
		if cp.PartNumber != expected {
			return nil, storage.ErrPartNumberOutOfRange
		}

		part, ok := session.parts[cp.PartNumber]
		if !ok {
			return nil, storage.ErrPartNumberOutOfRange
		}

		if part.etag != cp.ETag {
			return nil, storage.ErrPartETagMismatch
		}

		// Non-final parts must be at least partSize (contract §2).
		if i < len(opts.Parts)-1 && part.size < partSize {
			return nil, storage.ErrPartTooSmall
		}
	}

	// Assemble final object.
	var totalSize int64
	for _, cp := range opts.Parts {
		totalSize += session.parts[cp.PartNumber].size
	}

	assembled := make([]byte, 0, totalSize)
	for _, cp := range opts.Parts {
		assembled = append(assembled, session.parts[cp.PartNumber].data...)
	}

	assembledHash := md5.Sum(assembled)
	assembledEtag := hex.EncodeToString(assembledHash[:])

	now := time.Now()
	s.objects[session.key] = &objectData{
		data:         assembled,
		etag:         assembledEtag,
		contentType:  session.contentType,
		metadata:     session.metadata,
		lastModified: now,
	}

	delete(s.sessions, opts.UploadID)

	return &storage.ObjectInfo{
		Bucket:       "memory",
		Key:          session.key,
		ETag:         assembledEtag,
		Size:         totalSize,
		ContentType:  session.contentType,
		LastModified: now,
		Metadata:     session.metadata,
	}, nil
}

func (s *Service) AbortMultipart(_ context.Context, opts storage.AbortMultipartOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, opts.UploadID)

	return nil
}

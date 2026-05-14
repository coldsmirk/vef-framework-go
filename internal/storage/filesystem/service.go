package filesystem

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/coldsmirk/go-streams"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/storage"
)

const (
	partSize int64 = 4 * 1024 * 1024 // 4 MiB

	bucketName = "filesystem"

	// multipartDir is the hidden directory under root that holds in-flight
	// multipart sessions. Each session gets its own subdirectory keyed by
	// uploadID.
	multipartDir = ".multipart"

	// tmpDir is the hidden directory under root that holds in-flight
	// assembly temp files. Skipped by ListObjects and never cleaned up
	// by cleanupEmptyDirs.
	tmpDir = ".tmp"

	// etagsDir is the hidden directory under root that mirrors the object
	// tree and stores one small text file per object containing its MD5
	// ETag. Persisting the ETag at write time avoids re-reading the entire
	// object on every StatObject call (which the file proxy invokes per
	// request). Skipped by ListObjects.
	etagsDir = ".etags"
)

type manifest struct {
	Key         string `json:"key"`
	ContentType string `json:"contentType"`
}

type Service struct {
	root string
}

// New creates a filesystem storage service.
//
// Root MUST point to a shared volume (NFS, CephFS, EFS, k8s
// ReadWriteMany PVC) when more than one application instance is
// deployed; otherwise PutPart and CompleteMultipart from different
// instances will not see each other's parts.
func New(cfg config.FilesystemConfig) (storage.Service, error) {
	root := cfg.Root
	if root == "" {
		root = "./storage"
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create storage root directory: %w", err)
	}

	return &Service{root: root}, nil
}

func (s *Service) resolvePath(key string) string {
	return filepath.Join(s.root, filepath.FromSlash(key))
}

func (s *Service) etagPath(key string) string {
	return filepath.Join(s.root, etagsDir, filepath.FromSlash(key))
}

func (s *Service) sessionDir(uploadID string) string {
	return filepath.Join(s.root, multipartDir, uploadID)
}

// writeFileAtomic writes data to path via tmp-file + rename. The tmp
// suffix is unique per call so concurrent writers to the same path each
// produce a complete artifact in isolation; the final Rename yields
// last-writer-wins on path.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("filesystem: mkdir: %w", err)
	}

	tmp := path + ".tmp." + id.GenerateUUID()

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("filesystem: write tmp: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)

		return fmt.Errorf("filesystem: rename tmp: %w", err)
	}

	return nil
}

// writeETag persists etag to the sidecar tree for key. Failures are
// non-fatal for the caller's main operation — the ETag is a cache hint,
// not a correctness invariant — so the error is returned for logging
// rather than propagated as a write failure.
func (s *Service) writeETag(key, etag string) error {
	return writeFileAtomic(s.etagPath(key), []byte(etag))
}

// readETag returns the persisted ETag for key, or "" if no sidecar
// exists (e.g. object written by an older version, or sidecar removed).
// A missing sidecar is not an error: callers fall back to an empty ETag.
func (s *Service) readETag(key string) (string, error) {
	data, err := os.ReadFile(s.etagPath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}

		return "", err
	}

	return string(data), nil
}

// removeETag deletes the sidecar for key. Idempotent.
func (s *Service) removeETag(key string) {
	path := s.etagPath(key)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return
	}

	s.cleanupEmptyDirs(filepath.Dir(path))
}

func (s *Service) PutObject(_ context.Context, opts storage.PutObjectOptions) (*storage.ObjectInfo, error) {
	path := s.resolvePath(opts.Key)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	defer func() { _ = file.Close() }()

	hasher := md5.New()
	writer := io.MultiWriter(file, hasher)

	written, err := io.Copy(writer, opts.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	etag := hex.EncodeToString(hasher.Sum(nil))

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if err := s.writeETag(opts.Key, etag); err != nil {
		return nil, err
	}

	return &storage.ObjectInfo{
		Bucket:       bucketName,
		Key:          opts.Key,
		ETag:         etag,
		Size:         written,
		ContentType:  opts.ContentType,
		LastModified: stat.ModTime(),
		Metadata:     opts.Metadata,
	}, nil
}

func (s *Service) GetObject(_ context.Context, opts storage.GetObjectOptions) (io.ReadCloser, error) {
	path := s.resolvePath(opts.Key)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrObjectNotFound
		}

		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

func (s *Service) DeleteObject(_ context.Context, opts storage.DeleteObjectOptions) error {
	path := s.resolvePath(opts.Key)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	s.cleanupEmptyDirs(filepath.Dir(path))
	s.removeETag(opts.Key)

	return nil
}

func (s *Service) DeleteObjects(ctx context.Context, opts storage.DeleteObjectsOptions) error {
	return streams.FromSlice(opts.Keys).ForEachErr(func(key string) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		return s.DeleteObject(ctx, storage.DeleteObjectOptions{Key: key})
	})
}

func (s *Service) ListObjects(_ context.Context, opts storage.ListObjectsOptions) ([]storage.ObjectInfo, error) {
	var objects []storage.ObjectInfo

	prefix := opts.Prefix
	searchPath := s.resolvePath(prefix)

	err := filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) || os.IsNotExist(err) {
				return nil
			}

			return err
		}

		if d.IsDir() {
			// Skip hidden infrastructure directories entirely.
			if d.Name() == multipartDir || d.Name() == tmpDir || d.Name() == etagsDir {
				return filepath.SkipDir
			}

			return nil
		}

		relPath, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}

		key := filepath.ToSlash(relPath)

		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}

		if !opts.Recursive {
			relativeKey := strings.TrimPrefix(key, prefix)
			if strings.Contains(relativeKey, "/") {
				return nil
			}
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		contentType := mime.TypeByExtension(filepath.Ext(path))

		objects = append(objects, storage.ObjectInfo{
			Bucket:       bucketName,
			Key:          key,
			ETag:         "",
			Size:         info.Size(),
			ContentType:  contentType,
			LastModified: info.ModTime(),
		})

		if opts.MaxKeys > 0 && len(objects) >= opts.MaxKeys {
			return io.EOF
		}

		return nil
	})

	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	return objects, nil
}

func (s *Service) CopyObject(_ context.Context, opts storage.CopyObjectOptions) (*storage.ObjectInfo, error) {
	srcPath := s.resolvePath(opts.SourceKey)
	destPath := s.resolvePath(opts.DestKey)

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrObjectNotFound
		}

		return nil, fmt.Errorf("failed to open source file: %w", err)
	}

	defer func() { _ = src.Close() }()

	dest, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}

	defer func() { _ = dest.Close() }()

	hasher := md5.New()
	writer := io.MultiWriter(dest, hasher)

	written, err := io.Copy(writer, src)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	etag := hex.EncodeToString(hasher.Sum(nil))

	stat, err := dest.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat destination file: %w", err)
	}

	if err := s.writeETag(opts.DestKey, etag); err != nil {
		return nil, err
	}

	contentType := mime.TypeByExtension(filepath.Ext(destPath))

	return &storage.ObjectInfo{
		Bucket:       bucketName,
		Key:          opts.DestKey,
		ETag:         etag,
		Size:         written,
		ContentType:  contentType,
		LastModified: stat.ModTime(),
	}, nil
}

func (s *Service) StatObject(_ context.Context, opts storage.StatObjectOptions) (*storage.ObjectInfo, error) {
	path := s.resolvePath(opts.Key)

	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrObjectNotFound
		}

		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Read the ETag from its sidecar — written at PutObject /
	// CompleteMultipart / CopyObject time — to avoid re-hashing the
	// entire file on every Stat. Missing sidecar yields an empty ETag,
	// which the proxy treats as "no validator" rather than an error.
	etag, err := s.readETag(opts.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to read etag sidecar: %w", err)
	}

	contentType := mime.TypeByExtension(filepath.Ext(path))

	return &storage.ObjectInfo{
		Bucket:       bucketName,
		Key:          opts.Key,
		ETag:         etag,
		Size:         stat.Size(),
		ContentType:  contentType,
		LastModified: stat.ModTime(),
	}, nil
}

func (*Service) PartSize() int64   { return partSize }
func (*Service) MaxPartCount() int { return 0 }

// ── Multipart ───────────────────────────────────────────────────────────

func (s *Service) InitMultipart(_ context.Context, opts storage.InitMultipartOptions) (*storage.MultipartSession, error) {
	uploadID := id.GenerateUUID()
	dir := s.sessionDir(uploadID)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("filesystem: init multipart mkdir: %w", err)
	}

	m := manifest{Key: opts.Key, ContentType: opts.ContentType}

	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("filesystem: marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(dir, "manifest.json")

	manifestFile, err := os.Create(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("filesystem: create manifest: %w", err)
	}

	if _, err := manifestFile.Write(data); err != nil {
		_ = manifestFile.Close()

		return nil, fmt.Errorf("filesystem: write manifest: %w", err)
	}

	if err := manifestFile.Sync(); err != nil {
		_ = manifestFile.Close()

		return nil, fmt.Errorf("filesystem: sync manifest: %w", err)
	}

	if err := manifestFile.Close(); err != nil {
		return nil, fmt.Errorf("filesystem: close manifest: %w", err)
	}

	return &storage.MultipartSession{
		Key:      opts.Key,
		UploadID: uploadID,
	}, nil
}

func (s *Service) PutPart(_ context.Context, opts storage.PutPartOptions) (*storage.PartInfo, error) {
	dir := s.sessionDir(opts.UploadID)

	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrUploadSessionNotFound
		}

		return nil, err
	}

	partPath := filepath.Join(dir, strconv.Itoa(opts.PartNumber)+".part")
	// Unique per-call tmp suffix is required: a fixed `<n>.part.tmp`
	// path would still allow two concurrent PutPart calls for the
	// same PartNumber to interleave bytes inside the tmp file before
	// either rename runs. With a unique tmp file each writer produces
	// a complete artifact in isolation; the final atomic Rename then
	// gives us last-writer-wins on `<n>.part`.
	tmpPartPath := partPath + ".tmp." + id.GenerateUUID()

	file, err := os.Create(tmpPartPath)
	if err != nil {
		return nil, fmt.Errorf("filesystem: create part file: %w", err)
	}

	hasher := md5.New()
	writer := io.MultiWriter(file, hasher)

	written, copyErr := io.Copy(writer, opts.Reader)
	if copyErr != nil {
		_ = file.Close()
		_ = os.Remove(tmpPartPath)

		return nil, fmt.Errorf("filesystem: write part: %w", copyErr)
	}

	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPartPath)

		return nil, fmt.Errorf("filesystem: sync part: %w", err)
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPartPath)

		return nil, fmt.Errorf("filesystem: close part: %w", err)
	}

	if err := os.Rename(tmpPartPath, partPath); err != nil {
		_ = os.Remove(tmpPartPath)

		return nil, fmt.Errorf("filesystem: rename part: %w", err)
	}

	etag := hex.EncodeToString(hasher.Sum(nil))

	// Persist etag alongside the part for verification at Complete time.
	// The atomic tmp+rename keeps the recorded etag consistent with
	// whichever .part file ultimately wins the rename race.
	etagPath := filepath.Join(dir, strconv.Itoa(opts.PartNumber)+".etag")
	if err := writeFileAtomic(etagPath, []byte(etag)); err != nil {
		return nil, err
	}

	return &storage.PartInfo{
		PartNumber: opts.PartNumber,
		ETag:       etag,
		Size:       written,
	}, nil
}

func (s *Service) CompleteMultipart(_ context.Context, opts storage.CompleteMultipartOptions) (*storage.ObjectInfo, error) {
	if len(opts.Parts) == 0 {
		return nil, storage.ErrPartNumberOutOfRange
	}

	dir := s.sessionDir(opts.UploadID)

	manifestData, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrUploadSessionNotFound
		}

		return nil, fmt.Errorf("filesystem: read manifest: %w", err)
	}

	var m manifest
	if err := json.Unmarshal(manifestData, &m); err != nil {
		return nil, fmt.Errorf("filesystem: unmarshal manifest: %w", err)
	}

	// Verify parts: contiguous 1..N, ETags match, non-final parts >= partSize.
	sorted := make([]storage.CompletedPart, len(opts.Parts))
	copy(sorted, opts.Parts)
	slices.SortFunc(sorted, func(a, b storage.CompletedPart) int { return a.PartNumber - b.PartNumber })

	for i, cp := range sorted {
		if cp.PartNumber != i+1 {
			return nil, storage.ErrPartNumberOutOfRange
		}

		partFilePath := filepath.Join(dir, strconv.Itoa(cp.PartNumber)+".part")

		partStat, statErr := os.Stat(partFilePath)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				return nil, storage.ErrPartNumberOutOfRange
			}

			return nil, statErr
		}

		// Non-final parts must be at least partSize. The final part is
		// exempt per the multipart contract.
		if i < len(sorted)-1 && partStat.Size() < partSize {
			return nil, storage.ErrPartTooSmall
		}

		etagData, readErr := os.ReadFile(filepath.Join(dir, strconv.Itoa(cp.PartNumber)+".etag"))
		if readErr != nil {
			if os.IsNotExist(readErr) {
				return nil, storage.ErrPartNumberOutOfRange
			}

			return nil, readErr
		}

		if string(etagData) != cp.ETag {
			return nil, storage.ErrPartETagMismatch
		}
	}

	// Assemble into a temp file then atomic rename.
	assemblyDir := filepath.Join(s.root, tmpDir)
	if err := os.MkdirAll(assemblyDir, 0o755); err != nil {
		return nil, fmt.Errorf("filesystem: create tmp dir: %w", err)
	}

	tmpPath := filepath.Join(assemblyDir, id.GenerateUUID())

	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("filesystem: create tmp file: %w", err)
	}

	hasher := md5.New()
	writer := io.MultiWriter(tmpFile, hasher)

	var totalSize int64

	for _, cp := range sorted {
		partFile, openErr := os.Open(filepath.Join(dir, strconv.Itoa(cp.PartNumber)+".part"))
		if openErr != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)

			return nil, fmt.Errorf("filesystem: open part %d: %w", cp.PartNumber, openErr)
		}

		n, copyErr := io.Copy(writer, partFile)
		_ = partFile.Close()

		if copyErr != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)

			return nil, fmt.Errorf("filesystem: copy part %d: %w", cp.PartNumber, copyErr)
		}

		totalSize += n
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("filesystem: sync tmp: %w", err)
	}

	_ = tmpFile.Close()

	// Atomic rename to final path.
	finalPath := s.resolvePath(m.Key)
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("filesystem: mkdir final: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("filesystem: rename to final: %w", err)
	}

	// Cleanup session directory.
	_ = os.RemoveAll(dir)

	stat, err := os.Stat(finalPath)
	if err != nil {
		return nil, fmt.Errorf("filesystem: stat final: %w", err)
	}

	etag := hex.EncodeToString(hasher.Sum(nil))
	if err := s.writeETag(m.Key, etag); err != nil {
		return nil, err
	}

	return &storage.ObjectInfo{
		Bucket:       bucketName,
		Key:          m.Key,
		ETag:         etag,
		Size:         totalSize,
		ContentType:  m.ContentType,
		LastModified: stat.ModTime(),
	}, nil
}

func (s *Service) AbortMultipart(_ context.Context, opts storage.AbortMultipartOptions) error {
	dir := s.sessionDir(opts.UploadID)
	// Idempotent: non-existent directory is fine.
	_ = os.RemoveAll(dir)

	return nil
}

// ── helpers ─────────────────────────────────────────────────────────────

func (s *Service) cleanupEmptyDirs(dir string) {
	for dir != s.root && strings.HasPrefix(dir, s.root) {
		// Never remove the hidden infrastructure directories themselves.
		base := filepath.Base(dir)
		if base == multipartDir || base == tmpDir || base == etagsDir {
			break
		}

		if err := os.Remove(dir); err != nil {
			break
		}

		dir = filepath.Dir(dir)
	}
}

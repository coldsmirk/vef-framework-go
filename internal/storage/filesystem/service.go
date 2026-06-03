package filesystem

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path"
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

	// bucketName is a sentinel used in ObjectInfo.Bucket for this backend.
	// The filesystem backend is bucket-less; this constant signals that
	// divergence from minio's real-bucket semantics intentionally.
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
	// tree and stores one small JSON sidecar per object carrying its MD5
	// ETag and ContentType. Persisting these at write time avoids
	// re-reading the entire object on every StatObject call (which the
	// file proxy invokes per request). Skipped by ListObjects.
	etagsDir = ".etags"
)

var (
	errInvalidObjectKey = errors.New("filesystem: invalid object key")
	errInvalidUploadID  = errors.New("filesystem: invalid upload id")
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

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve storage root directory: %w", err)
	}

	root = filepath.Clean(absRoot)

	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create storage root directory: %w", err)
	}

	return &Service{root: root}, nil
}

func (*Service) cleanObjectKey(key string) (string, error) {
	if key == "" || filepath.IsAbs(key) || strings.ContainsAny(key, "\x00\\") {
		return "", errInvalidObjectKey
	}

	if path.Clean(key) != key {
		return "", errInvalidObjectKey
	}

	for segment := range strings.SplitSeq(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", errInvalidObjectKey
		}
	}

	return filepath.FromSlash(key), nil
}

func (s *Service) resolvePath(key string) (string, error) {
	cleanKey, err := s.cleanObjectKey(key)
	if err != nil {
		return "", err
	}

	path := filepath.Join(s.root, cleanKey)

	rel, err := filepath.Rel(s.root, path)
	if err != nil {
		return "", err
	}

	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errInvalidObjectKey
	}

	return path, nil
}

func (s *Service) etagPath(key string) (string, error) {
	cleanKey, err := s.cleanObjectKey(key)
	if err != nil {
		return "", err
	}

	return filepath.Join(s.root, etagsDir, cleanKey), nil
}

func (s *Service) sessionDir(uploadID string) (string, error) {
	if uploadID == "" ||
		uploadID == "." ||
		uploadID == ".." ||
		path.Clean(uploadID) != uploadID ||
		strings.ContainsAny(uploadID, "\x00/\\") {
		return "", errInvalidUploadID
	}

	return filepath.Join(s.root, multipartDir, uploadID), nil
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

// objectMeta is the JSON structure stored in the .etags sidecar tree.
// It carries the MD5 ETag and the caller-supplied ContentType so that
// StatObject and CopyObject can return them without re-deriving from the
// file extension. The sidecar is written atomically (tmp+rename) and is
// a cache/hint, not a correctness invariant.
type objectMeta struct {
	ETag        string `json:"etag"`
	ContentType string `json:"contentType"`
}

// writeMeta persists etag and contentType to the sidecar tree for key.
// Errors are returned for logging by callers; the sidecar is advisory.
func (s *Service) writeMeta(key, etag, contentType string) error {
	p, err := s.etagPath(key)
	if err != nil {
		return err
	}

	data, err := json.Marshal(objectMeta{ETag: etag, ContentType: contentType})
	if err != nil {
		return fmt.Errorf("filesystem: marshal sidecar: %w", err)
	}

	return writeFileAtomic(p, data)
}

// readMeta returns the persisted ETag and ContentType for key.
// If no sidecar exists (legacy object or sidecar lost), it falls back to
// the plain-text ETag format written by older versions of the service.
// Missing sidecar is not an error: callers treat an empty ETag as
// "no validator" and derive ContentType from the file extension.
func (s *Service) readMeta(key string) (etag, contentType string, err error) {
	p, err := s.etagPath(key)
	if err != nil {
		return "", "", err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}

		return "", "", err
	}

	// Attempt JSON decode (current format). A decode failure means the
	// sidecar predates the JSON format: fall back to treating the raw bytes
	// as the legacy plain-text ETag (no ContentType recorded).
	var m objectMeta
	if json.Unmarshal(data, &m) == nil {
		return m.ETag, m.ContentType, nil
	}

	return string(data), "", nil
}

// removeMeta deletes the sidecar for key. Non-NotExist errors are logged
// because they indicate a real IO or permission problem. Idempotent.
func (s *Service) removeMeta(key string) {
	p, err := s.etagPath(key)
	if err != nil {
		return
	}

	if err := os.Remove(p); err != nil {
		if !os.IsNotExist(err) {
			slog.Error("filesystem: failed to remove object sidecar", "key", key, "error", err)
		}

		return
	}

	s.cleanupEmptyDirs(filepath.Dir(p))
}

func (s *Service) PutObject(_ context.Context, opts storage.PutObjectOptions) (*storage.ObjectInfo, error) {
	destPath, err := s.resolvePath(opts.Key)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write via a unique tmp file then atomic rename so a partial write is
	// never observable at the final path. The body is streamed to avoid
	// buffering arbitrarily large objects in memory, so we use a streaming
	// md5.New() here rather than hashx.MD5Bytes (which requires a []byte).
	tmpPath := destPath + ".tmp." + id.GenerateUUID()

	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create tmp file: %w", err)
	}

	hasher := md5.New()
	writer := io.MultiWriter(tmpFile, hasher)

	written, copyErr := io.Copy(writer, opts.Reader)
	if copyErr != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("failed to write file: %w", copyErr)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("failed to sync file: %w", err)
	}

	stat, statErr := tmpFile.Stat()

	_ = tmpFile.Close()

	if statErr != nil {
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("failed to stat file: %w", statErr)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("failed to finalize file: %w", err)
	}

	etag := hex.EncodeToString(hasher.Sum(nil))

	if err := s.writeMeta(opts.Key, etag, opts.ContentType); err != nil {
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
	path, err := s.resolvePath(opts.Key)
	if err != nil {
		return nil, err
	}

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
	path, err := s.resolvePath(opts.Key)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	s.cleanupEmptyDirs(filepath.Dir(path))
	s.removeMeta(opts.Key)

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

func (s *Service) CopyObject(_ context.Context, opts storage.CopyObjectOptions) (*storage.ObjectInfo, error) {
	srcPath, err := s.resolvePath(opts.SourceKey)
	if err != nil {
		return nil, err
	}

	destPath, err := s.resolvePath(opts.DestKey)
	if err != nil {
		return nil, err
	}

	// Inherit the ContentType from the source object's sidecar so it is
	// propagated to the destination without re-deriving from the extension.
	// Fall back to extension-derived type only when no stored ContentType
	// exists (legacy objects written before the sidecar recorded it).
	_, srcContentType, err := s.readMeta(opts.SourceKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read source sidecar: %w", err)
	}

	if srcContentType == "" {
		srcContentType = mime.TypeByExtension(filepath.Ext(srcPath))
	}

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

	// Write via a unique tmp file then atomic rename so a partial copy is
	// never observable at the final path. Body is streamed (source may be
	// large) so we keep streaming md5.New() rather than hashx.MD5Bytes.
	tmpPath := destPath + ".tmp." + id.GenerateUUID()

	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination tmp file: %w", err)
	}

	hasher := md5.New()
	writer := io.MultiWriter(tmpFile, hasher)

	written, copyErr := io.Copy(writer, src)
	if copyErr != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("failed to copy file: %w", copyErr)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("failed to sync destination file: %w", err)
	}

	stat, statErr := tmpFile.Stat()

	_ = tmpFile.Close()

	if statErr != nil {
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("failed to stat destination file: %w", statErr)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)

		return nil, fmt.Errorf("failed to finalize destination file: %w", err)
	}

	etag := hex.EncodeToString(hasher.Sum(nil))

	if err := s.writeMeta(opts.DestKey, etag, srcContentType); err != nil {
		return nil, err
	}

	return &storage.ObjectInfo{
		Bucket:       bucketName,
		Key:          opts.DestKey,
		ETag:         etag,
		Size:         written,
		ContentType:  srcContentType,
		LastModified: stat.ModTime(),
	}, nil
}

func (s *Service) StatObject(_ context.Context, opts storage.StatObjectOptions) (*storage.ObjectInfo, error) {
	path, err := s.resolvePath(opts.Key)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrObjectNotFound
		}

		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Read ETag and ContentType from the sidecar — written at PutObject /
	// CompleteMultipart / CopyObject time — to avoid re-hashing the entire
	// file on every Stat. Missing sidecar yields an empty ETag and empty
	// ContentType; the proxy treats that as "no validator". Fall back to
	// extension-derived ContentType for legacy objects without a sidecar.
	etag, contentType, err := s.readMeta(opts.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to read object sidecar: %w", err)
	}

	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(path))
	}

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
	if _, err := s.cleanObjectKey(opts.Key); err != nil {
		return nil, err
	}

	uploadID := id.GenerateUUID()

	dir, err := s.sessionDir(uploadID)
	if err != nil {
		return nil, err
	}

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
	dir, err := s.sessionDir(opts.UploadID)
	if err != nil {
		return nil, err
	}

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

	dir, err := s.sessionDir(opts.UploadID)
	if err != nil {
		return nil, err
	}

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

		// Minimum-size enforcement is deferred to here (Complete) rather
		// than PutPart because at upload time we cannot know whether a
		// given part is the last one. S3 semantics exempt the final part
		// from the minimum-size requirement; only non-final parts must
		// satisfy it. Checking at PutPart would incorrectly reject valid
		// single-part or tail uploads.
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
	finalPath, err := s.resolvePath(m.Key)
	if err != nil {
		_ = os.Remove(tmpPath)

		return nil, err
	}

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
	if err := s.writeMeta(m.Key, etag, m.ContentType); err != nil {
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
	dir, err := s.sessionDir(opts.UploadID)
	if err != nil {
		return err
	}

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

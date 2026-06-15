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
	"maps"
	"mime"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

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

	// etagsDir is the hidden directory under root that mirrors the object
	// tree and stores one small JSON sidecar per object carrying its MD5
	// ETag and ContentType. Persisting these at write time avoids
	// re-reading the entire object on every StatObject call (which the
	// file proxy invokes per request).
	etagsDir = ".etags"
)

var (
	errInvalidObjectKey = errors.New("filesystem: invalid object key")
	errInvalidUploadID  = errors.New("filesystem: invalid upload id")
)

type manifest struct {
	Key         string            `json:"key"`
	ContentType string            `json:"contentType"`
	Metadata    map[string]string `json:"metadata,omitempty"`
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

// openPartFile opens a part file during multipart assembly. It is a package
// var so white-box tests can observe how many part descriptors lazyPartReader
// holds open at once (the FD-bound regression guard).
var openPartFile = os.Open

// lazyPartReader opens its part file on first Read and closes it at EOF (or via
// close()), so multipart assembly holds at most one part descriptor open at a
// time regardless of part count — eagerly opening every part would exhaust the
// process file-descriptor budget on large multi-part uploads.
type lazyPartReader struct {
	path string
	num  int
	f    *os.File
	done bool
}

func (r *lazyPartReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}

	if r.f == nil {
		f, err := openPartFile(r.path)
		if err != nil {
			r.done = true

			return 0, fmt.Errorf("filesystem: open part %d: %w", r.num, err)
		}

		r.f = f
	}

	n, err := r.f.Read(p)
	if err != nil {
		// EOF or a real error: this part is exhausted. Close it and stop; an
		// io.MultiReader advances to the next reader on EOF and still uses any
		// final bytes returned alongside it.
		r.close()
		r.done = true
	}

	return n, err
}

// close releases the part descriptor if still open. Safe to call repeatedly and
// after EOF; closeParts uses it to mop up any reader left open by an early
// writeStreamAtomic error.
func (r *lazyPartReader) close() {
	if r.f != nil {
		_ = r.f.Close()
		r.f = nil
	}
}

// writeStreamAtomic streams r into destPath via a unique tmp file + atomic
// rename, computing the MD5 ETag as it copies. A partial write is never
// observable at destPath: the bytes are hashed and flushed into the tmp
// file, then a single Rename publishes the complete artifact (last-writer-
// wins on destPath). The tmp file is removed on every error path. The body
// is streamed rather than buffered so arbitrarily large objects never sit
// in memory. The caller is responsible for creating destPath's parent
// directory.
func writeStreamAtomic(destPath string, r io.Reader) (etag string, modTime time.Time, written int64, err error) {
	tmpPath := destPath + ".tmp." + id.GenerateUUID()

	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("filesystem: create tmp file: %w", err)
	}

	hasher := md5.New()

	written, err = io.Copy(io.MultiWriter(tmpFile, hasher), r)
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)

		return "", time.Time{}, 0, fmt.Errorf("filesystem: write file: %w", err)
	}

	if err = tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)

		return "", time.Time{}, 0, fmt.Errorf("filesystem: sync file: %w", err)
	}

	stat, statErr := tmpFile.Stat()

	_ = tmpFile.Close()

	if statErr != nil {
		_ = os.Remove(tmpPath)

		return "", time.Time{}, 0, fmt.Errorf("filesystem: stat file: %w", statErr)
	}

	if err = os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)

		return "", time.Time{}, 0, fmt.Errorf("filesystem: finalize file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), stat.ModTime(), written, nil
}

// objectMeta is the JSON structure stored in the .etags sidecar tree.
// It carries the MD5 ETag, the caller-supplied ContentType, and any
// caller-supplied custom Metadata so that StatObject and CopyObject can
// return them without re-deriving from the file. The sidecar is written
// atomically (tmp+rename) and is a cache/hint, not a correctness
// invariant.
type objectMeta struct {
	ETag        string            `json:"etag"`
	ContentType string            `json:"contentType"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// cloneMetadata returns a copy of m, or nil when m is empty. Cloning at
// the write/return boundary prevents aliasing the caller's map and keeps
// the empty-vs-nil distinction sane (an empty input yields nil, never a
// shared zero-length map).
func cloneMetadata(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}

	return maps.Clone(m)
}

// writeMeta persists etag, contentType, and metadata to the sidecar tree
// for key. Metadata keys are canonicalized here (the persistence boundary) so
// the sidecar always holds S3/HTTP-canonical keys and readMeta returns them in
// the same provider-neutral form every backend uses. Errors are returned for
// logging by callers; the sidecar is advisory.
func (s *Service) writeMeta(key, etag, contentType string, metadata map[string]string) error {
	p, err := s.etagPath(key)
	if err != nil {
		return err
	}

	data, err := json.Marshal(objectMeta{ETag: etag, ContentType: contentType, Metadata: storage.CanonicalizeMetadataKeys(metadata)})
	if err != nil {
		return fmt.Errorf("filesystem: marshal sidecar: %w", err)
	}

	return writeFileAtomic(p, data)
}

// readMeta returns the persisted ETag, ContentType, and Metadata for key.
// If no sidecar exists (legacy object or sidecar lost), it falls back to
// the plain-text ETag format written by older versions of the service.
// Missing sidecar is not an error: callers treat an empty ETag as
// "no validator" and derive ContentType from the file extension.
func (s *Service) readMeta(key string) (etag, contentType string, metadata map[string]string, err error) {
	p, err := s.etagPath(key)
	if err != nil {
		return "", "", nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil, nil
		}

		return "", "", nil, err
	}

	// Attempt JSON decode (current format). A decode failure means the
	// sidecar predates the JSON format: fall back to treating the raw bytes
	// as the legacy plain-text ETag (no ContentType / Metadata recorded).
	var m objectMeta
	if json.Unmarshal(data, &m) == nil {
		return m.ETag, m.ContentType, cloneMetadata(m.Metadata), nil
	}

	return string(data), "", nil, nil
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

	etag, modTime, written, err := writeStreamAtomic(destPath, opts.Reader)
	if err != nil {
		return nil, err
	}

	// The object is committed at this point; the sidecar is advisory (a
	// cache/hint for StatObject, not a correctness invariant). A sidecar
	// write failure must NOT fail the operation — log it and return the
	// successful info with the freshly computed ETag in hand, mirroring
	// removeMeta. A fatal error here would tell the caller the upload
	// failed while the object is in fact present and readable.
	if err := s.writeMeta(opts.Key, etag, opts.ContentType, opts.Metadata); err != nil {
		slog.Error("filesystem: failed to write object sidecar", "key", opts.Key, "error", err)
	}

	return &storage.ObjectInfo{
		Bucket:       bucketName,
		Key:          opts.Key,
		ETag:         etag,
		Size:         written,
		ContentType:  opts.ContentType,
		LastModified: modTime,
		Metadata:     storage.CanonicalizeMetadataKeys(opts.Metadata),
	}, nil
}

func (s *Service) GetObject(_ context.Context, opts storage.GetObjectOptions) (io.ReadCloser, *storage.ObjectInfo, error) {
	path, err := s.resolvePath(opts.Key)
	if err != nil {
		return nil, nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, storage.ErrObjectNotFound
		}

		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Best-effort metadata: the reader is the contract's primary result, so
	// a stat/sidecar hiccup must not fail the read. The proxy tolerates a
	// nil info (streams without Content-Length / ETag), so on any error we
	// hand back the reader with a nil info rather than closing it.
	info, infoErr := s.statOpenFile(opts.Key, path, file)
	if infoErr != nil {
		return file, nil, nil //nolint:nilerr // best-effort: object body is open, metadata is advisory
	}

	return file, info, nil
}

// statOpenFile builds an ObjectInfo for an already-opened object file. It
// stats the open handle (consistent with the bytes the caller will read)
// and reads the advisory sidecar for the ETag, ContentType, and Metadata.
func (s *Service) statOpenFile(key, path string, file *os.File) (*storage.ObjectInfo, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	etag, contentType, metadata, err := s.readMeta(key)
	if err != nil {
		return nil, err
	}

	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(path))
	}

	return &storage.ObjectInfo{
		Bucket:       bucketName,
		Key:          key,
		ETag:         etag,
		Size:         stat.Size(),
		ContentType:  contentType,
		LastModified: stat.ModTime(),
		Metadata:     metadata,
	}, nil
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

	// Inherit the ContentType and custom Metadata from the source object's
	// sidecar so they propagate to the destination (consistent with the
	// minio and memory backends). Fall back to an extension-derived type
	// only when no stored ContentType exists (legacy objects written before
	// the sidecar recorded it).
	_, srcContentType, srcMetadata, err := s.readMeta(opts.SourceKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read source sidecar: %w", err)
	}

	// Canonicalize once so both the destination sidecar and the returned info
	// carry canonical keys even when the source sidecar predates this rule.
	srcMetadata = storage.CanonicalizeMetadataKeys(srcMetadata)

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

	etag, modTime, written, err := writeStreamAtomic(destPath, src)
	if err != nil {
		return nil, err
	}

	// Advisory sidecar — see PutObject. A write failure here does not undo
	// the committed copy, so log and return the successful info.
	if err := s.writeMeta(opts.DestKey, etag, srcContentType, srcMetadata); err != nil {
		slog.Error("filesystem: failed to write object sidecar", "key", opts.DestKey, "error", err)
	}

	return &storage.ObjectInfo{
		Bucket:       bucketName,
		Key:          opts.DestKey,
		ETag:         etag,
		Size:         written,
		ContentType:  srcContentType,
		LastModified: modTime,
		Metadata:     srcMetadata,
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

	// Read ETag, ContentType, and Metadata from the sidecar — written at
	// PutObject / CompleteMultipart / CopyObject time — to avoid re-hashing
	// the entire file on every Stat. Missing sidecar yields an empty ETag,
	// empty ContentType, and nil Metadata; the proxy treats that as "no
	// validator". Fall back to extension-derived ContentType for legacy
	// objects without a sidecar.
	etag, contentType, metadata, err := s.readMeta(opts.Key)
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
		Metadata:     metadata,
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

	m := manifest{Key: opts.Key, ContentType: opts.ContentType, Metadata: cloneMetadata(opts.Metadata)}

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

	// Canonicalize the manifest metadata at the object store boundary so both
	// the sidecar and the returned info carry canonical keys.
	m.Metadata = storage.CanonicalizeMetadataKeys(m.Metadata)

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

	finalPath, err := s.resolvePath(m.Key)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return nil, fmt.Errorf("filesystem: mkdir final: %w", err)
	}

	// Assemble the parts through writeStreamAtomic, which streams the
	// concatenation into a tmp file and atomically renames it into finalPath.
	// Each part is wrapped in a lazyPartReader that opens its file on first Read
	// and closes it at EOF, so at most ONE part descriptor is open at any instant
	// during assembly. Eagerly opening every part up front would exhaust the
	// process file-descriptor budget on large multi-part uploads (a 1 GiB upload
	// at the 4 MiB part size is 256 parts; operators raising MaxUploadSize push
	// this far higher, past the default ulimit -n).
	partReaders := make([]io.Reader, 0, len(sorted))
	lazyParts := make([]*lazyPartReader, 0, len(sorted))

	for _, cp := range sorted {
		lp := &lazyPartReader{
			path: filepath.Join(dir, strconv.Itoa(cp.PartNumber)+".part"),
			num:  cp.PartNumber,
		}
		lazyParts = append(lazyParts, lp)
		partReaders = append(partReaders, lp)
	}

	closeParts := func() {
		for _, lp := range lazyParts {
			lp.close()
		}
	}

	etag, modTime, totalSize, err := writeStreamAtomic(finalPath, io.MultiReader(partReaders...))

	closeParts()

	if err != nil {
		return nil, err
	}

	// Cleanup session directory.
	_ = os.RemoveAll(dir)

	// Advisory sidecar — see PutObject. A write failure here does not undo
	// the committed object, so log and return the successful info.
	if err := s.writeMeta(m.Key, etag, m.ContentType, m.Metadata); err != nil {
		slog.Error("filesystem: failed to write object sidecar", "key", m.Key, "error", err)
	}

	return &storage.ObjectInfo{
		Bucket:       bucketName,
		Key:          m.Key,
		ETag:         etag,
		Size:         totalSize,
		ContentType:  m.ContentType,
		LastModified: modTime,
		Metadata:     m.Metadata,
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
		if base == multipartDir || base == etagsDir {
			break
		}

		if err := os.Remove(dir); err != nil {
			break
		}

		dir = filepath.Dir(dir)
	}
}

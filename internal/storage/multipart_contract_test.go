package storage_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/storage/filesystem"
	"github.com/coldsmirk/vef-framework-go/internal/storage/memory"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// MultipartContractSuite is a backend-agnostic test suite that verifies
// the storage.Multipart interface contract. Run it against every backend
// that implements Multipart to ensure behavioral consistency.
type MultipartContractSuite struct {
	suite.Suite

	service   storage.Service
	multipart storage.Multipart
	partSize  int64
}

func (s *MultipartContractSuite) init(key string) *storage.MultipartSession {
	session, err := s.multipart.InitMultipart(context.Background(), storage.InitMultipartOptions{
		Key:         key,
		ContentType: "application/octet-stream",
	})
	s.Require().NoError(err, "InitMultipart should succeed")

	return session
}

func (s *MultipartContractSuite) put(session *storage.MultipartSession, partNum int, data []byte) *storage.PartInfo {
	info, err := s.multipart.PutPart(context.Background(), storage.PutPartOptions{
		Key:        session.Key,
		UploadID:   session.UploadID,
		PartNumber: partNum,
		Reader:     bytes.NewReader(data),
		Size:       int64(len(data)),
	})
	s.Require().NoError(err, "PutPart(%d) should succeed", partNum)

	return info
}

func (s *MultipartContractSuite) TestHappyPath() {
	session := s.init("contract/happy.bin")
	part1Data := bytes.Repeat([]byte{'a'}, int(s.partSize))
	part2Data := bytes.Repeat([]byte{'b'}, int(s.partSize)/2)

	p1 := s.put(session, 1, part1Data)
	p2 := s.put(session, 2, part2Data)

	info, err := s.multipart.CompleteMultipart(context.Background(), storage.CompleteMultipartOptions{
		Key: session.Key, UploadID: session.UploadID,
		Parts: []storage.CompletedPart{{PartNumber: 1, ETag: p1.ETag}, {PartNumber: 2, ETag: p2.ETag}},
	})
	s.Require().NoError(err, "CompleteMultipart should succeed")
	s.Equal(int64(len(part1Data)+len(part2Data)), info.Size, "Assembled size should match")

	reader, _, err := s.service.GetObject(context.Background(), storage.GetObjectOptions{Key: session.Key})
	s.Require().NoError(err, "GetObject should succeed after Complete")

	defer reader.Close()

	got, _ := io.ReadAll(reader)
	s.Equal(append(part1Data, part2Data...), got, "Assembled content should be part1+part2")
}

func (s *MultipartContractSuite) TestPutPartOverwrite() {
	session := s.init("contract/overwrite.bin")
	first := bytes.Repeat([]byte{'x'}, int(s.partSize))
	second := bytes.Repeat([]byte{'y'}, int(s.partSize))

	s.put(session, 1, first)
	p1 := s.put(session, 1, second) // overwrite

	_, err := s.multipart.CompleteMultipart(context.Background(), storage.CompleteMultipartOptions{
		Key: session.Key, UploadID: session.UploadID,
		Parts: []storage.CompletedPart{{PartNumber: 1, ETag: p1.ETag}},
	})
	s.Require().NoError(err, "Complete with latest ETag should succeed")

	reader, _, _ := s.service.GetObject(context.Background(), storage.GetObjectOptions{Key: session.Key})
	defer reader.Close()

	got, _ := io.ReadAll(reader)
	s.Equal(second, got, "Content should be the second (overwritten) upload")
}

func (s *MultipartContractSuite) TestETagMismatch() {
	session := s.init("contract/etag-mismatch.bin")
	s.put(session, 1, bytes.Repeat([]byte{'a'}, int(s.partSize)))

	_, err := s.multipart.CompleteMultipart(context.Background(), storage.CompleteMultipartOptions{
		Key: session.Key, UploadID: session.UploadID,
		Parts: []storage.CompletedPart{{PartNumber: 1, ETag: "wrong-etag"}},
	})
	s.ErrorIs(err, storage.ErrPartETagMismatch, "Complete with wrong ETag must return ErrPartETagMismatch")
}

func (s *MultipartContractSuite) TestPartNumberGap() {
	session := s.init("contract/gap.bin")
	p1 := s.put(session, 1, bytes.Repeat([]byte{'a'}, int(s.partSize)))
	p3 := s.put(session, 3, bytes.Repeat([]byte{'c'}, int(s.partSize)))

	_, err := s.multipart.CompleteMultipart(context.Background(), storage.CompleteMultipartOptions{
		Key: session.Key, UploadID: session.UploadID,
		Parts: []storage.CompletedPart{{PartNumber: 1, ETag: p1.ETag}, {PartNumber: 3, ETag: p3.ETag}},
	})
	s.ErrorIs(err, storage.ErrPartNumberOutOfRange, "Complete with gap (1,3) must return ErrPartNumberOutOfRange")
}

func (s *MultipartContractSuite) TestEmptyParts() {
	session := s.init("contract/empty.bin")

	_, err := s.multipart.CompleteMultipart(context.Background(), storage.CompleteMultipartOptions{
		Key: session.Key, UploadID: session.UploadID,
		Parts: []storage.CompletedPart{},
	})
	s.ErrorIs(err, storage.ErrPartNumberOutOfRange, "Complete with empty parts must return ErrPartNumberOutOfRange")
}

func (s *MultipartContractSuite) TestCompleteAfterComplete() {
	session := s.init("contract/double-complete.bin")
	p1 := s.put(session, 1, bytes.Repeat([]byte{'a'}, int(s.partSize)))

	_, err := s.multipart.CompleteMultipart(context.Background(), storage.CompleteMultipartOptions{
		Key: session.Key, UploadID: session.UploadID,
		Parts: []storage.CompletedPart{{PartNumber: 1, ETag: p1.ETag}},
	})
	s.Require().NoError(err, "First Complete should succeed")

	_, err = s.multipart.CompleteMultipart(context.Background(), storage.CompleteMultipartOptions{
		Key: session.Key, UploadID: session.UploadID,
		Parts: []storage.CompletedPart{{PartNumber: 1, ETag: p1.ETag}},
	})
	s.ErrorIs(err, storage.ErrUploadSessionNotFound, "Second Complete must return ErrUploadSessionNotFound")
}

func (s *MultipartContractSuite) TestPutPartAfterComplete() {
	session := s.init("contract/put-after-complete.bin")
	p1 := s.put(session, 1, bytes.Repeat([]byte{'a'}, int(s.partSize)))

	_, err := s.multipart.CompleteMultipart(context.Background(), storage.CompleteMultipartOptions{
		Key: session.Key, UploadID: session.UploadID,
		Parts: []storage.CompletedPart{{PartNumber: 1, ETag: p1.ETag}},
	})
	s.Require().NoError(err, "CompleteMultipart should succeed before testing PutPart after completion")

	_, err = s.multipart.PutPart(context.Background(), storage.PutPartOptions{
		Key: session.Key, UploadID: session.UploadID,
		PartNumber: 2, Reader: bytes.NewReader([]byte("x")), Size: 1,
	})
	s.ErrorIs(err, storage.ErrUploadSessionNotFound, "PutPart after Complete must return ErrUploadSessionNotFound")
}

func (s *MultipartContractSuite) TestAbortIdempotent() {
	err := s.multipart.AbortMultipart(context.Background(), storage.AbortMultipartOptions{
		Key: "contract/nonexistent.bin", UploadID: "does-not-exist",
	})
	s.NoError(err, "Abort on unknown session must return nil (idempotent)")
}

func (s *MultipartContractSuite) TestAbortThenPutPart() {
	session := s.init("contract/abort-then-put.bin")

	err := s.multipart.AbortMultipart(context.Background(), storage.AbortMultipartOptions{
		Key: session.Key, UploadID: session.UploadID,
	})
	s.Require().NoError(err, "Abort should succeed")

	_, err = s.multipart.PutPart(context.Background(), storage.PutPartOptions{
		Key: session.Key, UploadID: session.UploadID,
		PartNumber: 1, Reader: bytes.NewReader([]byte("x")), Size: 1,
	})
	s.ErrorIs(err, storage.ErrUploadSessionNotFound, "PutPart after Abort must return ErrUploadSessionNotFound")
}

func (s *MultipartContractSuite) TestNonFinalPartTooSmall() {
	session := s.init("contract/too-small.bin")
	small := bytes.Repeat([]byte{'s'}, int(s.partSize)/2) // < PartSize
	normal := bytes.Repeat([]byte{'n'}, int(s.partSize))

	p1 := s.put(session, 1, small)
	p2 := s.put(session, 2, normal)

	_, err := s.multipart.CompleteMultipart(context.Background(), storage.CompleteMultipartOptions{
		Key: session.Key, UploadID: session.UploadID,
		Parts: []storage.CompletedPart{{PartNumber: 1, ETag: p1.ETag}, {PartNumber: 2, ETag: p2.ETag}},
	})
	s.ErrorIs(err, storage.ErrPartTooSmall, "Non-final part smaller than PartSize must return ErrPartTooSmall")
}

// TestMetadataRoundTrip pins the provider-neutral contract that custom object
// Metadata supplied to PutObject is echoed back by StatObject under
// S3/HTTP-canonical keys, identically across every backend. The MinIO/S3
// protocol forces this canonicalization; memory and filesystem adopt the same
// rule at their store boundary (via storage.CanonicalizeMetadataKeys), so this
// suite — and the MinIO suite's own TestMetadataRoundTrip — assert the SAME
// guarantee against the identical helper, turning the MinIO finding into an
// enforced cross-backend invariant.
//
// The input is deliberately mixed-case so canonicalization is actually
// exercised: a lowercase key ("author") and an unconventional-case key
// ("mixed-KEY") must come back canonicalized, while an already-canonical key
// ("X-Custom") survives unchanged.
func (s *MultipartContractSuite) TestMetadataRoundTrip() {
	meta := map[string]string{"author": "a", "X-Custom": "b", "mixed-KEY": "c"}
	expected := storage.CanonicalizeMetadataKeys(meta)
	payload := []byte("metadata-payload")

	_, err := s.service.PutObject(context.Background(), storage.PutObjectOptions{
		Key:         "contract/metadata.bin",
		Reader:      bytes.NewReader(payload),
		Size:        int64(len(payload)),
		ContentType: "application/octet-stream",
		Metadata:    meta,
	})
	s.Require().NoError(err, "PutObject with metadata should succeed")

	info, err := s.service.StatObject(context.Background(), storage.StatObjectOptions{Key: "contract/metadata.bin"})
	s.Require().NoError(err, "StatObject should succeed")

	for k, v := range expected {
		s.Equal(v, info.Metadata[k],
			"StatObject must round-trip custom metadata under the canonical key %q across backends", k)
	}

	// The raw lowercase key must NOT survive — every backend canonicalizes.
	s.NotContains(info.Metadata, "author",
		"the verbatim lowercase 'author' must not survive; backends canonicalize to 'Author'")
}

// ── Entry points ────────────────────────────────────────────────────────

func TestMultipartContractMemory(t *testing.T) {
	svc := memory.New()
	mp := svc.(storage.Multipart)
	suite.Run(t, &MultipartContractSuite{
		service: svc, multipart: mp,
		partSize: mp.PartSize(),
	})
}

func TestMultipartContractFilesystem(t *testing.T) {
	svc, err := filesystem.New(config.FilesystemConfig{Root: t.TempDir()})
	require.NoError(t, err, "Filesystem backend creation should succeed")

	mp := svc.(storage.Multipart)
	suite.Run(t, &MultipartContractSuite{
		service: svc, multipart: mp,
		partSize: mp.PartSize(),
	})
}

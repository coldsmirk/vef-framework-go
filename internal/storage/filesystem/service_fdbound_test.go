package filesystem

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// TestCompleteMultipartHoldsAtMostOnePartOpen is the FD-bound regression guard
// for the lazy part-assembly rewrite. CompleteMultipart concatenates every part
// through a lazyPartReader that opens its file on first Read and closes it at
// EOF, so assembly must hold at most ONE part descriptor open at any instant —
// regardless of part count. An eager "open every part up front" implementation
// would exhaust the process file-descriptor budget on large uploads.
//
// The test installs a counting wrapper over the openPartFile seam. Because
// CompleteMultipart assembles parts single-threaded (io.MultiReader reads
// sequentially), the wrapper can synchronously count how many already-opened
// part files are still open at the moment a new one is opened: a part file is
// "still open" iff Stat() on its handle does not error. With lazy assembly that
// count is always 0 (the previous part is closed before the next opens); an
// eager implementation would observe N-1 still-open files and drive
// maxConcurrent up to the part count.
func TestCompleteMultipartHoldsAtMostOnePartOpen(t *testing.T) {
	svc, err := New(config.FilesystemConfig{Root: t.TempDir()})
	require.NoError(t, err, "filesystem backend creation should succeed")

	mp := svc.(storage.Multipart)
	ctx := context.Background()

	const partCount = 64

	session, err := mp.InitMultipart(ctx, storage.InitMultipartOptions{
		Key:         "fdbound/object.bin",
		ContentType: "application/octet-stream",
	})
	require.NoError(t, err, "InitMultipart should succeed")

	partSizeBytes := mp.PartSize()
	parts := make([]storage.CompletedPart, 0, partCount)

	for n := 1; n <= partCount; n++ {
		// Non-final parts must satisfy the minimum-size rule; the final part is
		// exempt, so make it deliberately smaller to keep the upload cheap.
		size := int(partSizeBytes)
		if n == partCount {
			size = 16
		}

		data := bytes.Repeat([]byte{byte('a' + n%26)}, size)

		info, putErr := mp.PutPart(ctx, storage.PutPartOptions{
			Key:        session.Key,
			UploadID:   session.UploadID,
			PartNumber: n,
			Reader:     bytes.NewReader(data),
			Size:       int64(size),
		})
		require.NoError(t, putErr, "PutPart(%d) should succeed", n)

		parts = append(parts, storage.CompletedPart{PartNumber: n, ETag: info.ETag})
	}

	// Install the counting seam and restore it afterwards.
	original := openPartFile

	defer func() { openPartFile = original }()

	var (
		opened        []*os.File
		maxConcurrent int
	)

	openPartFile = func(path string) (*os.File, error) {
		// Count part files opened earlier that are still open. A closed *os.File
		// returns an error from Stat (os.ErrClosed); an open one does not.
		concurrent := 0

		for _, f := range opened {
			if _, statErr := f.Stat(); statErr == nil {
				concurrent++
			}
		}

		// concurrent counts the files open *before* this open; the new handle
		// pushes the live total to concurrent+1.
		if concurrent+1 > maxConcurrent {
			maxConcurrent = concurrent + 1
		}

		f, openErr := original(path)
		if openErr != nil {
			return nil, openErr
		}

		opened = append(opened, f)

		return f, nil
	}

	info, err := mp.CompleteMultipart(ctx, storage.CompleteMultipartOptions{
		Key:      session.Key,
		UploadID: session.UploadID,
		Parts:    parts,
	})
	require.NoError(t, err, "CompleteMultipart should succeed")
	require.NotNil(t, info, "CompleteMultipart should return ObjectInfo")

	require.Len(t, opened, partCount,
		"every part file should be opened exactly once during assembly")
	require.LessOrEqual(t, maxConcurrent, 1,
		"assembly must hold at most one part descriptor open at a time; "+
			"maxConcurrent=%d indicates parts are opened eagerly instead of lazily", maxConcurrent)
}

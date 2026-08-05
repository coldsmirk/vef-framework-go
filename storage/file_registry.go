package storage

import "context"

// FileRegistry is the read side of the durable upload registry: it
// answers what the framework knows about an object key beyond the key
// itself — original filename, size, MIME type, uploader, upload time,
// lifecycle state.
//
// Business code needs it because a business model stores only the
// storage key. Rendering "报告.pdf (2.4 MB), uploaded by 张三" from a
// bare priv/2026/08/04/<uuid>.pdf therefore requires this lookup.
//
// The interface is read-only by design. Recording an upload is the
// framework's own completeness invariant — it happens inside the
// transaction that finalizes the upload — and exposing a write method
// would invite records the framework cannot keep consistent with the
// backend.
//
// Implementations MUST be safe for concurrent use.
type FileRegistry interface {
	// Lookup returns the records for the supplied object keys, keyed by
	// object key. Keys with no record are omitted rather than reported
	// as an error: a caller rendering filenames must degrade to the key,
	// never fail the request. Duplicate keys are deduplicated; an empty
	// or nil keys argument returns an empty map.
	//
	// Records of deleted objects are returned too (FileRecord.IsDeleted
	// distinguishes them) — a business row that still references a
	// deleted key wants the filename, not silence.
	Lookup(ctx context.Context, keys []string) (map[string]FileRecord, error)
}

package storage

import "net/textproto"

// CanonicalizeMetadataKeys returns a new map whose keys are each passed through
// textproto.CanonicalMIMEHeaderKey, the S3/HTTP-header canonical form (e.g.
// "author" -> "Author", "x-custom" -> "X-Custom"). Values are copied verbatim.
//
// This is the single source of truth for the object-metadata key contract:
// every backend normalizes keys through this helper at the store boundary so
// that metadata round-trips in identical canonical form regardless of provider.
// The MinIO/S3 protocol forces this canonicalization (user metadata travels as
// "X-Amz-Meta-<key>" headers), so the memory and filesystem backends adopt the
// same rule rather than diverging with verbatim keys.
//
// A nil or empty input yields a nil result. If two raw keys canonicalize to the
// same key, last-writer-wins by Go map iteration order; supplying keys that
// collide under canonicalization is a caller error.
func CanonicalizeMetadataKeys(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}

	out := make(map[string]string, len(m))
	for k, v := range m {
		out[textproto.CanonicalMIMEHeaderKey(k)] = v
	}

	return out
}

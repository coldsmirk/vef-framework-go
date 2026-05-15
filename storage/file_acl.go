package storage

import (
	"context"
	"strings"

	"github.com/coldsmirk/vef-framework-go/security"
)

// Object key namespace prefixes. The framework uses these to communicate
// the intended visibility of a key to the FileACL layer; the storage
// resource emits keys under PublicPrefix when the upload is flagged
// public, and under PrivatePrefix otherwise.
//
// These are conventions, not enforcement: any FileACL implementation is
// free to ignore the prefix and decide visibility purely from business
// state (e.g. a per-key visibility column on the owning row).
const (
	// PublicPrefix is the key namespace for objects intended to be
	// world-readable (or readable by any authenticated principal,
	// depending on the ACL implementation). DefaultFileACL grants read
	// access to keys under this prefix.
	PublicPrefix = "pub/"

	// PrivatePrefix is the key namespace for objects whose visibility is
	// controlled by business state. DefaultFileACL denies read access
	// to keys under this prefix; business modules MUST register a
	// FileACL implementation that consults their own ownership / ACL
	// tables to grant access.
	PrivatePrefix = "priv/"
)

// FileACL decides whether a principal may read an object key.
//
// The storage module is provider-neutral and intentionally has no model
// of "ownership" — that information lives entirely in the business
// layer (which model owns which key, what visibility rules apply, what
// roles have read access, etc.). Business modules implement FileACL to
// bridge that gap and inject the implementation into the framework via
// vef.SupplyFileACL.
//
// Typical implementation pattern:
//
//  1. Maintain a reverse index from object key to the owning row,
//     populated by Files.OnCreate / Files.OnUpdate.
//  2. In CanRead, look up the row by key and decide based on the
//     principal's identity, roles, or tenant against the row's
//     visibility / owner columns.
//
// Implementations MUST return false (not error) for unauthorized
// access; errors are reserved for backend / lookup failures (database
// unavailable, etc.) and surface to the caller as 500-class responses.
type FileACL interface {
	// CanRead returns true when principal is authorized to read key.
	// Called by the proxy middleware (GET /storage/files/<key>) and the
	// stat RPC. Pub/* keys typically short-circuit before reaching this
	// hook; implementations only see keys that need authoritative
	// authorization.
	CanRead(ctx context.Context, principal *security.Principal, key string) (bool, error)
}

// DefaultFileACL is the framework-provided default ACL. It grants read
// access only to keys under PublicPrefix.
//
// This default keeps the framework safe-by-default: without an explicit
// override, the storage module behaves as a pub-only file server and
// never exposes private keys to authenticated callers, regardless of
// who asks. Business modules that need any access beyond pub/ MUST
// register their own FileACL via vef.SupplyFileACL.
type DefaultFileACL struct{}

// CanRead allows reads of keys under PublicPrefix and denies everything
// else. Principal is ignored — the default ACL has no notion of
// per-principal access; that is the business module's responsibility.
func (*DefaultFileACL) CanRead(_ context.Context, _ *security.Principal, key string) (bool, error) {
	return strings.HasPrefix(key, PublicPrefix), nil
}

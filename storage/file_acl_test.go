package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
)

func TestDefaultFileACL(t *testing.T) {
	ctx := context.Background()
	principal := &security.Principal{ID: "user-1", Name: "alice"}
	acl := new(storage.DefaultFileACL)

	t.Run("CanReadAllowsPublicPrefix", func(t *testing.T) {
		allowed, err := acl.CanRead(ctx, principal, storage.PublicPrefix+"2026/05/11/cover.png")
		require.NoError(t, err, "Default ACL must not error on a well-formed key")
		assert.True(t, allowed, "Default ACL must allow reads under PublicPrefix")
	})

	t.Run("CanReadAllowsPublicPrefixForAnonymous", func(t *testing.T) {
		// nil principal should still get the same answer — pub/ is meant
		// to be readable by anyone, identity does not matter.
		allowed, err := acl.CanRead(ctx, nil, storage.PublicPrefix+"hero.jpg")
		require.NoError(t, err, "Default ACL must not error for anonymous principal")
		assert.True(t, allowed, "Default ACL must allow anonymous reads under PublicPrefix")
	})

	t.Run("CanReadDeniesPrivatePrefix", func(t *testing.T) {
		allowed, err := acl.CanRead(ctx, principal, storage.PrivatePrefix+"2026/05/11/secret.bin")
		require.NoError(t, err, "Default ACL must not error on a well-formed key")
		assert.False(t, allowed, "Default ACL must deny reads under PrivatePrefix without business override")
	})

	t.Run("CanReadDeniesUnknownPrefix", func(t *testing.T) {
		// Keys that don't start with pub/ or priv/ are out-of-convention
		// and must be denied — we treat them as private by default.
		allowed, err := acl.CanRead(ctx, principal, "uploads/raw/file.bin")
		require.NoError(t, err, "Default ACL must not error on a non-conventional key")
		assert.False(t, allowed, "Default ACL must deny reads of keys outside PublicPrefix")
	})

	t.Run("CanReadDeniesEmptyKey", func(t *testing.T) {
		// Empty key should not match PublicPrefix and must be denied.
		allowed, err := acl.CanRead(ctx, principal, "")
		require.NoError(t, err, "Default ACL must not error on an empty key")
		assert.False(t, allowed, "Default ACL must deny reads of empty keys")
	})

	t.Run("CanListDeniesEverything", func(t *testing.T) {
		// List is intentionally restrictive in the default ACL — even
		// PublicPrefix listing requires a business override.
		for _, prefix := range []string{
			"",
			storage.PublicPrefix,
			storage.PrivatePrefix,
			"any/random/prefix/",
		} {
			allowed, err := acl.CanList(ctx, principal, prefix)
			require.NoError(t, err, "Default ACL must not error on CanList(%q)", prefix)
			assert.False(t, allowed, "Default ACL must deny CanList for prefix %q", prefix)
		}
	})
}

// Compile-time assertion that DefaultFileACL satisfies the FileACL
// contract — guards against accidental signature drift.
var _ storage.FileACL = new(storage.DefaultFileACL)

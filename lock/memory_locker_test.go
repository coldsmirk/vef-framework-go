package lock

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoryLocker(t *testing.T) {
	locker := NewMemoryLocker()

	steal := func(t *testing.T, name string) {
		t.Helper()

		store, ok := locker.(*MemoryLocker)
		require.True(t, ok, "the memory locker should expose its concrete type for the steal hook")

		store.mu.Lock()
		delete(store.holders, name)
		store.mu.Unlock()
	}

	runLockerContract(t, locker, steal)
}

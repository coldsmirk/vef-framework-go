package exec

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/datasource"
)

// StubRegistry reports every name as unregistered, which is all the release
// path consults. Every other registry method is left to the embedded nil
// interface: reaching one means the test drifted from what it covers.
type StubRegistry struct {
	datasource.Registry
}

func (*StubRegistry) Has(string) bool {
	return false
}

func TestSystemDatabasesRelease(t *testing.T) {
	t.Run("ReclaimsLockEntry", func(t *testing.T) {
		databases := newSystemDatabases(new(StubRegistry), nil)

		require.NoError(t, databases.Release(t.Context(), "sys-a"), "Releasing an unregistered system should be a no-op")
		assert.Empty(t, databases.sources, "Release should reclaim the released system's lock entry")
	})

	t.Run("KeepsMutualExclusionUnderConcurrentReleases", func(t *testing.T) {
		databases := newSystemDatabases(new(StubRegistry), nil)

		const (
			holders = 8
			rounds  = 50
		)

		// guarded is deliberately unsynchronized: the system's lock is its only
		// protection, so two callers serializing on different lock entries for
		// one system surface as a lost update and a race-detector report.
		var (
			wg      sync.WaitGroup
			guarded int
		)

		for range holders {
			wg.Go(func() {
				for range rounds {
					source := databases.lockSource("sys-x")
					guarded++

					source.mu.Unlock()
				}
			})
		}

		for range 4 {
			wg.Go(func() {
				for range rounds {
					assert.NoError(t, databases.Release(t.Context(), "sys-x"), "Concurrent release should not fail")
				}
			})
		}

		wg.Wait()

		assert.Equal(t, holders*rounds, guarded, "Every guarded increment should be serialized by the system's lock")

		require.NoError(t, databases.Release(t.Context(), "sys-x"), "Final release should succeed")
		assert.Empty(t, databases.sources, "No lock entry should outlive the last release")
	})
}

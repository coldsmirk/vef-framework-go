package inbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pubinbox "github.com/coldsmirk/vef-framework-go/event/inbox"
	iinbox "github.com/coldsmirk/vef-framework-go/internal/event/inbox"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// fakeRepo is a minimal Repository stub for Cleaner tests. Only
// DeleteOlderThan is exercised; the other methods are not reached.
type fakeRepo struct {
	// deleted receives the cutoff passed to DeleteOlderThan.
	cutoffs []timex.DateTime
	// returns is the sequence of (count, error) responses to return.
	returns []deleteReturn
}

type deleteReturn struct {
	count int64
	err   error
}

func (f *fakeRepo) DeleteOlderThan(_ context.Context, cutoff timex.DateTime) (int64, error) {
	f.cutoffs = append(f.cutoffs, cutoff)

	idx := len(f.cutoffs) - 1
	if idx >= len(f.returns) {
		return 0, nil
	}

	r := f.returns[idx]

	return r.count, r.err
}

func (*fakeRepo) Acquire(_ context.Context, _, _ string, _ timex.DateTime) (pubinbox.AcquireResult, string, error) {
	panic("not expected")
}

func (*fakeRepo) MarkCompleted(_ context.Context, _, _, _ string) error {
	panic("not expected")
}

func (*fakeRepo) Release(_ context.Context, _, _, _ string) error {
	panic("not expected")
}

func TestCleanerCleanup(t *testing.T) {
	t.Run("deletes records older than retention window", func(t *testing.T) {
		repo := &fakeRepo{returns: []deleteReturn{{count: 3, err: nil}}}
		retention := time.Hour

		before := timex.Now()
		cleaner := iinbox.NewCleaner(repo, retention, nil)
		cleaner.Cleanup(context.Background())

		after := timex.Now()

		require.Len(t, repo.cutoffs, 1, "Cleanup should call DeleteOlderThan exactly once")

		cutoff := repo.cutoffs[0]
		require.True(
			t,
			cutoff.Unwrap().After(before.Unwrap().Add(-retention-time.Second)),
			"Cutoff should be approximately now minus retention",
		)
		require.True(
			t,
			cutoff.Unwrap().Before(after.Unwrap().Add(-retention+time.Second)),
			"Cutoff should not be newer than now minus retention",
		)
	})

	t.Run("returns without logging when zero records deleted", func(t *testing.T) {
		repo := &fakeRepo{returns: []deleteReturn{{count: 0, err: nil}}}
		cleaner := iinbox.NewCleaner(repo, time.Hour, nil)

		// Must not panic — nil logger is replaced by Discard inside NewCleaner.
		cleaner.Cleanup(context.Background())

		require.Len(t, repo.cutoffs, 1, "Cleanup should still call DeleteOlderThan when nothing is deleted")
	})

	t.Run("logs and returns on repository error without panicking", func(t *testing.T) {
		repoErr := errors.New("db unavailable")
		repo := &fakeRepo{returns: []deleteReturn{{count: 0, err: repoErr}}}
		cleaner := iinbox.NewCleaner(repo, time.Hour, nil)

		// Should not panic or propagate the error to the caller.
		cleaner.Cleanup(context.Background())

		require.Len(t, repo.cutoffs, 1, "Cleanup should call DeleteOlderThan even when it returns an error")
	})

	t.Run("uses provided retention to derive cutoff", func(t *testing.T) {
		repo := &fakeRepo{returns: []deleteReturn{{count: 1, err: nil}, {count: 1, err: nil}}}
		shortRetention := 5 * time.Minute
		longRetention := 24 * time.Hour

		iinbox.NewCleaner(repo, shortRetention, nil).Cleanup(context.Background())
		iinbox.NewCleaner(repo, longRetention, nil).Cleanup(context.Background())

		require.Len(t, repo.cutoffs, 2, "Each Cleanup call should produce one DeleteOlderThan call")
		shortCutoff := repo.cutoffs[0].Unwrap()
		longCutoff := repo.cutoffs[1].Unwrap()

		require.True(
			t,
			shortCutoff.After(longCutoff),
			"Short retention should yield a more recent cutoff than long retention",
		)
	})
}

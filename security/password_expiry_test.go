package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubMetadataLoader struct {
	changedAt time.Time
	err       error
	called    bool
}

func (s *stubMetadataLoader) PasswordChangedAt(context.Context, *Principal) (time.Time, error) {
	s.called = true

	return s.changedAt, s.err
}

func TestExpiryPasswordChangeChecker(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")
	maxAge := 90 * 24 * time.Hour

	t.Run("DisabledWhenMaxAgeZero", func(t *testing.T) {
		loader := &stubMetadataLoader{changedAt: time.Now().Add(-time.Hour)}
		checker := NewExpiryPasswordChangeChecker(loader, 0)

		data, err := checker.Check(ctx, principal)

		require.NoError(t, err, "a disabled checker should not error")
		assert.Nil(t, data, "a disabled checker should never require a change")
		assert.False(t, loader.called, "a disabled checker should not query the loader")
	})

	t.Run("NotExpiredWhenYoungerThanMaxAge", func(t *testing.T) {
		loader := &stubMetadataLoader{changedAt: time.Now().Add(-time.Hour)}
		checker := NewExpiryPasswordChangeChecker(loader, maxAge)

		data, err := checker.Check(ctx, principal)

		require.NoError(t, err, "a fresh password should not error")
		assert.Nil(t, data, "a password younger than max age should not require a change")
	})

	t.Run("ExpiredWhenOlderThanMaxAge", func(t *testing.T) {
		changedAt := time.Now().Add(-100 * 24 * time.Hour)
		loader := &stubMetadataLoader{changedAt: changedAt}
		checker := NewExpiryPasswordChangeChecker(loader, maxAge)

		data, err := checker.Check(ctx, principal)

		require.NoError(t, err, "an expired password should not error")
		require.NotNil(t, data, "an expired password should require a change")
		assert.Equal(t, PasswordChangeReasonExpired, data.Reason, "the reason should be expired")
		assert.Equal(t, changedAt.Add(maxAge), data.Meta["expiredAt"], "meta should report when the password expired")
	})

	t.Run("UnknownTimestampPasses", func(t *testing.T) {
		loader := &stubMetadataLoader{changedAt: time.Time{}}
		checker := NewExpiryPasswordChangeChecker(loader, maxAge)

		data, err := checker.Check(ctx, principal)

		require.NoError(t, err, "an unknown timestamp should not error")
		assert.Nil(t, data, "an unknown timestamp should not force a change")
	})

	t.Run("LoaderErrorPropagates", func(t *testing.T) {
		loadErr := errors.New("db down")
		loader := &stubMetadataLoader{err: loadErr}
		checker := NewExpiryPasswordChangeChecker(loader, maxAge)

		_, err := checker.Check(ctx, principal)

		require.ErrorIs(t, err, loadErr, "a loader error should propagate")
	})

	t.Run("NilLoaderPanics", func(t *testing.T) {
		assert.PanicsWithValue(t, "security: PasswordMetadataLoader is required", func() {
			NewExpiryPasswordChangeChecker(nil, maxAge)
		}, "constructing with a nil loader should panic")
	})
}

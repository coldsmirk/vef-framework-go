package security

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/result"
)

// TestNewMemoryChallengeTokenStore tests constructor and interface compliance.
func TestNewMemoryChallengeTokenStore(t *testing.T) {
	t.Run("CreatesValidStore", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()

		assert.NotNil(t, store, "Store should not be nil")
		_, ok := store.(*MemoryChallengeTokenStore)
		assert.True(t, ok, "Should return *MemoryChallengeTokenStore")
	})

	t.Run("ImplementsInterface", func(*testing.T) {
		_ = NewMemoryChallengeTokenStore()
	})
}

// TestMemoryChallengeTokenStoreGenerate tests token generation scenarios.
func TestMemoryChallengeTokenStoreGenerate(t *testing.T) {
	t.Run("WithPendingAndResolved", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		principal := NewUser("user1", "Alice", "admin")

		token, err := store.Generate(context.Background(), &ChallengeState{
			AuthType:  AuthTypePassword,
			Principal: principal,
			Resolved:  []string{"sms"},
			Pending:   []string{"totp", "department"},
		})

		require.NoError(t, err, "Should generate token without error")
		assert.NotEmpty(t, token, "Should return a non-empty token")
	})

	t.Run("WithNilResolved", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		principal := NewUser("user2", "Bob")

		token, err := store.Generate(context.Background(), &ChallengeState{AuthType: AuthTypePassword, Principal: principal, Pending: []string{"totp"}})

		require.NoError(t, err, "Should generate token without error")
		assert.NotEmpty(t, token, "Should return a non-empty token")
	})

	t.Run("WithEmptySlices", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		principal := NewUser("user3", "Charlie")

		token, err := store.Generate(context.Background(), &ChallengeState{AuthType: AuthTypePassword, Principal: principal, Resolved: []string{}, Pending: []string{}})

		require.NoError(t, err, "Should generate token with empty slices")
		assert.NotEmpty(t, token, "Should return a non-empty token")
	})

	t.Run("WithDetails", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		principal := NewUser("user4", "Diana", "editor")
		principal.Details = map[string]any{"department": "engineering", "level": 3}

		token, err := store.Generate(context.Background(), &ChallengeState{AuthType: AuthTypePassword, Principal: principal, Pending: []string{"totp"}})

		require.NoError(t, err, "Should generate token with details")
		assert.NotEmpty(t, token, "Should return a non-empty token")
	})

	t.Run("WithNoRoles", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		principal := NewUser("user5", "Eve")

		token, err := store.Generate(context.Background(), &ChallengeState{AuthType: AuthTypePassword, Principal: principal, Pending: []string{"totp"}})

		require.NoError(t, err, "Should generate token without roles")
		assert.NotEmpty(t, token, "Should return a non-empty token")
	})
}

// TestMemoryChallengeTokenStoreParse tests token parsing and round-trip scenarios.
func TestMemoryChallengeTokenStoreParse(t *testing.T) {
	t.Run("RoundTripWithFullState", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		principal := NewUser("user1", "Alice", "admin", "editor")
		pending := []string{"totp", "department"}
		resolved := []string{"sms"}

		token, err := store.Generate(context.Background(), &ChallengeState{
			AuthType:  "wechat_mini",
			Username:  "alice@login",
			Principal: principal,
			Resolved:  resolved,
			Pending:   pending,
		})
		require.NoError(t, err, "Should generate token without error")

		state, err := store.Parse(context.Background(), token)
		require.NoError(t, err, "Should parse token without error")
		require.NotNil(t, state, "Should return non-nil state")

		assert.Equal(t, "wechat_mini", state.AuthType, "Should preserve the login mechanism")
		assert.Equal(t, "user1", state.Principal.ID, "Should preserve principal ID")
		assert.Equal(t, "Alice", state.Principal.Name, "Should preserve principal name")
		assert.Equal(t, "alice@login", state.Username, "Should preserve the original login identifier")
		assert.Equal(t, PrincipalTypeUser, state.Principal.Type, "Should create user principal")
		assert.Equal(t, []string{"admin", "editor"}, state.Principal.Roles, "Should preserve roles")
		assert.Equal(t, pending, state.Pending, "Should preserve pending list")
		assert.Equal(t, resolved, state.Resolved, "Should preserve resolved list")
	})

	t.Run("WithNilResolved", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		principal := NewUser("user2", "Bob")

		token, err := store.Generate(context.Background(), &ChallengeState{AuthType: AuthTypePassword, Principal: principal, Pending: []string{"totp"}})
		require.NoError(t, err, "Should generate token without error")

		state, err := store.Parse(context.Background(), token)
		require.NoError(t, err, "Should parse token without error")
		require.NotNil(t, state, "Should return non-nil state")

		assert.Equal(t, []string{"totp"}, state.Pending, "Should preserve pending list")
		assert.Nil(t, state.Resolved, "Should preserve nil resolved list")
	})

	t.Run("WithNoRoles", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		principal := NewUser("user3", "Charlie")

		token, err := store.Generate(context.Background(), &ChallengeState{AuthType: AuthTypePassword, Principal: principal, Pending: []string{"totp"}})
		require.NoError(t, err, "Should generate token without error")

		state, err := store.Parse(context.Background(), token)
		require.NoError(t, err, "Should parse token without error")
		require.NotNil(t, state, "Should return non-nil state")

		assert.Empty(t, state.Principal.Roles, "Should have empty roles")
	})

	t.Run("WithDetails", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		principal := NewUser("user4", "Diana", "admin")
		principal.Details = map[string]any{"department": "engineering"}

		token, err := store.Generate(context.Background(), &ChallengeState{AuthType: AuthTypePassword, Principal: principal, Pending: []string{"totp"}})
		require.NoError(t, err, "Should generate token without error")

		state, err := store.Parse(context.Background(), token)
		require.NoError(t, err, "Should parse token without error")
		require.NotNil(t, state, "Should return non-nil state")

		assert.NotNil(t, state.Principal.Details, "Should preserve details")
		details, ok := state.Principal.Details.(map[string]any)
		require.True(t, ok, "Details should be map[string]any")
		assert.Equal(t, "engineering", details["department"], "Should preserve detail values")
	})

	t.Run("SubjectWithAtSignInName", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		principal := NewUser("user5", "user@example.com")

		token, err := store.Generate(context.Background(), &ChallengeState{AuthType: AuthTypePassword, Principal: principal, Pending: []string{"totp"}})
		require.NoError(t, err, "Should generate token without error")

		state, err := store.Parse(context.Background(), token)
		require.NoError(t, err, "Should parse token without error")
		require.NotNil(t, state, "Should return non-nil state")

		assert.Equal(t, "user5", state.Principal.ID, "Should preserve principal ID")
		assert.Equal(t, "user@example.com", state.Principal.Name, "Should preserve name with @ sign")
	})

	t.Run("RejectsEmptyToken", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()

		_, err := store.Parse(context.Background(), "")
		require.Error(t, err, "Should reject empty token")

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error")
		assert.Equal(t, ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
	})

	t.Run("RejectsNonExistentToken", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()

		_, err := store.Parse(context.Background(), "non-existent-token-id")
		require.Error(t, err, "Should reject non-existent token")

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error")
		assert.Equal(t, ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
	})
}

// TestMemoryChallengeTokenStoreIsolation pins value semantics: the cache holds the
// state itself rather than a serialized copy, so neither the state a caller
// handed in nor any state handed out may reach the cached one.
func TestMemoryChallengeTokenStoreIsolation(t *testing.T) {
	t.Run("GeneratedStateIsCopied", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		state := &ChallengeState{
			AuthType:  AuthTypePassword,
			Principal: NewUser("user1", "Alice"),
			Resolved:  []string{"sms"},
			Pending:   []string{"totp", "department"},
		}

		token, err := store.Generate(context.Background(), state)
		require.NoError(t, err, "Should generate token without error")

		state.Resolved[0] = "overwritten"
		state.Pending[0] = "overwritten"

		parsed, err := store.Parse(context.Background(), token)
		require.NoError(t, err, "Should parse token without error")
		assert.Equal(t, []string{"sms"}, parsed.Resolved, "A write to the generated state's resolved list must not reach the cache")
		assert.Equal(t, []string{"totp", "department"}, parsed.Pending, "A write to the generated state's pending list must not reach the cache")
	})

	t.Run("ParsedStateIsDetached", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore().(*MemoryChallengeTokenStore)
		key := "seeded"

		// Seeded straight into the cache: Generate caches a copy without spare
		// capacity, and spare capacity is what lets an append write in place
		// instead of reallocating — the only way two handed-out states sharing a
		// backing array would show it.
		resolved := make([]string, 1, 4)
		resolved[0] = "sms"

		err := store.cache.Set(context.Background(), key, ChallengeState{
			AuthType:  AuthTypePassword,
			Principal: NewUser("user1", "Alice"),
			Resolved:  resolved,
			Pending:   []string{"totp", "department"},
		}, ChallengeTokenExpires)
		require.NoError(t, err, "Seeding the cache should succeed")

		first, err := store.Parse(context.Background(), key)
		require.NoError(t, err, "Should parse the seeded token the first time")

		second, err := store.Parse(context.Background(), key)
		require.NoError(t, err, "Should parse the seeded token the second time")

		// Two resolves racing on one token, each recording its own step.
		first.Resolved = append(first.Resolved, "totp")
		second.Resolved = append(second.Resolved, "department")
		first.Pending[0] = "tampered"

		assert.Equal(t, []string{"sms", "totp"}, first.Resolved, "One handed-out state's append must not be overwritten by another's")

		cached, err := store.Parse(context.Background(), key)
		require.NoError(t, err, "Should parse the seeded token a third time")
		assert.Equal(t, []string{"sms"}, cached.Resolved, "An append to handed-out state must not reach the cache")
		assert.Equal(t, []string{"totp", "department"}, cached.Pending, "A write to handed-out state must not reach the cache")
	})
}

// TestMemoryChallengeTokenStoreTokenUniqueness tests that multiple generates produce unique tokens.
func TestMemoryChallengeTokenStoreTokenUniqueness(t *testing.T) {
	t.Run("MultipleGeneratesProduceUniqueTokens", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		principal := NewUser("user1", "Alice", "admin")

		tokens := make(map[string]struct{}, 100)
		for range 100 {
			token, err := store.Generate(context.Background(), &ChallengeState{AuthType: AuthTypePassword, Principal: principal, Pending: []string{"totp"}})
			require.NoError(t, err, "Should generate token without error")

			tokens[token] = struct{}{}
		}

		assert.Len(t, tokens, 100, "All 100 tokens should be unique")
	})
}

// TestMemoryChallengeTokenStoreTTLExpiration tests token TTL behavior.
// ChallengeTokenExpires is 5 minutes, so we can only verify tokens are valid within the TTL window.
func TestMemoryChallengeTokenStoreTTLExpiration(t *testing.T) {
	t.Run("TokenAvailableWithinTTL", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()
		principal := NewUser("user1", "Alice", "admin")

		token, err := store.Generate(context.Background(), &ChallengeState{AuthType: AuthTypePassword, Principal: principal, Pending: []string{"totp"}})
		require.NoError(t, err, "Should generate token without error")

		state, err := store.Parse(context.Background(), token)
		require.NoError(t, err, "Should parse freshly generated token without error")
		assert.Equal(t, "user1", state.Principal.ID, "Should preserve principal ID")
	})
}

// TestMemoryChallengeTokenStoreConcurrency tests concurrent access safety.
func TestMemoryChallengeTokenStoreConcurrency(t *testing.T) {
	t.Run("ConcurrentGenerateAndParse", func(t *testing.T) {
		store := NewMemoryChallengeTokenStore()

		var wg sync.WaitGroup

		numGoroutines := 100

		for i := range numGoroutines {
			wg.Go(func() {
				principal := NewUser(fmt.Sprintf("user-%d", i), fmt.Sprintf("Name-%d", i))

				token, err := store.Generate(
					context.Background(),
					&ChallengeState{AuthType: AuthTypePassword, Principal: principal, Resolved: []string{"sms"}, Pending: []string{"totp"}},
				)
				assert.NoError(t, err, "Should generate token without error in goroutine %d", i)
				assert.NotEmpty(t, token, "Should return non-empty token in goroutine %d", i)

				state, err := store.Parse(context.Background(), token)
				assert.NoError(t, err, "Should parse token without error in goroutine %d", i)
				assert.NotNil(t, state, "Should return non-nil state in goroutine %d", i)
			})
		}

		wg.Wait()
	})
}

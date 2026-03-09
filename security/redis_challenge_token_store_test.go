package security

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/result"
)

type RedisChallengeTokenStoreTestSuite struct {
	suite.Suite

	container *testx.RedisContainer
	client    *redis.Client
	store     ChallengeTokenStore
}

func (s *RedisChallengeTokenStoreTestSuite) SetupSuite() {
	ctx := context.Background()
	s.container = testx.NewRedisContainer(ctx, s.T())

	s.client = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", s.container.Redis.Host, s.container.Redis.Port),
		DB:   int(s.container.Redis.Database),
	})

	err := s.client.Ping(ctx).Err()
	s.Require().NoError(err, "Should connect to Redis")

	s.store = NewRedisChallengeTokenStore(s.client)
}

func (s *RedisChallengeTokenStoreTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Close()
	}
}

func (s *RedisChallengeTokenStoreTestSuite) SetupTest() {
	s.client.FlushDB(context.Background())
}

// --- Generate ---

func (s *RedisChallengeTokenStoreTestSuite) TestGenerate() {
	s.Run("WithPendingAndResolved", func() {
		principal := NewUser("user1", "Alice", "admin")

		token, err := s.store.Generate(principal, []string{"totp", "department"}, []string{"sms"})

		s.Require().NoError(err, "Should generate token without error")
		s.NotEmpty(token, "Should return a non-empty token")
	})

	s.Run("WithNilResolved", func() {
		principal := NewUser("user2", "Bob")

		token, err := s.store.Generate(principal, []string{"totp"}, nil)

		s.Require().NoError(err, "Should generate token without error")
		s.NotEmpty(token, "Should return a non-empty token")
	})

	s.Run("WithEmptySlices", func() {
		principal := NewUser("user3", "Charlie")

		token, err := s.store.Generate(principal, []string{}, []string{})

		s.Require().NoError(err, "Should generate token with empty slices")
		s.NotEmpty(token, "Should return a non-empty token")
	})

	s.Run("WithDetails", func() {
		principal := NewUser("user4", "Diana", "editor")
		principal.Details = map[string]any{"department": "engineering", "level": 3}

		token, err := s.store.Generate(principal, []string{"totp"}, nil)

		s.Require().NoError(err, "Should generate token with details")
		s.NotEmpty(token, "Should return a non-empty token")
	})

	s.Run("WithNoRoles", func() {
		principal := NewUser("user5", "Eve")

		token, err := s.store.Generate(principal, []string{"totp"}, nil)

		s.Require().NoError(err, "Should generate token without roles")
		s.NotEmpty(token, "Should return a non-empty token")
	})
}

// --- Parse ---

func (s *RedisChallengeTokenStoreTestSuite) TestParse() {
	s.Run("RoundTripWithFullState", func() {
		principal := NewUser("user1", "Alice", "admin", "editor")
		pending := []string{"totp", "department"}
		resolved := []string{"sms"}

		token, err := s.store.Generate(principal, pending, resolved)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse token without error")
		s.Require().NotNil(state, "Should return non-nil state")

		s.Equal("user1", state.Principal.ID, "Should preserve principal ID")
		s.Equal("Alice", state.Principal.Name, "Should preserve principal name")
		s.Equal(PrincipalTypeUser, state.Principal.Type, "Should create user principal")
		s.Equal([]string{"admin", "editor"}, state.Principal.Roles, "Should preserve roles")
		s.Equal(pending, state.Pending, "Should preserve pending list")
		s.Equal(resolved, state.Resolved, "Should preserve resolved list")
	})

	s.Run("WithNilResolved", func() {
		principal := NewUser("user2", "Bob")

		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse token without error")
		s.Require().NotNil(state, "Should return non-nil state")

		s.Equal([]string{"totp"}, state.Pending, "Should preserve pending list")
		s.Nil(state.Resolved, "Should preserve nil resolved list")
	})

	s.Run("WithNoRoles", func() {
		principal := NewUser("user3", "Charlie")

		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse token without error")
		s.Require().NotNil(state, "Should return non-nil state")

		s.Empty(state.Principal.Roles, "Should have empty roles")
	})

	s.Run("WithDetails", func() {
		principal := NewUser("user4", "Diana", "admin")
		principal.Details = map[string]any{"department": "engineering"}

		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse token without error")
		s.Require().NotNil(state, "Should return non-nil state")

		s.NotNil(state.Principal.Details, "Should preserve details")
		data, err := json.Marshal(state.Principal.Details)
		s.Require().NoError(err, "Should marshal details")

		var details map[string]any
		s.Require().NoError(json.Unmarshal(data, &details), "Should unmarshal details")
		s.Equal("engineering", details["department"], "Should preserve detail values")
	})

	s.Run("SubjectWithAtSignInName", func() {
		principal := NewUser("user5", "user@example.com")

		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse token without error")
		s.Require().NotNil(state, "Should return non-nil state")

		s.Equal("user5", state.Principal.ID, "Should preserve principal ID")
		s.Equal("user@example.com", state.Principal.Name, "Should preserve name with @ sign")
	})

	s.Run("RejectsEmptyToken", func() {
		_, err := s.store.Parse("")
		s.Require().Error(err, "Should reject empty token")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
	})

	s.Run("RejectsNonExistentToken", func() {
		_, err := s.store.Parse("non-existent-token-id")
		s.Require().Error(err, "Should reject non-existent token")

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "Should return a result.Error")
		s.Equal(result.ErrCodeTokenInvalid, resErr.Code, "Should return token invalid error code")
	})
}

// --- Token Uniqueness ---

func (s *RedisChallengeTokenStoreTestSuite) TestTokenUniqueness() {
	s.Run("MultipleGeneratesProduceUniqueTokens", func() {
		principal := NewUser("user1", "Alice", "admin")

		tokens := make(map[string]struct{}, 100)
		for range 100 {
			token, err := s.store.Generate(principal, []string{"totp"}, nil)
			s.Require().NoError(err, "Should generate token without error")

			tokens[token] = struct{}{}
		}

		s.Len(tokens, 100, "All 100 tokens should be unique")
	})
}

// --- TTL Expiration ---

// ChallengeTokenExpires is 5 minutes, so we can only verify tokens are valid within the TTL window.
func (s *RedisChallengeTokenStoreTestSuite) TestTTLExpiration() {
	s.Run("TokenAvailableWithinTTL", func() {
		principal := NewUser("user1", "Alice", "admin")

		token, err := s.store.Generate(principal, []string{"totp"}, nil)
		s.Require().NoError(err, "Should generate token without error")

		state, err := s.store.Parse(token)
		s.Require().NoError(err, "Should parse freshly generated token without error")
		s.Equal("user1", state.Principal.ID, "Should preserve principal ID")
	})
}

// --- Concurrency ---

func (s *RedisChallengeTokenStoreTestSuite) TestConcurrency() {
	s.Run("ConcurrentGenerateAndParse", func() {
		var wg sync.WaitGroup

		numGoroutines := 100

		for i := range numGoroutines {
			wg.Go(func() {
				principal := NewUser(
					fmt.Sprintf("user-%d", i),
					fmt.Sprintf("Name-%d", i),
					"role1",
				)

				token, err := s.store.Generate(principal, []string{"totp"}, []string{"sms"})
				s.Require().NoError(err, "Should generate token without error in goroutine %d", i)
				s.NotEmpty(token, "Should return non-empty token in goroutine %d", i)

				state, err := s.store.Parse(token)
				s.Require().NoError(err, "Should parse token without error in goroutine %d", i)
				s.Require().NotNil(state, "Should return non-nil state in goroutine %d", i)
				s.Equal(fmt.Sprintf("user-%d", i), state.Principal.ID, "Should preserve principal ID in goroutine %d", i)
			})
		}

		wg.Wait()
	})
}

func TestRedisChallengeTokenStore(t *testing.T) {
	suite.Run(t, new(RedisChallengeTokenStoreTestSuite))
}

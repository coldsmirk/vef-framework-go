package mold

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
)

// MockDictionaryLoader records dictionary load calls for resolver tests.
type MockDictionaryLoader struct {
	mock.Mock
}

func (m *MockDictionaryLoader) Load(ctx context.Context, key string) (map[string]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(map[string]string), args.Error(1)
}

// CachedDictionaryResolverTestSuite tests the CachedDictionaryResolver component.
// Covers: caching behavior, invalidation (specific and global), error handling,
// edge cases (empty keys, not found), panic scenarios, and concurrent access.
type CachedDictionaryResolverTestSuite struct {
	suite.Suite

	ctx context.Context
	bus event.Bus
}

func (s *CachedDictionaryResolverTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.bus = eventtest.NewFakeBus()
}

func (s *CachedDictionaryResolverTestSuite) newResolver(loader DictionaryLoader) DictionaryResolver {
	return NewCachedDictionaryResolver(loader, s.bus)
}

func (s *CachedDictionaryResolverTestSuite) TestCachesEntries() {
	loader := new(MockDictionaryLoader)
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft":     "草稿",
		"published": "已发布",
	}, nil).Once()

	resolver := s.newResolver(loader)

	result, err := resolver.Resolve(s.ctx, "status", "published")
	s.NoError(err, "Published status should resolve successfully")
	s.Equal("已发布", result, "Published status should resolve to its display label")
	s.T().Logf("First resolve: status 'published' -> '%s'", result)

	result2, err := resolver.Resolve(s.ctx, "status", "draft")
	s.NoError(err, "Draft status should resolve from cache")
	s.Equal("草稿", result2, "Draft status should resolve to its cached display label")
	s.T().Logf("Second resolve (cached): status 'draft' -> '%s'", result2)

	loader.AssertExpectations(s.T())
}

func (s *CachedDictionaryResolverTestSuite) TestInvalidatesSpecificKeys() {
	loader := new(MockDictionaryLoader)
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft": "草稿",
	}, nil).Once()
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft":    "草稿",
		"archived": "已归档",
	}, nil).Once()

	resolver := s.newResolver(loader)

	first, err := resolver.Resolve(s.ctx, "status", "draft")
	s.NoError(err, "Draft status should resolve before invalidation")
	s.Equal("草稿", first, "Draft status should resolve to its original display label")
	s.T().Logf("Before invalidation: status 'draft' -> '%s'", first)

	s.Require().NoError(PublishDictionaryChangedEvent(s.ctx, s.bus, "status"),
		"Specific dictionary invalidation event should publish")
	s.T().Logf("Published invalidation event for 'status' key")

	second, err := resolver.Resolve(s.ctx, "status", "archived")
	s.NoError(err, "Archived status should resolve after invalidation")
	s.Equal("已归档", second, "Archived status should resolve from reloaded data")
	s.T().Logf("After invalidation: status 'archived' -> '%s'", second)

	loader.AssertExpectations(s.T())
}

func (s *CachedDictionaryResolverTestSuite) TestInvalidatesAllKeys() {
	loader := new(MockDictionaryLoader)
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft": "草稿",
	}, nil).Once()
	loader.On("Load", mock.Anything, "category").Return(map[string]string{
		"news": "新闻",
	}, nil).Once()
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft":     "草稿",
		"published": "已发布",
	}, nil).Once()

	resolver := s.newResolver(loader)

	firstStatus, err := resolver.Resolve(s.ctx, "status", "draft")
	s.NoError(err, "Status dictionary should resolve before invalidation")
	s.Equal("草稿", firstStatus, "Status dictionary should return the draft display label")
	s.T().Logf("Before invalidation: status 'draft' -> '%s'", firstStatus)

	firstCategory, err := resolver.Resolve(s.ctx, "category", "news")
	s.NoError(err, "Category dictionary should resolve before invalidation")
	s.Equal("新闻", firstCategory, "Category dictionary should return the news display label")
	s.T().Logf("Before invalidation: category 'news' -> '%s'", firstCategory)

	s.Require().NoError(PublishDictionaryChangedEvent(s.ctx, s.bus),
		"Global dictionary invalidation event should publish")
	s.T().Logf("Published global invalidation event (all keys)")

	updatedStatus, err := resolver.Resolve(s.ctx, "status", "published")
	s.NoError(err, "Status dictionary should resolve after global invalidation")
	s.Equal("已发布", updatedStatus, "Updated status dictionary should return the published display label")
	s.T().Logf("After invalidation: status 'published' -> '%s'", updatedStatus)

	loader.AssertExpectations(s.T())
}

func (s *CachedDictionaryResolverTestSuite) TestLoaderError() {
	loader := new(MockDictionaryLoader)
	expectedErr := context.DeadlineExceeded
	loader.On("Load", mock.Anything, "status").Return(map[string]string(nil), expectedErr).Once()

	resolver := s.newResolver(loader)

	result, err := resolver.Resolve(s.ctx, "status", "draft")
	s.Error(err, "Loader failure should return an error")
	s.True(errors.Is(err, expectedErr), "Error should wrap the original error")
	s.Contains(err.Error(), "failed to load dictionary \"status\"", "Error message should describe the failure")
	s.Equal("", result, "Loader failure should return an empty result")
	s.T().Logf("Loader error correctly propagated: %v", err)

	loader.AssertExpectations(s.T())
}

func (s *CachedDictionaryResolverTestSuite) TestEmptyKeyOrCode() {
	loader := new(MockDictionaryLoader)

	resolver := s.newResolver(loader)

	result1, err1 := resolver.Resolve(s.ctx, "", "code")
	s.NoError(err1, "Empty key should not error")
	s.Equal("", result1, "Empty key should return an empty result")
	s.T().Logf("Empty key case: returned '%s'", result1)

	result2, err2 := resolver.Resolve(s.ctx, "key", "")
	s.NoError(err2, "Empty code should not error")
	s.Equal("", result2, "Empty code should return an empty result")
	s.T().Logf("Empty code case: returned '%s'", result2)

	loader.AssertExpectations(s.T())
}

func (s *CachedDictionaryResolverTestSuite) TestCodeNotFound() {
	loader := new(MockDictionaryLoader)
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft":     "草稿",
		"published": "已发布",
	}, nil).Once()

	resolver := s.newResolver(loader)

	result, err := resolver.Resolve(s.ctx, "status", "archived")
	s.NoError(err, "Missing dictionary code should not error")
	s.Equal("", result, "Missing dictionary code should return an empty result")
	s.T().Logf("Code 'archived' not found in dictionary, returned empty result")

	loader.AssertExpectations(s.T())
}

func (s *CachedDictionaryResolverTestSuite) TestPanicsWhenLoaderIsNil() {
	s.Panics(func() {
		NewCachedDictionaryResolver(nil, s.bus)
	}, "Nil loader should panic")

	s.T().Logf("Correctly panicked with nil loader")
}

func (s *CachedDictionaryResolverTestSuite) TestPanicsWhenBusIsNil() {
	loader := new(MockDictionaryLoader)
	s.Panics(func() {
		NewCachedDictionaryResolver(loader, nil)
	}, "Nil bus should panic")

	s.T().Logf("Correctly panicked with nil bus")
}

func (s *CachedDictionaryResolverTestSuite) TestNilCacheCreatesDefault() {
	loader := new(MockDictionaryLoader)
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft": "草稿",
	}, nil).Once()

	resolver := NewCachedDictionaryResolver(loader, s.bus)

	result, err := resolver.Resolve(s.ctx, "status", "draft")
	s.NoError(err, "Default cache should resolve successfully")
	s.Equal("草稿", result, "Default cache should resolve the draft display label")
	s.T().Logf("Default cache works correctly: status 'draft' -> '%s'", result)

	loader.AssertExpectations(s.T())
}

// TestSingleflightMergesConcurrentRequests verifies that concurrent requests for the same dictionary key
// are merged by singleflight and only trigger one underlying load operation.
func (s *CachedDictionaryResolverTestSuite) TestSingleflightMergesConcurrentRequests() {
	loader := new(MockDictionaryLoader)

	// Setup mock to return dictionary data for the key
	dictData := map[string]string{
		"draft":     "草稿",
		"published": "已发布",
		"archived":  "已归档",
	}

	// The mock should be called only once, even though we make multiple concurrent requests
	loader.On("Load", mock.Anything, "status").
		Return(dictData, nil).
		Once()

	resolver := s.newResolver(loader)

	// Make multiple concurrent requests for the same dictionary key
	const numRequests = 10

	var wg sync.WaitGroup

	results := make([]string, numRequests)
	errors := make([]error, numRequests)

	s.T().Logf("Launching %d concurrent requests for 'status' dictionary", numRequests)

	for i := range numRequests {
		wg.Go(func() {
			// Different codes but same dictionary key
			codes := []string{"draft", "published", "archived"}
			code := codes[i%len(codes)]
			results[i], errors[i] = resolver.Resolve(s.ctx, "status", code)
		})
	}

	wg.Wait()

	// All requests should succeed
	successCount := 0
	for i := range numRequests {
		s.NoError(errors[i], "Request %d should not error", i)
		s.NotEmpty(results[i], "Request %d should return a result", i)

		// Verify the result matches expected value
		codes := []string{"draft", "published", "archived"}
		expectedCode := codes[i%len(codes)]
		expectedValue := dictData[expectedCode]
		s.Equal(expectedValue, results[i], "Request %d should return correct value", i)

		successCount++
	}

	s.T().Logf("All %d concurrent requests completed successfully", successCount)
	s.T().Logf("Loader was called only once (singleflight merged requests)")

	// The mock should have been called only once, proving that singleflight merged all requests
	loader.AssertExpectations(s.T())
}

// TestCachedDictionaryResolverTestSuite tests cached data dict resolver test suite functionality.
func TestCachedDictionaryResolver(t *testing.T) {
	suite.Run(t, new(CachedDictionaryResolverTestSuite))
}

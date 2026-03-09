package mold

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/event"
	ievent "github.com/coldsmirk/vef-framework-go/internal/event"
)

// CachedDataDictResolverTestSuite tests the CachedDataDictResolver component.
// Covers: caching behavior, invalidation (specific and global), error handling,
// edge cases (empty keys, not found), panic scenarios, and concurrent access.
type CachedDataDictResolverTestSuite struct {
	suite.Suite

	ctx context.Context
	bus event.Bus
}

func (s *CachedDataDictResolverTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.bus = ievent.NewMemoryBus([]event.Middleware{})

	err := s.bus.Start()
	s.Require().NoError(err, "Should not return error")
}

func (s *CachedDataDictResolverTestSuite) TearDownSuite() {
	_ = s.bus.Shutdown(context.Background())
}

func (s *CachedDataDictResolverTestSuite) newResolver(loader DataDictLoader) DataDictResolver {
	return NewCachedDataDictResolver(loader, s.bus)
}

func (s *CachedDataDictResolverTestSuite) TestCachesEntries() {
	loader := new(MockDataDictLoader)
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft":     "草稿",
		"published": "已发布",
	}, nil).Once()

	resolver := s.newResolver(loader)

	result, err := resolver.Resolve(s.ctx, "status", "published")
	s.NoError(err, "Should resolve 'published' status successfully")
	s.Equal("已发布", result, "Should return correct published value")
	s.T().Logf("First resolve: status 'published' -> '%s'", result)

	result2, err := resolver.Resolve(s.ctx, "status", "draft")
	s.NoError(err, "Should resolve 'draft' status from cache")
	s.Equal("草稿", result2, "Should return correct draft value from cache")
	s.T().Logf("Second resolve (cached): status 'draft' -> '%s'", result2)

	loader.AssertExpectations(s.T())
}

func (s *CachedDataDictResolverTestSuite) TestInvalidatesSpecificKeys() {
	loader := new(MockDataDictLoader)
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft": "草稿",
	}, nil).Once()
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft":    "草稿",
		"archived": "已归档",
	}, nil).Once()

	resolver := s.newResolver(loader)

	first, err := resolver.Resolve(s.ctx, "status", "draft")
	s.NoError(err, "Should resolve 'draft' status before invalidation")
	s.Equal("草稿", first, "Should return correct draft value")
	s.T().Logf("Before invalidation: status 'draft' -> '%s'", first)

	PublishDataDictChangedEvent(s.bus, "status")
	time.Sleep(10 * time.Millisecond)
	s.T().Logf("Published invalidation event for 'status' key")

	second, err := resolver.Resolve(s.ctx, "status", "archived")
	s.NoError(err, "Should resolve 'archived' status after invalidation")
	s.Equal("已归档", second, "Should return correct archived value from reloaded data")
	s.T().Logf("After invalidation: status 'archived' -> '%s'", second)

	loader.AssertExpectations(s.T())
}

func (s *CachedDataDictResolverTestSuite) TestInvalidatesAllKeys() {
	loader := new(MockDataDictLoader)
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
	s.NoError(err, "Should resolve 'status' before invalidation")
	s.Equal("草稿", firstStatus, "Should return correct status value")
	s.T().Logf("Before invalidation: status 'draft' -> '%s'", firstStatus)

	firstCategory, err := resolver.Resolve(s.ctx, "category", "news")
	s.NoError(err, "Should resolve 'category' before invalidation")
	s.Equal("新闻", firstCategory, "Should return correct category value")
	s.T().Logf("Before invalidation: category 'news' -> '%s'", firstCategory)

	PublishDataDictChangedEvent(s.bus)
	time.Sleep(10 * time.Millisecond)
	s.T().Logf("Published global invalidation event (all keys)")

	updatedStatus, err := resolver.Resolve(s.ctx, "status", "published")
	s.NoError(err, "Should resolve 'status' after global invalidation")
	s.Equal("已发布", updatedStatus, "Should return updated published value from reloaded data")
	s.T().Logf("After invalidation: status 'published' -> '%s'", updatedStatus)

	loader.AssertExpectations(s.T())
}

func (s *CachedDataDictResolverTestSuite) TestLoaderError() {
	loader := new(MockDataDictLoader)
	expectedErr := context.DeadlineExceeded
	loader.On("Load", mock.Anything, "status").Return(map[string]string(nil), expectedErr).Once()

	resolver := s.newResolver(loader)

	result, err := resolver.Resolve(s.ctx, "status", "draft")
	s.Error(err, "Should return error when loader fails")
	s.True(errors.Is(err, expectedErr), "Error should wrap the original error")
	s.Contains(err.Error(), "failed to load dictionary \"status\"", "Error message should describe the failure")
	s.Equal("", result, "Should return empty result on error")
	s.T().Logf("Loader error correctly propagated: %v", err)

	loader.AssertExpectations(s.T())
}

func (s *CachedDataDictResolverTestSuite) TestEmptyKeyOrCode() {
	loader := new(MockDataDictLoader)

	resolver := s.newResolver(loader)

	result1, err1 := resolver.Resolve(s.ctx, "", "code")
	s.NoError(err1, "Should not error with empty key")
	s.Equal("", result1, "Should return empty result for empty key")
	s.T().Logf("Empty key case: returned '%s'", result1)

	result2, err2 := resolver.Resolve(s.ctx, "key", "")
	s.NoError(err2, "Should not error with empty code")
	s.Equal("", result2, "Should return empty result for empty code")
	s.T().Logf("Empty code case: returned '%s'", result2)

	loader.AssertExpectations(s.T())
}

func (s *CachedDataDictResolverTestSuite) TestCodeNotFound() {
	loader := new(MockDataDictLoader)
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft":     "草稿",
		"published": "已发布",
	}, nil).Once()

	resolver := s.newResolver(loader)

	result, err := resolver.Resolve(s.ctx, "status", "archived")
	s.NoError(err, "Should not error when code is not found")
	s.Equal("", result, "Should return empty result for non-existent code")
	s.T().Logf("Code 'archived' not found in dictionary, returned empty result")

	loader.AssertExpectations(s.T())
}

func (s *CachedDataDictResolverTestSuite) TestPanicsWhenLoaderIsNil() {
	s.Panics(func() {
		NewCachedDataDictResolver(nil, s.bus)
	}, "Expected panic when loader is nil")

	s.T().Logf("Correctly panicked with nil loader")
}

func (s *CachedDataDictResolverTestSuite) TestPanicsWhenBusIsNil() {
	loader := new(MockDataDictLoader)
	s.Panics(func() {
		NewCachedDataDictResolver(loader, nil)
	}, "Expected panic when bus is nil")

	s.T().Logf("Correctly panicked with nil bus")
}

func (s *CachedDataDictResolverTestSuite) TestNilCacheCreatesDefault() {
	loader := new(MockDataDictLoader)
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft": "草稿",
	}, nil).Once()

	resolver := NewCachedDataDictResolver(loader, s.bus)

	result, err := resolver.Resolve(s.ctx, "status", "draft")
	s.NoError(err, "Should resolve successfully with default cache")
	s.Equal("草稿", result, "Should return correct value")
	s.T().Logf("Default cache works correctly: status 'draft' -> '%s'", result)

	loader.AssertExpectations(s.T())
}

// TestSingleflightMergesConcurrentRequests verifies that concurrent requests for the same dictionary key
// are merged by singleflight and only trigger one underlying load operation.
func (s *CachedDataDictResolverTestSuite) TestSingleflightMergesConcurrentRequests() {
	loader := new(MockDataDictLoader)

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

type MockDataDictLoader struct {
	mock.Mock
}

func (m *MockDataDictLoader) Load(ctx context.Context, key string) (map[string]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(map[string]string), args.Error(1)
}

// TestCachedDataDictResolverTestSuite tests cached data dict resolver test suite functionality.
func TestCachedDataDictResolver(t *testing.T) {
	suite.Run(t, new(CachedDataDictResolverTestSuite))
}

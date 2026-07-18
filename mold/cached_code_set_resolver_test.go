package mold

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
)

// MockCodeSetLoader records code set load calls for resolver tests.
type MockCodeSetLoader struct {
	mock.Mock
}

func (m *MockCodeSetLoader) Load(ctx context.Context, codeSet string) (map[string]string, error) {
	args := m.Called(ctx, codeSet)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(map[string]string), args.Error(1)
}

// CachedCodeSetResolverTestSuite tests the CachedCodeSetResolver component.
// Covers: caching behavior, invalidation (specific and global), error handling,
// edge cases (empty keys, not found), panic scenarios, and concurrent access.
type CachedCodeSetResolverTestSuite struct {
	suite.Suite

	ctx context.Context
	bus event.Bus
}

func (s *CachedCodeSetResolverTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.bus = eventtest.NewFakeBus()
}

func (s *CachedCodeSetResolverTestSuite) newResolver(loader CodeSetLoader) CodeSetResolver {
	return NewCachedCodeSetResolver(loader, s.bus)
}

func (s *CachedCodeSetResolverTestSuite) TestCachesEntries() {
	loader := new(MockCodeSetLoader)
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

func (s *CachedCodeSetResolverTestSuite) TestInvalidatesSpecificKeys() {
	loader := new(MockCodeSetLoader)
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

	s.Require().NoError(PublishCodeSetChangedEvent(s.ctx, s.bus, "status"),
		"Specific code set invalidation event should publish")
	s.T().Logf("Published invalidation event for 'status' code set")

	second, err := resolver.Resolve(s.ctx, "status", "archived")
	s.NoError(err, "Archived status should resolve after invalidation")
	s.Equal("已归档", second, "Archived status should resolve from reloaded data")
	s.T().Logf("After invalidation: status 'archived' -> '%s'", second)

	loader.AssertExpectations(s.T())
}

func (s *CachedCodeSetResolverTestSuite) TestInvalidatesAllKeys() {
	loader := new(MockCodeSetLoader)
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
	s.NoError(err, "Status code set should resolve before invalidation")
	s.Equal("草稿", firstStatus, "Status code set should return the draft display label")
	s.T().Logf("Before invalidation: status 'draft' -> '%s'", firstStatus)

	firstCategory, err := resolver.Resolve(s.ctx, "category", "news")
	s.NoError(err, "Category code set should resolve before invalidation")
	s.Equal("新闻", firstCategory, "Category code set should return the news display label")
	s.T().Logf("Before invalidation: category 'news' -> '%s'", firstCategory)

	s.Require().NoError(PublishCodeSetChangedEvent(s.ctx, s.bus),
		"Global code set invalidation event should publish")
	s.T().Logf("Published global invalidation event (all keys)")

	updatedStatus, err := resolver.Resolve(s.ctx, "status", "published")
	s.NoError(err, "Status code set should resolve after global invalidation")
	s.Equal("已发布", updatedStatus, "Updated status code set should return the published display label")
	s.T().Logf("After invalidation: status 'published' -> '%s'", updatedStatus)

	loader.AssertExpectations(s.T())
}

func (s *CachedCodeSetResolverTestSuite) TestLoaderError() {
	loader := new(MockCodeSetLoader)
	expectedErr := context.DeadlineExceeded
	loader.On("Load", mock.Anything, "status").Return(map[string]string(nil), expectedErr).Once()

	resolver := s.newResolver(loader)

	result, err := resolver.Resolve(s.ctx, "status", "draft")
	s.Error(err, "Loader failure should return an error")
	s.ErrorIs(err, expectedErr, "Error should wrap the original error")
	s.Contains(err.Error(), "failed to load code set \"status\"", "Error message should describe the failure")
	s.Equal("", result, "Loader failure should return an empty result")
	s.T().Logf("Loader error correctly propagated: %v", err)

	loader.AssertExpectations(s.T())
}

func (s *CachedCodeSetResolverTestSuite) TestEmptyKeyOrCode() {
	loader := new(MockCodeSetLoader)

	resolver := s.newResolver(loader)

	result1, err1 := resolver.Resolve(s.ctx, "", "code")
	s.NoError(err1, "Empty code set should not error")
	s.Equal("", result1, "Empty code set should return an empty result")
	s.T().Logf("Empty code set case: returned '%s'", result1)

	result2, err2 := resolver.Resolve(s.ctx, "code_set", "")
	s.NoError(err2, "Empty code should not error")
	s.Equal("", result2, "Empty code should return an empty result")
	s.T().Logf("Empty code case: returned '%s'", result2)

	loader.AssertExpectations(s.T())
}

func (s *CachedCodeSetResolverTestSuite) TestCodeNotFound() {
	loader := new(MockCodeSetLoader)
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft":     "草稿",
		"published": "已发布",
	}, nil).Once()

	resolver := s.newResolver(loader)

	result, err := resolver.Resolve(s.ctx, "status", "archived")
	s.NoError(err, "Missing code should not error")
	s.Equal("", result, "Missing code should return an empty result")
	s.T().Logf("Code 'archived' not found in code set, returned empty result")

	loader.AssertExpectations(s.T())
}

func (s *CachedCodeSetResolverTestSuite) TestPanicsWhenLoaderIsNil() {
	s.Panics(func() {
		NewCachedCodeSetResolver(nil, s.bus)
	}, "Nil loader should panic")

	s.T().Logf("Correctly panicked with nil loader")
}

func (s *CachedCodeSetResolverTestSuite) TestPanicsWhenBusIsNil() {
	loader := new(MockCodeSetLoader)
	s.Panics(func() {
		NewCachedCodeSetResolver(loader, nil)
	}, "Nil bus should panic")

	s.T().Logf("Correctly panicked with nil bus")
}

func (s *CachedCodeSetResolverTestSuite) TestNilCacheCreatesDefault() {
	loader := new(MockCodeSetLoader)
	loader.On("Load", mock.Anything, "status").Return(map[string]string{
		"draft": "草稿",
	}, nil).Once()

	resolver := NewCachedCodeSetResolver(loader, s.bus)

	result, err := resolver.Resolve(s.ctx, "status", "draft")
	s.NoError(err, "Default cache should resolve successfully")
	s.Equal("草稿", result, "Default cache should resolve the draft display label")
	s.T().Logf("Default cache works correctly: status 'draft' -> '%s'", result)

	loader.AssertExpectations(s.T())
}

// TestSingleflightMergesConcurrentRequests verifies that concurrent requests for the same code set
// are merged by singleflight and only trigger one underlying load operation.
func (s *CachedCodeSetResolverTestSuite) TestSingleflightMergesConcurrentRequests() {
	loader := new(MockCodeSetLoader)

	// Setup mock to return code set data for the code set
	codeSetData := map[string]string{
		"draft":     "草稿",
		"published": "已发布",
		"archived":  "已归档",
	}

	// The mock should be called only once, even though we make multiple concurrent requests
	loader.On("Load", mock.Anything, "status").
		Return(codeSetData, nil).
		Once()

	resolver := s.newResolver(loader)

	// Make multiple concurrent requests for the same code set
	const numRequests = 10

	var wg sync.WaitGroup

	results := make([]string, numRequests)
	errors := make([]error, numRequests)

	s.T().Logf("Launching %d concurrent requests for 'status' code set", numRequests)

	for i := range numRequests {
		wg.Go(func() {
			// Different codes but same code set
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
		expectedValue := codeSetData[expectedCode]
		s.Equal(expectedValue, results[i], "Request %d should return correct value", i)

		successCount++
	}

	s.T().Logf("All %d concurrent requests completed successfully", successCount)
	s.T().Logf("Loader was called only once (singleflight merged requests)")

	// The mock should have been called only once, proving that singleflight merged all requests
	loader.AssertExpectations(s.T())
}

// TestCachedCodeSetResolver tests cached code set resolver test suite functionality.
func TestCachedCodeSetResolver(t *testing.T) {
	suite.Run(t, new(CachedCodeSetResolverTestSuite))
}

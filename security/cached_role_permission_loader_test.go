package security

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/event"
	ievent "github.com/coldsmirk/vef-framework-go/internal/event"
)

type CachedRolePermissionsLoaderTestSuite struct {
	suite.Suite

	ctx context.Context
	bus event.Bus
}

func (s *CachedRolePermissionsLoaderTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.bus = ievent.NewMemoryBus([]event.Middleware{})
	err := s.bus.(interface{ Start() error }).Start()
	s.Require().NoError(err, "Should start event bus")
}

func (s *CachedRolePermissionsLoaderTestSuite) TestCachesResults() {
	mockLoader := new(MockRolePermissionsLoader)

	adminPerms := map[string]DataScope{
		"test.read":  NewAllDataScope(),
		"test.write": NewAllDataScope(),
	}
	userPerms := map[string]DataScope{
		"test.read": NewSelfDataScope(""),
	}

	mockLoader.On("LoadPermissions", mock.Anything, "admin").
		Return(adminPerms, nil).
		Once()
	mockLoader.On("LoadPermissions", mock.Anything, "user").
		Return(userPerms, nil).
		Once()

	cachedLoader := NewCachedRolePermissionsLoader(mockLoader, s.bus)

	result, err := cachedLoader.LoadPermissions(s.ctx, "admin")
	s.Require().NoError(err, "Should load admin permissions")
	s.Require().Equal(2, len(result), "Admin should have 2 permissions")
	s.Require().Contains(result, "test.read", "Should contain expected value")
	s.Require().Contains(result, "test.write", "Should contain expected value")
	s.Require().NotContains(result, "test.delete", "Should not contain unexpected value")

	result2, err := cachedLoader.LoadPermissions(s.ctx, "admin")
	s.Require().NoError(err, "Should load cached admin permissions")
	s.Require().Equal(2, len(result2), "Admin should have 2 permissions")
	s.Require().Contains(result2, "test.read", "Should contain expected value")
	s.Require().Contains(result2, "test.write", "Should contain expected value")

	result3, err := cachedLoader.LoadPermissions(s.ctx, "user")
	s.Require().NoError(err, "Should load user permissions")
	s.Require().Equal(1, len(result3), "User should have 1 permission")
	s.Require().Contains(result3, "test.read", "Should contain expected value")
	s.Require().NotContains(result3, "test.write", "Should not contain unexpected value")

	result4, err := cachedLoader.LoadPermissions(s.ctx, "user")
	s.Require().NoError(err, "Should load cached user permissions")
	s.Require().Equal(1, len(result4), "User should have 1 permission")
	s.Require().Contains(result4, "test.read", "Should contain expected value")

	mockLoader.AssertExpectations(s.T())
}

func (s *CachedRolePermissionsLoaderTestSuite) TestInvalidatesSpecificRoles() {
	mockLoader := new(MockRolePermissionsLoader)

	adminPerms1 := map[string]DataScope{
		"test.read":  NewAllDataScope(),
		"test.write": NewAllDataScope(),
	}
	userPerms := map[string]DataScope{
		"test.read": NewSelfDataScope(""),
	}

	mockLoader.On("LoadPermissions", mock.Anything, "admin").
		Return(adminPerms1, nil).
		Once()
	mockLoader.On("LoadPermissions", mock.Anything, "user").
		Return(userPerms, nil).
		Once()

	adminPerms2 := map[string]DataScope{
		"test.read":   NewAllDataScope(),
		"test.write":  NewAllDataScope(),
		"test.delete": NewAllDataScope(),
	}
	mockLoader.On("LoadPermissions", mock.Anything, "admin").
		Return(adminPerms2, nil).
		Once()

	cachedLoader := NewCachedRolePermissionsLoader(mockLoader, s.bus)

	result, err := cachedLoader.LoadPermissions(s.ctx, "admin")
	s.Require().NoError(err, "Should load admin permissions")
	s.Require().Contains(result, "test.read", "Should contain expected value")
	s.Require().Contains(result, "test.write", "Should contain expected value")
	s.Require().NotContains(result, "test.delete", "Should not contain unexpected value")

	resultUser, err := cachedLoader.LoadPermissions(s.ctx, "user")
	s.Require().NoError(err, "Should load user permissions")
	s.Require().Contains(resultUser, "test.read", "Should contain expected value")

	PublishRolePermissionsChangedEvent(s.bus, "admin")
	time.Sleep(10 * time.Millisecond)

	result2, err := cachedLoader.LoadPermissions(s.ctx, "admin")
	s.Require().NoError(err, "Should reload admin permissions after invalidation")
	s.Require().Contains(result2, "test.read", "Should contain expected value")
	s.Require().Contains(result2, "test.write", "Should contain expected value")
	s.Require().Contains(result2, "test.delete", "Should contain expected value")

	resultUser2, err := cachedLoader.LoadPermissions(s.ctx, "user")
	s.Require().NoError(err, "Should load cached user permissions")
	s.Require().Contains(resultUser2, "test.read", "Should contain expected value")

	mockLoader.AssertExpectations(s.T())
}

func (s *CachedRolePermissionsLoaderTestSuite) TestInvalidatesAllRoles() {
	mockLoader := new(MockRolePermissionsLoader)

	adminPerms1 := map[string]DataScope{
		"test.read":  NewAllDataScope(),
		"test.write": NewAllDataScope(),
	}
	userPerms1 := map[string]DataScope{
		"test.read": NewSelfDataScope(""),
	}

	mockLoader.On("LoadPermissions", mock.Anything, "admin").
		Return(adminPerms1, nil).
		Once()
	mockLoader.On("LoadPermissions", mock.Anything, "user").
		Return(userPerms1, nil).
		Once()

	adminPerms2 := map[string]DataScope{
		"test.read":   NewAllDataScope(),
		"test.write":  NewAllDataScope(),
		"test.delete": NewAllDataScope(),
	}
	userPerms2 := map[string]DataScope{
		"test.read":   NewSelfDataScope(""),
		"test.update": NewSelfDataScope(""),
	}

	mockLoader.On("LoadPermissions", mock.Anything, "admin").
		Return(adminPerms2, nil).
		Once()
	mockLoader.On("LoadPermissions", mock.Anything, "user").
		Return(userPerms2, nil).
		Once()

	cachedLoader := NewCachedRolePermissionsLoader(mockLoader, s.bus)

	result, err := cachedLoader.LoadPermissions(s.ctx, "admin")
	s.Require().NoError(err, "Should load admin permissions")
	s.Require().Contains(result, "test.read", "Should contain expected value")
	s.Require().Contains(result, "test.write", "Should contain expected value")
	s.Require().NotContains(result, "test.delete", "Should not contain unexpected value")

	resultUser, err := cachedLoader.LoadPermissions(s.ctx, "user")
	s.Require().NoError(err, "Should load user permissions")
	s.Require().Contains(resultUser, "test.read", "Should contain expected value")
	s.Require().NotContains(resultUser, "test.update", "Should not contain unexpected value")

	PublishRolePermissionsChangedEvent(s.bus)
	time.Sleep(10 * time.Millisecond)

	result2, err := cachedLoader.LoadPermissions(s.ctx, "admin")
	s.Require().NoError(err, "Should reload admin permissions after invalidation")
	s.Require().Contains(result2, "test.read", "Should contain expected value")
	s.Require().Contains(result2, "test.write", "Should contain expected value")
	s.Require().Contains(result2, "test.delete", "Should contain expected value")

	resultUser2, err := cachedLoader.LoadPermissions(s.ctx, "user")
	s.Require().NoError(err, "Should reload user permissions after invalidation")
	s.Require().Contains(resultUser2, "test.read", "Should contain expected value")
	s.Require().Contains(resultUser2, "test.update", "Should contain expected value")

	mockLoader.AssertExpectations(s.T())
}

func (s *CachedRolePermissionsLoaderTestSuite) TestEmptyRole() {
	mockLoader := new(MockRolePermissionsLoader)

	emptyMap := make(map[string]DataScope)
	mockLoader.On("LoadPermissions", mock.Anything, "").
		Return(emptyMap, nil).
		Once()

	cachedLoader := NewCachedRolePermissionsLoader(mockLoader, s.bus)

	result, err := cachedLoader.LoadPermissions(s.ctx, "")
	s.Require().NoError(err, "Should load permissions for empty role")
	s.Require().NotNil(result, "Result should not be nil")
	s.Require().Empty(result, "Result should be empty")

	mockLoader.AssertExpectations(s.T())
}

func (s *CachedRolePermissionsLoaderTestSuite) TestLoaderError() {
	mockLoader := new(MockRolePermissionsLoader)
	expectedError := context.DeadlineExceeded
	mockLoader.On("LoadPermissions", mock.Anything, "admin").
		Return(nil, expectedError).
		Once()

	cachedLoader := NewCachedRolePermissionsLoader(mockLoader, s.bus)

	result, err := cachedLoader.LoadPermissions(s.ctx, "admin")
	s.Require().Error(err, "Should return error from loader")
	s.Require().Equal(expectedError, err, "Error should match expected error")
	s.Require().Nil(result, "Result should be nil on error")

	mockLoader.AssertExpectations(s.T())
}

// TestSingleflightMergesConcurrentRequests tests that concurrent requests for the same role
// are merged by singleflight and trigger only one underlying load operation.
func (s *CachedRolePermissionsLoaderTestSuite) TestSingleflightMergesConcurrentRequests() {
	mockLoader := new(MockRolePermissionsLoader)

	adminPerms := map[string]DataScope{
		"test.read":  NewAllDataScope(),
		"test.write": NewAllDataScope(),
	}

	mockLoader.On("LoadPermissions", mock.Anything, "admin").
		Return(adminPerms, nil).
		Once()

	cachedLoader := NewCachedRolePermissionsLoader(mockLoader, s.bus)

	const numRequests = 10

	var wg sync.WaitGroup

	results := make([]map[string]DataScope, numRequests)
	errors := make([]error, numRequests)

	for i := range numRequests {
		wg.Go(func() {
			results[i], errors[i] = cachedLoader.LoadPermissions(s.ctx, "admin")
		})
	}

	wg.Wait()

	for i := range numRequests {
		s.Require().NoError(errors[i], "Request %d should not error", i)
		s.Require().NotNil(results[i], "Request %d should return a result", i)
		s.Require().Contains(results[i], "test.read", "Should contain expected value")
		s.Require().Contains(results[i], "test.write", "Should contain expected value")
	}

	mockLoader.AssertExpectations(s.T())
}

type MockRolePermissionsLoader struct {
	mock.Mock
}

func (m *MockRolePermissionsLoader) LoadPermissions(ctx context.Context, role string) (map[string]DataScope, error) {
	args := m.Called(ctx, role)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(map[string]DataScope), args.Error(1)
}

// TestCachedRolePermissionsLoaderTestSuite tests cached role permissions loader test suite functionality.
func TestCachedRolePermissionsLoader(t *testing.T) {
	suite.Run(t, new(CachedRolePermissionsLoaderTestSuite))
}

package testx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type BaseMock struct {
	value string
}

type MockFeatureTestSuite struct {
	suite.Suite

	base *BaseMock
}

func (s *MockFeatureTestSuite) TestPlaceholder() {
	s.Equal("test", s.base.value, "TestPlaceholder should match expected value")
}

// TestRegistryAdd tests registry add functionality.
func TestRegistryAdd(t *testing.T) {
	r := NewRegistry[BaseMock]()

	r.Add(func(base *BaseMock) suite.TestingSuite {
		return &MockFeatureTestSuite{base: base}
	})

	assert.Len(t, r.factories, 1, "Registry should have 1 factory")
}

// TestRegistryNameExtraction tests registry name extraction functionality.
func TestRegistryNameExtraction(t *testing.T) {
	r := NewRegistry[BaseMock]()

	r.Add(func(base *BaseMock) suite.TestingSuite {
		return &MockFeatureTestSuite{base: base}
	})

	// Verify the name was extracted correctly (MockFeatureTestSuite → MockFeature)
	assert.Equal(t, "MockFeature", r.factories[0].name, "Should strip TestSuite suffix")
}

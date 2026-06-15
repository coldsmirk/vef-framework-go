package testx

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// SuiteFactory creates a testify suite instance from a shared base configuration.
type SuiteFactory[B any] func(base *B) suite.TestingSuite

type namedFactory[B any] struct {
	name    string
	factory SuiteFactory[B]
}

// SuiteRegistry holds suite factories and orchestrates their execution across databases.
type SuiteRegistry[B any] struct {
	factories []namedFactory[B]
}

// NewRegistry creates a new empty suite registry.
func NewRegistry[B any]() *SuiteRegistry[B] {
	return new(SuiteRegistry[B])
}

// Add registers a suite factory. The test name is auto-extracted from the concrete
// suite type name, with "TestSuite" suffix stripped for cleaner test output.
func (r *SuiteRegistry[B]) Add(factory SuiteFactory[B]) {
	var zero B

	s := factory(&zero)
	typeName := reflect.TypeOf(s).Elem().Name()
	name := strings.TrimSuffix(typeName, "TestSuite")

	r.factories = append(r.factories, namedFactory[B]{name: name, factory: factory})
}

// RunAll iterates all databases, creates a base via baseFactory for each, then runs
// every registered suite. Test hierarchy: TestAll/<DBDisplayName>/<SuiteName>/...
func (r *SuiteRegistry[B]) RunAll(t *testing.T, baseFactory func(env *DBEnv) *B) {
	ForEachDB(t, func(t *testing.T, env *DBEnv) {
		base := baseFactory(env)

		for _, f := range r.factories {
			t.Run(f.name, func(t *testing.T) {
				s := f.factory(base)
				suite.Run(t, s)
			})
		}
	})
}

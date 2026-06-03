package expression_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/expression"
)

func TestAsPredicate(t *testing.T) {
	var o expression.CompileOptions
	assert.False(t, o.Predicate, "Default options should not be a predicate")

	expression.AsPredicate()(&o)
	assert.True(t, o.Predicate, "AsPredicate should set Predicate")
}

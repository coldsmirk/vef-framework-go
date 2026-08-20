package lint_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/coldsmirk/vef-framework-go/internal/lint"
)

// TestUnusedParam covers both spellings the rule proposes — an entirely unnamed
// list and the blank identifier — and pins the grouped-name case, where a fix
// that only deletes names would silently drop parameters from the signature.
func TestUnusedParam(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), lint.UnusedParam, "unusedparam")
}

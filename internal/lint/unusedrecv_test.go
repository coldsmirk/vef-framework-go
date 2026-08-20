package lint_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/coldsmirk/vef-framework-go/internal/lint"
)

// TestUnusedRecv checks the reported diagnostics against the `want` comments in
// testdata and the applied fixes against the .golden file beside them, so a rule
// that stops reporting and a fix that rewrites the wrong text both fail here.
func TestUnusedRecv(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), lint.UnusedRecv, "unusedrecv")
}

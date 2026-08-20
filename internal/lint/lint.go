package lint

import "golang.org/x/tools/go/analysis"

// Analyzers returns every rule vef-lint enforces.
//
// Registering a rule here is all a new one needs: multichecker derives the
// command line from each Analyzer's Name, so every rule can be run, skipped, or
// applied with -fix on its own without any wiring of its own.
func Analyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		UnusedRecv,
		UnusedParam,
	}
}

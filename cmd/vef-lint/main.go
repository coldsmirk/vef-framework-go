package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/coldsmirk/vef-framework-go/internal/lint"
)

func main() {
	multichecker.Main(lint.Analyzers()...)
}

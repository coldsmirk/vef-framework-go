package main

import (
	"os"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd"
)

var (
	version = "0.0.1"
	date    = ""
)

func main() {
	cmd.Init(version, date)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

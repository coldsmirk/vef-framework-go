package main

import (
	"os"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd"
	"github.com/coldsmirk/vef-framework-go/config"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/logx"
)

var (
	version = "0.0.1"
	date    = ""
)

func main() {
	quietFrameworkLogs()
	cmd.Init(version, date)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// quietFrameworkLogs raises the framework's log threshold so linking a package
// that logs on its own schedule — the data source registry announcing that it
// closed, for one — does not interleave with a command's own output. An
// explicit VEF_LOG_LEVEL still wins, which is how a generation problem inside
// the framework is debugged.
func quietFrameworkLogs() {
	if os.Getenv(config.EnvLogLevel) == "" {
		ilogx.SetLevel(logx.LevelWarn)
	}
}

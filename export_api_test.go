package vef

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// unreachableDataSourceConfig points the primary at a port nothing listens on.
// That is the whole point of the test below: describing the API surface must
// not require the database to be up, so a build pipeline or a laptop with no
// containers running can produce the manifest.
const unreachableDataSourceConfig = `[vef.app]
name = "export-test"

[vef.data_sources.primary]
type = "postgres"
host = "127.0.0.1"
port = 1
user = "nobody"
password = ""
database = "nothing"
`

// writeConfigProject lays out a directory the framework's config loader finds
// and makes it the working directory for the test.
func writeConfigProject(t *testing.T) {
	t.Helper()

	root := t.TempDir()
	configs := filepath.Join(root, "configs")

	require.NoError(t, os.MkdirAll(configs, 0o755), "creating the config directory must succeed")
	require.NoError(t, os.WriteFile(filepath.Join(configs, "application.toml"),
		[]byte(unreachableDataSourceConfig), 0o644), "writing the config must succeed")

	t.Chdir(root)
}

func TestExportAPI(t *testing.T) {
	writeConfigProject(t)

	var out bytes.Buffer
	require.NoError(t, ExportAPI(&out),
		"exporting the API surface must succeed without a reachable database")

	var manifest struct {
		Framework string `json:"framework"`
		Resources []struct {
			Name       string `json:"name"`
			Kind       string `json:"kind"`
			Operations []struct {
				Action     string `json:"action"`
				Auth       string `json:"auth"`
				Permission string `json:"permission"`
				Params     string `json:"params"`
			} `json:"operations"`
		} `json:"resources"`
	}

	require.NoError(t, json.Unmarshal(out.Bytes(), &manifest), "the manifest must be valid JSON")

	t.Run("ReportsTheFrameworkVersion", func(t *testing.T) {
		require.NotEmpty(t, manifest.Framework, "the manifest must name the framework that produced it")
	})

	t.Run("IncludesTheBuiltInResources", func(t *testing.T) {
		names := make([]string, 0, len(manifest.Resources))
		for _, resource := range manifest.Resources {
			names = append(names, resource.Name)
		}

		require.Contains(t, names, "security/auth", "the core auth resource must be described")
		require.Contains(t, names, "sys/monitor", "the core monitor resource must be described")
	})

	t.Run("DescribesOperationsAndTheirPayloads", func(t *testing.T) {
		for _, resource := range manifest.Resources {
			if resource.Name != "security/auth" {
				continue
			}

			require.Equal(t, "rpc", resource.Kind, "a core resource is RPC")

			for _, op := range resource.Operations {
				if op.Action != "login" {
					continue
				}

				require.Equal(t, "none", op.Auth, "login must be reported as unauthenticated")
				require.NotEmpty(t, op.Params, "login's payload type must be resolved from its handler")

				return
			}
		}

		t.Fatal("the login operation must appear in the manifest")
	})
}

// TestExportAPIIsStable pins the property the whole format is designed around:
// the manifest is committed, so two runs of an unchanged application must
// produce identical bytes or every diff is noise.
func TestExportAPIIsStable(t *testing.T) {
	writeConfigProject(t)

	var first, second bytes.Buffer

	require.NoError(t, ExportAPI(&first), "the first export must succeed")
	require.NoError(t, ExportAPI(&second), "the second export must succeed")

	require.Equal(t, first.String(), second.String(), "two exports of one application must be byte-identical")
}

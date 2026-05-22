package resource_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/api/collector"
	iresource "github.com/coldsmirk/vef-framework-go/internal/approval/resource"
)

func TestManagementResourceRequiredPermissions(t *testing.T) {
	collectors := []api.OperationsCollector{
		collector.NewResourceProviderCollector(),
		collector.NewEmbeddedProviderCollector(),
	}

	t.Run("FlowResource", func(t *testing.T) {
		resource := iresource.NewFlowResource(nil, nil)
		specs := collectSpecs(resource, collectors...)

		expected := map[string]string{
			"create":          "approval:flow:create",
			"deploy":          "approval:flow:deploy",
			"publish_version": "approval:flow:publish",
			"get_graph":       "approval:flow:query",
			"find_flows":      "approval:flow:query",
			"update_flow":     "approval:flow:update",
			"toggle_active":   "approval:flow:update",
			"find_versions":   "approval:flow:query",
		}

		assertRequiredPermissions(t, specs, expected)
	})

	t.Run("CategoryResource", func(t *testing.T) {
		resource := iresource.NewCategoryResource()
		specs := collectSpecs(resource, collectors...)

		expected := map[string]string{
			"find_tree":         "approval:category:query",
			"find_tree_options": "approval:category:query",
			"create":            "approval:category:create",
			"update":            "approval:category:update",
			"delete":            "approval:category:delete",
		}

		assertRequiredPermissions(t, specs, expected)
	})

	t.Run("DelegationResource", func(t *testing.T) {
		resource := iresource.NewDelegationResource()
		specs := collectSpecs(resource, collectors...)

		expected := map[string]string{
			"find_page": "approval:delegation:query",
			"create":    "approval:delegation:create",
			"update":    "approval:delegation:update",
			"delete":    "approval:delegation:delete",
		}

		assertRequiredPermissions(t, specs, expected)
	})
}

func collectSpecs(resource api.Resource, collectors ...api.OperationsCollector) []api.OperationSpec {
	var specs []api.OperationSpec
	for _, item := range collectors {
		specs = append(specs, item.Collect(resource)...)
	}

	return specs
}

func assertRequiredPermissions(t *testing.T, specs []api.OperationSpec, expected map[string]string) {
	t.Helper()

	require.NotEmpty(t, specs, "Should collect operations from resource")

	permByAction := make(map[string]string, len(specs))
	for _, spec := range specs {
		permByAction[spec.Action] = spec.RequiredPermission
	}

	for action, permission := range expected {
		actual, exists := permByAction[action]
		require.True(t, exists, "Should expose %s action", action)
		assert.Equal(t, permission, actual, "Action %s should declare the expected RequiredPermission", action)
	}
}

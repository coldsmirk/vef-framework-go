package crud

import (
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/null"
)

// Default column mapping for DataOption.
var defaultDataOptionColumnMapping = &DataOptionColumnMapping{
	LabelColumn: defaultLabelColumn,
	ValueColumn: defaultValueColumn,
}

// DataOption represents a selectable item with display text and underlying value.
// Commonly used for dropdown lists, radio buttons, and select components.
type DataOption struct {
	// Display text shown to users
	Label string `json:"label" bun:"label"`
	// Underlying value used in form submissions
	Value string `json:"value" bun:"value"`
	// Additional descriptive text (optional)
	Description string `json:"description,omitempty" bun:"description"`
	// Custom metadata for extended functionality (optional)
	Meta map[string]any `json:"meta,omitempty" bun:"meta"`
}

// DataOptionColumnMapping defines the mapping between database columns and option fields.
type DataOptionColumnMapping struct {
	// Column name for label (default: "name")
	LabelColumn string `json:"labelColumn"`
	// Column name for value (default: "id")
	ValueColumn string `json:"valueColumn"`
	// Column name for description
	DescriptionColumn string `json:"descriptionColumn"`
	// Additional columns to include in meta field
	// Supports simple column names or "column AS alias" format
	// Examples: ["status", "role_name as role"]
	MetaColumns []string `json:"metaColumns"`
}

// DataOptionConfig is the API request meta for querying options.
type DataOptionConfig struct {
	api.M

	DataOptionColumnMapping
}

// TreeDataOption represents a hierarchical selectable item that can contain child options.
// Used for tree-structured selections like category menus or organizational hierarchies.
type TreeDataOption struct {
	DataOption

	// Unique identifier for the tree node
	ID string `json:"-" bun:"id"`
	// Parent node identifier (null for root nodes)
	ParentID null.String `json:"-" bun:"parent_id"`
	// Nested child options forming the tree structure
	Children []TreeDataOption `json:"children,omitempty" bun:"-"`
}

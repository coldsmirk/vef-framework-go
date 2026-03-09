package tabular

const (
	// TagTabular is the struct tag name for tabular field configuration.
	TagTabular = "tabular"

	// Tag attributes for field configuration.
	AttrDive      = "dive"      // Recursively parse embedded struct
	AttrName      = "name"      // Column name (header)
	AttrWidth     = "width"     // Column width (for display/export hints)
	AttrOrder     = "order"     // Column order
	AttrDefault   = "default"   // Default value for import
	AttrFormat    = "format"    // Format template (date/number format)
	AttrFormatter = "formatter" // Custom formatter name for export
	AttrParser    = "parser"    // Custom parser name for import

	// Special tag value.
	IgnoreField = "-" // Ignore this field
)

package formeditor

import "github.com/coldsmirk/vef-framework-go/approval"

// richSchema is the subset of the vef-framework-react form-editor schema the
// projector reads. The document carries far more (variables, per-device layout
// props, widget styling); only the value-bearing structure matters here, so
// everything else is ignored at unmarshal time. The `version` field is not
// interpreted — the TypeScript projector ignores it too, so any generation is
// accepted and the deploy-time projection is what pins compatibility.
type richSchema struct {
	DataSources   []richDataSource  `json:"dataSources"`
	Presentations richPresentations `json:"presentations"`
}

// richPresentations holds the per-device block trees. Both are optional at the
// wire level; the projector walks pc first, then mobile.
type richPresentations struct {
	PC     *richLayer `json:"pc"`
	Mobile *richLayer `json:"mobile"`
}

// richLayer is one device's root block list.
type richLayer struct {
	Children []richBlock `json:"children"`
}

// richTab is one tab of a tabs container; its children live in the enclosing
// value scope (tabs never rebind).
type richTab struct {
	Children []richBlock `json:"children"`
}

// richBlock is any node in the layout tree. The form-editor schema is a
// discriminated union keyed by Type; Go has no native sum type, so every
// variant's value-bearing fields are captured here and read per Type. Layout
// containers (section/flex/grid) carry Children, tabs carries Tabs, a subform
// carries Template + row bounds, and keyed leaves carry Key plus their widget
// props.
type richBlock struct {
	Type string `json:"type"`
	Key  string `json:"key"`
	// Label is a pointer so an omitted label (fall back to Key) is
	// distinguished from an explicit empty string, matching the designer's
	// `label ?? key` semantics.
	Label       *string           `json:"label"`
	ColumnType  string            `json:"columnType"`
	Placeholder string            `json:"placeholder"`
	Precision   *int              `json:"precision"`
	Validate    *richValidate     `json:"validate"`
	DataSource  *richOptionSource `json:"dataSource"`

	// Upload. MaxCount decides whether the value is one storage key or a list.
	MaxCount *int `json:"maxCount"`

	// Subform.
	Template []richBlock `json:"template"`
	MinRows  *int        `json:"minRows"`
	MaxRows  *int        `json:"maxRows"`

	// Layout containers.
	Children []richBlock `json:"children"`
	Tabs     []richTab   `json:"tabs"`
}

// richValidate mirrors the form-editor Validatable mixin. Numeric bounds are
// pointers so their presence is detectable; pattern/message are plain strings
// because the target ValidationRule cannot carry an empty one anyway.
type richValidate struct {
	Required  *bool    `json:"required"`
	MinLength *int     `json:"minLength"`
	MaxLength *int     `json:"maxLength"`
	Min       *float64 `json:"min"`
	Max       *float64 `json:"max"`
	Pattern   string   `json:"pattern"`
	Message   string   `json:"message"`
}

// richOptionSource is a selection field's inline option source
// (`{ kind: "static" | "ref" | "remote", ... }`). Static and ref-to-static
// resolve to a projected option list; a remote source is host-resolved at
// runtime and projects its request/mapping instead, so a consumer can replay it
// to render a stored value as its label.
type richOptionSource struct {
	Kind         string                        `json:"kind"`
	Options      []approval.FieldOption        `json:"options"`
	DataSourceID string                        `json:"dataSourceId"`
	Request      *approval.RemoteOptionRequest `json:"request"`
	Mapping      *approval.RemoteOptionMapping `json:"mapping"`
}

// richDataSource is a form-global, reusable option source referenced by fields
// through `{ kind: "ref", dataSourceId }`. It shares the approval FieldOption,
// RemoteOptionRequest and RemoteOptionMapping shapes verbatim — the designer's
// `{ label, value }`, `{ resource, action, ... }` and `{ labelKey, ... }` are
// the same wire contracts.
type richDataSource struct {
	ID      string                        `json:"id"`
	Kind    string                        `json:"kind"`
	Options []approval.FieldOption        `json:"options"`
	Request *approval.RemoteOptionRequest `json:"request"`
	Mapping *approval.RemoteOptionMapping `json:"mapping"`
}

// isLayoutContainer reports whether a node type is a pure-layout container whose
// body stays in the enclosing value scope. A subform is deliberately excluded:
// it opens a new value scope and is projected as a table field.
func isLayoutContainer(nodeType string) bool {
	switch nodeType {
	case "section", "tabs", "flex", "grid":
		return true
	default:
		return false
	}
}

// containerBodies returns the child block-lists of a layout container, in a
// stable order: one list for section/flex/grid, one per tab for tabs. Only ever
// called for a node isLayoutContainer accepts.
func containerBodies(node *richBlock) [][]richBlock {
	if node.Type == "tabs" {
		bodies := make([][]richBlock, 0, len(node.Tabs))
		for i := range node.Tabs {
			bodies = append(bodies, node.Tabs[i].Children)
		}

		return bodies
	}

	return [][]richBlock{node.Children}
}

// isKeyed reports whether a node binds a value (a non-empty key). An empty key
// is treated as non-keyed, matching the designer's isKeyedNode guard, so the
// runtime never binds a value to an empty path.
func isKeyed(node *richBlock) bool {
	return node.Key != ""
}

// walkRootKeyed visits every root-scope keyed node (keyed leaf or subform) in
// document order, recursing transparently through layout containers so their
// children stay root scope. A subform is visited but NOT descended: its
// template opens a deeper value scope and is projected separately as the
// subform's columns. The visit stops and returns the first error a visitor
// raises, which is what lets the projector fail the deploy on the first
// contract-breaking node.
func walkRootKeyed(blocks []richBlock, visit func(*richBlock) error) error {
	for i := range blocks {
		node := &blocks[i]

		if isLayoutContainer(node.Type) {
			for _, body := range containerBodies(node) {
				if err := walkRootKeyed(body, visit); err != nil {
					return err
				}
			}

			continue
		}

		// A subform is a keyed root node projected as a table; a keyed leaf is
		// projected as a scalar field. Non-keyed presentation nodes (button,
		// divider, alert-block, paragraph) bind no value and are skipped.
		if isKeyed(node) {
			if err := visit(node); err != nil {
				return err
			}
		}
	}

	return nil
}

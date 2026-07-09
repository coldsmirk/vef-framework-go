package formeditor

import (
	"bytes"
	"encoding/json"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// NewParser returns the built-in form-schema parser for the vef-framework-react
// form-editor rich schema.
//
// It mirrors @vef-framework-react/approval-form-bridge `projectFormSchema`
// (packages/approval-form-bridge/src/project.ts) field-for-field — the flat
// definition it derives is the same inventory the designer previews. The two
// implementations are a lockstep pair: any change to the projection semantics
// (kind mapping, column-type inference, subform / cross-device handling) must
// land in both, and the shared golden fixtures under testdata/ pin the parity.
func NewParser() approval.FormSchemaParser {
	return new(parser)
}

type parser struct{}

// ParseFormFields walks both device presentations (pc first, then mobile),
// projecting every root-scope keyed node into a flat field definition and
// deduplicating by key across devices. A cross-device kind or table-shape
// conflict, an unmappable / unknown widget, a nested subform, or an empty detail
// table aborts with an outward-facing error; a nil, empty, or null document
// yields no fields.
func (*parser) ParseFormFields(schema json.RawMessage) ([]approval.FormFieldDefinition, error) {
	if isBlankSchema(schema) {
		return nil, nil
	}

	var doc richSchema
	if err := json.Unmarshal(schema, &doc); err != nil {
		return nil, errSchemaMalformed(err)
	}

	dataSources := indexDataSources(doc.DataSources)

	// key → what the first (pc-winning) sighting projected, enough to detect a
	// cross-device contract conflict on a later sighting without re-projecting.
	seen := make(map[string]seenProjection)

	var fields []approval.FormFieldDefinition

	for _, layer := range []*richLayer{doc.Presentations.PC, doc.Presentations.Mobile} {
		if layer == nil {
			continue
		}

		err := walkRootKeyed(layer.Children, func(node *richBlock) error {
			if prev, ok := seen[node.Key]; ok {
				return checkCrossDevice(node, prev)
			}

			projected, err := projectNode(node, dataSources)
			if err != nil {
				return err
			}

			projected.SortOrder = len(fields)
			fields = append(fields, *projected)
			seen[node.Key] = newSeenProjection(node, projected.Kind)

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return fields, nil
}

// seenProjection records what an already-projected key resolved to, so a later
// cross-device sighting can be checked for a contract conflict.
type seenProjection struct {
	kind approval.FieldKind
	// tableSignature is the column signature of a table field, nil for scalars.
	tableSignature *string
}

func newSeenProjection(node *richBlock, kind approval.FieldKind) seenProjection {
	seen := seenProjection{kind: kind}
	if node.Type == "subform" {
		signature := peekTableSignature(node)
		seen.tableSignature = &signature
	}

	return seen
}

// checkCrossDevice reports whether a second sighting of an already-projected key
// would deploy a contract the first sighting's device cannot submit against. The
// submitted data contract is shared across devices, so a differing kind or table
// column-set is an error, not a preference. A same-kind options / validation
// divergence is not detected (documented limitation), and a second sighting that
// is itself unprojectable is silently deduped — pc already won the key.
func checkCrossDevice(node *richBlock, prev seenProjection) error {
	kind := peekKind(node)

	if kind != "" && kind != prev.kind {
		return errCrossDeviceKindMismatch(node.Key, prev.kind, kind)
	}

	if kind == approval.FieldTable && prev.tableSignature != nil && peekTableSignature(node) != *prev.tableSignature {
		return errCrossDeviceTableMismatch(node.Key)
	}

	return nil
}

// indexDataSources keys the form-global data sources by id for ref resolution.
func indexDataSources(sources []richDataSource) map[string]richDataSource {
	indexed := make(map[string]richDataSource, len(sources))
	for _, source := range sources {
		indexed[source.ID] = source
	}

	return indexed
}

// isBlankSchema reports whether the raw document is nil, whitespace-only, or the
// JSON null literal — each meaning "a flow without a form".
func isBlankSchema(schema json.RawMessage) bool {
	trimmed := bytes.TrimSpace(schema)

	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

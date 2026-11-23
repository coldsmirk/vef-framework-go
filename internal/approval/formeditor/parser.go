package formeditor

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/coldsmirk/go-collections"

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
// deduplicating by key across devices. When a key appears on both devices, both
// sides are projected and their data contracts (stripping display-only fields)
// are compared structurally so any divergence aborts the deploy. An unmappable
// / unknown widget, a nested subform, or an empty detail table also aborts with
// an outward-facing error; a nil, empty, or null document yields no fields.
//
// The parser is pure, so it ignores ctx; the parameter exists for host parsers
// that resolve remote sources while deriving fields (see FormSchemaParser).
func (*parser) ParseFormFields(_ context.Context, schema json.RawMessage) ([]approval.FormFieldDefinition, error) {
	if isBlankSchema(schema) {
		return nil, nil
	}

	var doc richSchema
	if err := json.Unmarshal(schema, &doc); err != nil {
		return nil, errSchemaMalformed(err)
	}

	dataSources := indexDataSources(doc.DataSources)
	pcFields := make(map[string]*approval.FormFieldDefinition)

	var fields []approval.FormFieldDefinition

	if doc.Presentations.PC != nil {
		pcSeen := collections.NewHashSet[string]()

		err := walkRootKeyed(doc.Presentations.PC.Children, func(node *richBlock) error {
			// Classify the widget BEFORE per-device dedupe so an unmappable /
			// unknown type aborts consistently regardless of sighting order.
			if err := classifyWidget(node); err != nil {
				return err
			}

			if !pcSeen.Add(node.Key) {
				return nil
			}

			projected, err := projectNode(node, dataSources)
			if err != nil {
				return err
			}

			projected.SortOrder = len(fields)
			fields = append(fields, *projected)
			pcFields[node.Key] = projected

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	if doc.Presentations.Mobile != nil {
		mobileSeen := collections.NewHashSet[string]()

		err := walkRootKeyed(doc.Presentations.Mobile.Children, func(node *richBlock) error {
			if err := classifyWidget(node); err != nil {
				return err
			}

			if !mobileSeen.Add(node.Key) {
				return nil
			}

			projected, err := projectNode(node, dataSources)
			if err != nil {
				return err
			}

			if pcField, exists := pcFields[node.Key]; exists {
				return compareFieldContracts(node.Key, pcField, projected)
			}

			projected.SortOrder = len(fields)
			fields = append(fields, *projected)

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return fields, nil
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

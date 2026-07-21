package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	"github.com/coldsmirk/vef-framework-go/mapx"
	"github.com/coldsmirk/vef-framework-go/reflectx"
)

// Identifier uniquely identifies an API operation.
type Identifier struct {
	Resource string `json:"resource" form:"resource" validate:"required,alphanum_us_slash" label_i18n:"api_request_resource"`
	Action   string `json:"action" form:"action" validate:"required" label_i18n:"api_request_action"`
	Version  string `json:"version" form:"version" validate:"required,alphanum" label_i18n:"api_request_version"`
}

// String returns a string representation of the identifier.
func (id Identifier) String() string {
	return id.Resource + ":" + id.Action + ":" + id.Version
}

// Params holds API request parameters.
//
// JSON payloads are parsed with number preservation: numeric values arrive as
// json.Number so Decode can project them into typed numeric fields and
// json.RawMessage captures without float64 precision loss. Decode surfaces
// numbers to untyped (any) targets as float64.
type Params map[string]any

// UnmarshalJSON parses params preserving numeric fidelity via json.Number.
func (p *Params) UnmarshalJSON(data []byte) error {
	return unmarshalNumberPreserving(data, (*map[string]any)(p))
}

// Decode decodes params into a struct. Request keys the target does not
// declare are ignored, so a client still sending a retired field keeps
// working; DecodeReportingUnmapped surfaces them instead of dropping them in
// silence.
func (p Params) Decode(out any) error {
	_, err := p.DecodeReportingUnmapped(out)

	return err
}

// DecodeReportingUnmapped decodes exactly like Decode and additionally reports
// the request keys the target struct does not declare, sorted. The framework's
// request pipeline logs them once per operation, which keeps a misspelled or
// retired field visible without failing a request older clients still send.
func (p Params) DecodeReportingUnmapped(out any) ([]string, error) {
	var metadata mapx.Metadata

	if err := decodeMapWithOptions(p, out, ErrInvalidParamsType, mapx.WithMetadata(&metadata)); err != nil {
		return nil, err
	}

	// Map iteration leaves Unused unordered; sort so callers can key on it.
	slices.Sort(metadata.Unused)

	return metadata.Unused, nil
}

// Meta holds API request metadata.
//
// JSON payloads are parsed with number preservation, exactly like Params.
type Meta map[string]any

// UnmarshalJSON parses meta preserving numeric fidelity via json.Number.
func (m *Meta) UnmarshalJSON(data []byte) error {
	return unmarshalNumberPreserving(data, (*map[string]any)(m))
}

// Decode decodes meta into a struct.
func (m Meta) Decode(out any) error {
	return decodeMap(m, out, ErrInvalidMetaType)
}

// unmarshalNumberPreserving decodes JSON with json.Decoder.UseNumber so
// numbers keep their exact digits instead of collapsing to float64.
func unmarshalNumberPreserving(data []byte, out *map[string]any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	return decoder.Decode(out)
}

// decodeMap decodes a map into a struct with type validation.
func decodeMap(data map[string]any, out any, typeErr error) error {
	return decodeMapWithOptions(data, out, typeErr)
}

func decodeMapWithOptions(data map[string]any, out any, typeErr error, options ...mapx.DecoderOption) error {
	if !reflectx.IsPointerToStruct(reflect.TypeOf(out)) {
		return fmt.Errorf("%w, got %T", typeErr, out)
	}

	decoder, err := mapx.NewDecoder(out, options...)
	if err != nil {
		return err
	}

	return decoder.Decode(data)
}

// Request represents a unified API request.
type Request struct {
	Identifier

	Params Params `json:"params"`
	Meta   Meta   `json:"meta"`
}

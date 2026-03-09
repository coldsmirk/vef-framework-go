//nolint:revive // package name is intentional
package api

import (
	"fmt"
	"reflect"

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
type Params map[string]any

// Decode decodes params into a struct.
func (p Params) Decode(out any) error {
	return decodeMap(p, out, ErrInvalidParamsType)
}

// Meta holds API request metadata.
type Meta map[string]any

// Decode decodes meta into a struct.
func (m Meta) Decode(out any) error {
	return decodeMap(m, out, ErrInvalidMetaType)
}

// decodeMap decodes a map into a struct with type validation.
func decodeMap(data map[string]any, out any, typeErr error) error {
	if !reflectx.IsPointerToStruct(reflect.TypeOf(out)) {
		return fmt.Errorf("%w, got %T", typeErr, out)
	}

	decoder, err := mapx.NewDecoder(out)
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

// GetParam retrieves a value from the request params by key.
func (r *Request) GetParam(key string) (any, bool) {
	if r.Params == nil {
		return nil, false
	}

	value, exists := r.Params[key]

	return value, exists
}

// GetMeta retrieves a value from the request metadata by key.
func (r *Request) GetMeta(key string) (any, bool) {
	if r.Meta == nil {
		return nil, false
	}

	value, exists := r.Meta[key]

	return value, exists
}

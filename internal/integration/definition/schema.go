package definition

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// CompileSchema parses and resolves raw as a self-contained JSON Schema
// (draft 2020-12). Remote $ref references are rejected — contract schemas
// live in the database and must validate deterministically and offline.
func CompileSchema(raw json.RawMessage) (*jsonschema.Resolved, error) {
	schema := new(jsonschema.Schema)
	if err := json.Unmarshal(raw, schema); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("resolve schema: %w", err)
	}

	return resolved, nil
}

package approval

import (
	"encoding/json"
	"fmt"
)

// FormData wraps a map to provide helper methods for form data operations.
type FormData map[string]any

func NewFormData(data map[string]any) FormData {
	if data == nil {
		return make(FormData)
	}

	return FormData(data)
}

func (f FormData) Get(key string) any      { return f[key] }
func (f FormData) Set(key string, val any) { f[key] = val }
func (f FormData) ToMap() map[string]any   { return f }

// Clone creates a deep copy via JSON serialization.
// It returns an error if the map contains values that cannot be marshaled to JSON
// (e.g. channels, functions). The caller should treat an error as uncloneable data
// and act accordingly rather than silently losing the original content.
func (f FormData) Clone() (FormData, error) {
	if len(f) == 0 {
		return make(FormData), nil
	}

	jsonBytes, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("approval: FormData.Clone marshal: %w", err)
	}

	var cloned map[string]any
	if err := json.Unmarshal(jsonBytes, &cloned); err != nil {
		return nil, fmt.Errorf("approval: FormData.Clone unmarshal: %w", err)
	}

	return FormData(cloned), nil
}

package approval

import "encoding/json"

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
func (f FormData) Clone() FormData {
	if len(f) == 0 {
		return make(FormData)
	}

	jsonBytes, err := json.Marshal(f)
	if err != nil {
		return make(FormData)
	}

	var cloned map[string]any
	if err := json.Unmarshal(jsonBytes, &cloned); err != nil {
		return make(FormData)
	}

	return FormData(cloned)
}

package cache

import "encoding/json"

// valueSerializer handles serialization/deserialization of cache values.
type valueSerializer[T any] interface {
	Serialize(value T) ([]byte, error)
	Deserialize(data []byte) (T, error)
}

// jsonSerializer implements valueSerializer using JSON encoding.
// It provides human-readable serialization format and cross-language compatibility.
type jsonSerializer[T any] struct{}

func (jsonSerializer[T]) Serialize(value T) ([]byte, error) {
	return json.Marshal(value)
}

func (jsonSerializer[T]) Deserialize(data []byte) (value T, err error) {
	err = json.Unmarshal(data, &value)

	return value, err
}

// newJSONSerializer creates a new JSON-based serializer.
func newJSONSerializer[T any]() valueSerializer[T] {
	return jsonSerializer[T]{}
}

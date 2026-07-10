package binding

import (
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/coldsmirk/vef-framework-go/approval"
)

type storedRecordKeyPart struct {
	Column string `json:"column"`
	Kind   string `json:"kind"`
	Value  string `json:"value"`
}

func recordKeysEqual(left, right json.RawMessage) (bool, error) {
	var leftParts []storedRecordKeyPart
	if err := json.Unmarshal(left, &leftParts); err != nil {
		return false, fmt.Errorf("decode stored business record key: %w", err)
	}

	var rightParts []storedRecordKeyPart
	if err := json.Unmarshal(right, &rightParts); err != nil {
		return false, fmt.Errorf("decode expected business record key: %w", err)
	}

	return slices.Equal(leftParts, rightParts), nil
}

func encodeRecordKey(config *approval.BusinessBindingConfig, key approval.BusinessRecordKey) (json.RawMessage, error) {
	validated, err := validateRecordKey(config, key)
	if err != nil {
		return nil, err
	}

	parts := make([]storedRecordKeyPart, 0, len(config.KeyColumns))
	for _, column := range config.KeyColumns {
		part, err := encodeRecordKeyPart(column, validated[column])
		if err != nil {
			return nil, err
		}

		parts = append(parts, part)
	}

	data, err := json.Marshal(parts)
	if err != nil {
		return nil, fmt.Errorf("encode business record key: %w", err)
	}

	return data, nil
}

func validateRecordKey(config *approval.BusinessBindingConfig, key approval.BusinessRecordKey) (approval.BusinessRecordKey, error) {
	if len(key) != len(config.KeyColumns) {
		return nil, fmt.Errorf("%w: expected %d key values, got %d", ErrInvalidBusinessRef, len(config.KeyColumns), len(key))
	}

	validated := make(approval.BusinessRecordKey, len(key))
	for _, column := range config.KeyColumns {
		value, ok := key[column]
		if !ok || value == nil {
			return nil, fmt.Errorf("%w: missing key column %q", ErrInvalidBusinessRef, column)
		}

		converted, err := driver.DefaultParameterConverter.ConvertValue(value)
		if err != nil {
			return nil, fmt.Errorf("%w: key column %q: %w", ErrInvalidBusinessRef, column, err)
		}

		validated[column] = converted
	}

	return validated, nil
}

func encodeRecordKeyPart(column string, value driver.Value) (storedRecordKeyPart, error) {
	part := storedRecordKeyPart{Column: column}

	switch value := value.(type) {
	case string:
		part.Kind, part.Value = "string", value
	case int64:
		part.Kind, part.Value = "int64", strconv.FormatInt(value, 10)
	case float64:
		part.Kind, part.Value = "float64", strconv.FormatFloat(value, 'g', -1, 64)
	case bool:
		part.Kind, part.Value = "bool", strconv.FormatBool(value)
	case []byte:
		part.Kind, part.Value = "bytes", base64.StdEncoding.EncodeToString(value)
	case time.Time:
		encoded, err := value.MarshalText()
		if err != nil {
			return storedRecordKeyPart{}, fmt.Errorf("%w: key column %q time value: %w", ErrInvalidBusinessRef, column, err)
		}

		part.Kind, part.Value = "time", string(encoded)

	default:
		return storedRecordKeyPart{}, fmt.Errorf("%w: key column %q has unsupported driver value %T", ErrInvalidBusinessRef, column, value)
	}

	return part, nil
}

func decodeRecordKey(data json.RawMessage) (approval.BusinessRecordKey, error) {
	var parts []storedRecordKeyPart
	if err := json.Unmarshal(data, &parts); err != nil {
		return nil, fmt.Errorf("decode stored business record key: %w", err)
	}

	if len(parts) == 0 {
		return nil, fmt.Errorf("%w: stored key is empty", ErrInvalidBusinessRef)
	}

	key := make(approval.BusinessRecordKey, len(parts))
	for _, part := range parts {
		if part.Column == "" {
			return nil, fmt.Errorf("%w: stored key has an empty column", ErrInvalidBusinessRef)
		}

		if _, exists := key[part.Column]; exists {
			return nil, fmt.Errorf("%w: duplicate stored key column %q", ErrInvalidBusinessRef, part.Column)
		}

		value, err := decodeRecordKeyPart(part)
		if err != nil {
			return nil, err
		}

		key[part.Column] = value
	}

	return key, nil
}

func decodeRecordKeyPart(part storedRecordKeyPart) (driver.Value, error) {
	switch part.Kind {
	case "string":
		return part.Value, nil
	case "int64":
		value, err := strconv.ParseInt(part.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("decode stored key column %q as int64: %w", part.Column, err)
		}

		return value, nil

	case "float64":
		value, err := strconv.ParseFloat(part.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("decode stored key column %q as float64: %w", part.Column, err)
		}

		return value, nil

	case "bool":
		value, err := strconv.ParseBool(part.Value)
		if err != nil {
			return nil, fmt.Errorf("decode stored key column %q as bool: %w", part.Column, err)
		}

		return value, nil

	case "bytes":
		value, err := base64.StdEncoding.DecodeString(part.Value)
		if err != nil {
			return nil, fmt.Errorf("decode stored key column %q as bytes: %w", part.Column, err)
		}

		return value, nil

	case "time":
		var value time.Time
		if err := value.UnmarshalText([]byte(part.Value)); err != nil {
			return nil, fmt.Errorf("decode stored key column %q as time: %w", part.Column, err)
		}

		return value, nil

	default:
		return nil, fmt.Errorf("%w: stored key column %q has unknown kind %q", ErrInvalidBusinessRef, part.Column, part.Kind)
	}
}

package mapx

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"reflect"

	"github.com/coldsmirk/go-collections"
)

// Cached reflect.Type values used by the decode hooks below.
var (
	jsonRawMessageType     = reflect.TypeFor[json.RawMessage]()
	fileHeaderPtrType      = reflect.TypeFor[*multipart.FileHeader]()
	fileHeaderPtrSliceType = reflect.TypeFor[[]*multipart.FileHeader]()

	// CollectionSetBuilders maps each supported collections set-family
	// interface (Set[T], SortedSet[T], ConcurrentSet[T], ConcurrentSortedSet[T])
	// to a builder that constructs the concrete instance from a slice/array
	// reflect.Value. Element types T cover string and all primitive numeric
	// kinds. Callers that need additional element types (for example a named
	// type satisfying cmp.Ordered) can extend the chain via WithDecodeHook or
	// by composing onto the package-level DecoderHook variable.
	collectionSetBuilders = buildCollectionSetRegistry()
)

// Float boundaries for safe conversion to int64/uint64.
//
// Math.MaxInt64 (2^63-1) and math.MaxUint64 (2^64-1) cannot be exactly
// represented in float64; the nearest representable values round up to 2^63
// and 2^64 respectively. Comparing a float64 against the rounded constants
// using `>` therefore misses the boundary itself: f == 2^63 would silently
// pass and `int64(f)` becomes implementation-defined. Using exclusive `>=`
// against the next-representable boundary (2^63 / 2^64, both exact in
// float64) closes that gap.
const (
	float64Pow63    = -float64(math.MinInt64) // 2^63, exact in float64.
	float64Pow64    = float64Pow63 * 2        // 2^64, exact in float64.
	float64MinInt64 = float64(math.MinInt64)  // -2^63, exact in float64.
)

// convertJSONRawMessage handles conversion of arbitrary data to json.RawMessage.
// When the target type is json.RawMessage ([]byte), it re-marshals the source value to JSON bytes.
func convertJSONRawMessage(_, to reflect.Type, value any) (any, error) {
	if to != jsonRawMessageType {
		return value, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(data), nil
}

// convertFileHeader handles conversion from []*multipart.FileHeader to *multipart.FileHeader.
func convertFileHeader(from, to reflect.Type, value any) (any, error) {
	if from == fileHeaderPtrSliceType && to == fileHeaderPtrType {
		if files := value.([]*multipart.FileHeader); len(files) == 1 {
			return files[0], nil
		}
	}

	return value, nil
}

// convertSliceToCollectionSet bridges a slice or array source (typically
// []any obtained from JSON arrays) into a registered collections set-family
// interface. Targets that are not registered fall through unchanged so the
// rest of the decode hook chain can handle them.
func convertSliceToCollectionSet(from, to reflect.Type, value any) (any, error) {
	if from.Kind() != reflect.Slice && from.Kind() != reflect.Array {
		return value, nil
	}

	builder, ok := collectionSetBuilders[to]
	if !ok {
		return value, nil
	}

	return builder(reflect.ValueOf(value))
}

// buildCollectionSetRegistry constructs the lookup table consumed by
// convertSliceToCollectionSet. It registers the four set-family interfaces for
// every supported element type. Adding a new element type requires a single
// registerCollectionSet call below.
func buildCollectionSetRegistry() map[reflect.Type]func(reflect.Value) (any, error) {
	registry := make(map[reflect.Type]func(reflect.Value) (any, error))

	registerCollectionSet[string](registry)
	registerCollectionSet[int](registry)
	registerCollectionSet[int8](registry)
	registerCollectionSet[int16](registry)
	registerCollectionSet[int32](registry)
	registerCollectionSet[int64](registry)
	registerCollectionSet[uint](registry)
	registerCollectionSet[uint8](registry)
	registerCollectionSet[uint16](registry)
	registerCollectionSet[uint32](registry)
	registerCollectionSet[uint64](registry)
	registerCollectionSet[float32](registry)
	registerCollectionSet[float64](registry)

	return registry
}

// registerCollectionSet wires the four set-family interfaces of element type T
// to their corresponding builders backed by HashSet, TreeSet,
// ConcurrentHashSet and ConcurrentSkipSet.
func registerCollectionSet[T cmp.Ordered](registry map[reflect.Type]func(reflect.Value) (any, error)) {
	registry[reflect.TypeFor[collections.Set[T]]()] = buildHashSet[T]
	registry[reflect.TypeFor[collections.SortedSet[T]]()] = buildTreeSet[T]
	registry[reflect.TypeFor[collections.ConcurrentSet[T]]()] = buildConcurrentHashSet[T]
	registry[reflect.TypeFor[collections.ConcurrentSortedSet[T]]()] = buildConcurrentSkipSet[T]
}

func buildHashSet[T cmp.Ordered](rv reflect.Value) (any, error) {
	return fillSet(rv, collections.NewHashSetWithCapacity[T](rv.Len()))
}

func buildTreeSet[T cmp.Ordered](rv reflect.Value) (any, error) {
	return fillSet(rv, collections.NewTreeSetOrdered[T]())
}

func buildConcurrentHashSet[T cmp.Ordered](rv reflect.Value) (any, error) {
	return fillSet(rv, collections.NewConcurrentHashSet[T]())
}

func buildConcurrentSkipSet[T cmp.Ordered](rv reflect.Value) (any, error) {
	return fillSet(rv, collections.NewConcurrentSkipSet[T]())
}

// fillSet copies elements from a slice/array reflect.Value into a typed set,
// converting compatible scalar widths and rejecting elements that would
// silently overflow, truncate, or mix incompatible kinds.
func fillSet[T cmp.Ordered, S interface{ Add(T) bool }](rv reflect.Value, out S) (any, error) {
	target := reflect.TypeFor[T]()

	for i := range rv.Len() {
		elem := rv.Index(i)
		// []any has elements of Kind Interface; unwrap to the concrete value
		// so subsequent kind checks see the source type, not interface{}.
		if elem.Kind() == reflect.Interface {
			elem = elem.Elem()
		}

		if !elem.IsValid() {
			return nil, fmt.Errorf("mapx: index %d, target %s: %w", i, target, ErrCollectionSetNilElement)
		}

		converted, err := convertScalar(elem, target)
		if err != nil {
			return nil, fmt.Errorf("mapx: element %d: %w", i, err)
		}

		out.Add(converted.Interface().(T))
	}

	return out, nil
}

// convertScalar converts a scalar reflect.Value into target, refusing
// cross-family conversions (string ↔ numeric) and any numeric conversion that
// would silently overflow, truncate a fractional component, or accept a
// non-finite float. The returned reflect.Value has type target.
func convertScalar(src reflect.Value, target reflect.Type) (reflect.Value, error) {
	targetKind := target.Kind()
	srcKind := src.Kind()

	if targetKind == reflect.String {
		if srcKind != reflect.String {
			return reflect.Value{}, fmt.Errorf("%s -> %s: %w", src.Type(), target, ErrCollectionSetIncompatibleKind)
		}

		return src.Convert(target), nil
	}

	if !isNumericKind(srcKind) {
		return reflect.Value{}, fmt.Errorf("%s -> %s: %w", src.Type(), target, ErrCollectionSetIncompatibleKind)
	}

	switch targetKind {
	case reflect.Float32, reflect.Float64:
		return convertToFloat(src, target)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return convertToInt(src, target)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return convertToUint(src, target)
	default:
		return reflect.Value{}, fmt.Errorf("target kind %s: %w", targetKind, ErrCollectionSetUnsupportedTarget)
	}
}

func isNumericKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func convertToFloat(src reflect.Value, target reflect.Type) (reflect.Value, error) {
	var f float64

	switch src.Kind() {
	case reflect.Float32, reflect.Float64:
		f = src.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		f = float64(src.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		f = float64(src.Uint())
	}

	if reflect.Zero(target).OverflowFloat(f) {
		return reflect.Value{}, fmt.Errorf("value %v -> %s: %w", f, target, ErrCollectionSetOverflow)
	}

	return reflect.ValueOf(f).Convert(target), nil
}

func convertToInt(src reflect.Value, target reflect.Type) (reflect.Value, error) {
	var iv int64

	switch src.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		iv = src.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uv := src.Uint()
		if uv > math.MaxInt64 {
			return reflect.Value{}, fmt.Errorf("value %d -> %s: %w", uv, target, ErrCollectionSetOverflow)
		}

		iv = int64(uv)

	case reflect.Float32, reflect.Float64:
		f := src.Float()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return reflect.Value{}, fmt.Errorf("value %v -> %s: %w", f, target, ErrCollectionSetNotFinite)
		}

		if f != math.Trunc(f) {
			return reflect.Value{}, fmt.Errorf("value %v -> %s: %w", f, target, ErrCollectionSetNonInteger)
		}

		// Exclusive upper bound: float64MinInt64 is exact (-2^63) but
		// math.MaxInt64 rounds up to 2^63 in float64, so `> MaxInt64`
		// would miss f == 2^63. Use `>= 2^63` instead.
		if f < float64MinInt64 || f >= float64Pow63 {
			return reflect.Value{}, fmt.Errorf("value %v -> %s: %w", f, target, ErrCollectionSetOverflow)
		}

		iv = int64(f)
	}

	if reflect.Zero(target).OverflowInt(iv) {
		return reflect.Value{}, fmt.Errorf("value %d -> %s: %w", iv, target, ErrCollectionSetOverflow)
	}

	return reflect.ValueOf(iv).Convert(target), nil
}

func convertToUint(src reflect.Value, target reflect.Type) (reflect.Value, error) {
	var uv uint64

	switch src.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uv = src.Uint()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		iv := src.Int()
		if iv < 0 {
			return reflect.Value{}, fmt.Errorf("value %d -> %s: %w", iv, target, ErrCollectionSetNegative)
		}

		uv = uint64(iv)

	case reflect.Float32, reflect.Float64:
		f := src.Float()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return reflect.Value{}, fmt.Errorf("value %v -> %s: %w", f, target, ErrCollectionSetNotFinite)
		}

		if f != math.Trunc(f) {
			return reflect.Value{}, fmt.Errorf("value %v -> %s: %w", f, target, ErrCollectionSetNonInteger)
		}

		// Exclusive upper bound: math.MaxUint64 rounds up to 2^64 in
		// float64, so `> MaxUint64` would miss f == 2^64. Use `>= 2^64`.
		if f < 0 || f >= float64Pow64 {
			return reflect.Value{}, fmt.Errorf("value %v -> %s: %w", f, target, ErrCollectionSetOverflow)
		}

		uv = uint64(f)
	}

	if reflect.Zero(target).OverflowUint(uv) {
		return reflect.Value{}, fmt.Errorf("value %d -> %s: %w", uv, target, ErrCollectionSetOverflow)
	}

	return reflect.ValueOf(uv).Convert(target), nil
}

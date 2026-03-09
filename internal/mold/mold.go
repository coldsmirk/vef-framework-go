package mold

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/coldsmirk/vef-framework-go/mold"
)

var (
	timeType           = reflect.TypeFor[time.Time]()
	restrictedAliasErr = "mold: alias %q either contains restricted characters or is the same as a restricted tag needed for normal operation"
	restrictedTagErr   = "mold: tag %q either contains restricted characters or is the same as a restricted tag needed for normal operation"
)

// MoldTransformer is the base controlling object which contains
// all necessary information.
type MoldTransformer struct {
	tagName          string
	aliases          map[string]string
	transformations  map[string]mold.Func
	structLevelFuncs map[reflect.Type]mold.StructLevelFunc
	interceptors     map[reflect.Type]mold.InterceptorFunc
	cCache           *structCache
	tCache           *tagCache
}

// New creates a new Transform object with default tag name of 'mold'.
func New() *MoldTransformer {
	tc := new(tagCache)
	tc.m.Store(make(map[string]*cTag))

	sc := new(structCache)
	sc.m.Store(make(map[reflect.Type]*cStruct))

	return &MoldTransformer{
		tagName:         "mold",
		aliases:         make(map[string]string),
		transformations: make(map[string]mold.Func),
		interceptors:    make(map[reflect.Type]mold.InterceptorFunc),
		cCache:          sc,
		tCache:          tc,
	}
}

// Register adds a transformation with the given tag.
//
// NOTE: This method is not thread-safe; register all transformations before use.
func (t *MoldTransformer) Register(tag string, fn mold.Func) {
	if tag == "" {
		panic("mold: transformation tag cannot be empty")
	}

	if fn == nil {
		panic("mold: transformation function cannot be nil")
	}

	if _, ok := restrictedTags[tag]; ok || strings.ContainsAny(tag, restrictedTagChars) {
		panic(fmt.Sprintf(restrictedTagErr, tag))
	}

	t.transformations[tag] = fn
}

// RegisterAlias registers a mapping of a single transform tag that
// defines a common or complex set of transformations.
//
// NOTE: This method is not thread-safe; register all aliases before use.
func (t *MoldTransformer) RegisterAlias(alias, tags string) {
	if alias == "" {
		panic("mold: transformation alias cannot be empty")
	}

	if tags == "" {
		panic("mold: aliased tags cannot be empty")
	}

	if _, ok := restrictedTags[alias]; ok || strings.ContainsAny(alias, restrictedTagChars) {
		panic(fmt.Sprintf(restrictedAliasErr, alias))
	}

	t.aliases[alias] = tags
}

// RegisterStructLevel registers a StructLevelFunc against a number of types.
//
// NOTE: This method is not thread-safe; register all struct-level functions before use.
func (t *MoldTransformer) RegisterStructLevel(fn mold.StructLevelFunc, types ...any) {
	if t.structLevelFuncs == nil {
		t.structLevelFuncs = make(map[reflect.Type]mold.StructLevelFunc)
	}

	for _, typ := range types {
		t.structLevelFuncs[reflect.TypeOf(typ)] = fn
	}
}

// RegisterInterceptor registers interceptor functions against one or more types.
// InterceptorFunc allows intercepting incoming values to redirect modifications to an inner type/value.
func (t *MoldTransformer) RegisterInterceptor(fn mold.InterceptorFunc, types ...any) {
	for _, typ := range types {
		t.interceptors[reflect.TypeOf(typ)] = fn
	}
}

// Struct applies transformations against the provided struct.
func (t *MoldTransformer) Struct(ctx context.Context, v any) error {
	orig := reflect.ValueOf(v)
	if orig.Kind() != reflect.Ptr || orig.IsNil() {
		return &ErrInvalidTransformValue{typ: reflect.TypeOf(v), fn: "Struct"}
	}

	val := orig.Elem()
	typ := val.Type()

	if val.Kind() != reflect.Struct || typ == timeType {
		return &ErrInvalidTransformation{typ: reflect.TypeOf(v)}
	}

	return t.setByStruct(ctx, orig, val, typ)
}

func (t *MoldTransformer) setByStruct(ctx context.Context, parent, current reflect.Value, typ reflect.Type) error {
	cs, ok := t.cCache.Get(typ)
	if !ok {
		var err error
		if cs, err = t.extractStructCache(current); err != nil {
			return err
		}
	}

	if cs.fn != nil {
		if err := cs.fn(ctx, MoldStructLevel{
			transformer: t,
			parent:      parent,
			current:     current,
		}); err != nil {
			return err
		}
	}

	for name, field := range cs.fields {
		if err := t.setByFieldWithContainer(ctx, name, current.Field(field.idx), field.cTags, current, cs); err != nil {
			return err
		}
	}

	return nil
}

// Field applies the provided transformations against the variable.
func (t *MoldTransformer) Field(ctx context.Context, v any, tags string) error {
	if tags == "" || tags == ignoreTag {
		return nil
	}

	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Pointer || val.IsNil() {
		return &ErrInvalidTransformValue{typ: reflect.TypeOf(v), fn: "Field"}
	}

	val = val.Elem()

	ctag, err := t.getOrParseTagCache(tags)
	if err != nil {
		return err
	}

	return t.setByField(ctx, val, ctag)
}

func (t *MoldTransformer) getOrParseTagCache(tags string) (*cTag, error) {
	if ctag, ok := t.tCache.Get(tags); ok {
		return ctag, nil
	}

	t.tCache.lock.Lock()
	defer t.tCache.lock.Unlock()

	if ctag, ok := t.tCache.Get(tags); ok {
		return ctag, nil
	}

	ctag, _, err := t.parseFieldTagsRecursive(tags, "", "", false)
	if err != nil {
		return nil, err
	}

	t.tCache.Set(tags, ctag)

	return ctag, nil
}

func (t *MoldTransformer) setByField(ctx context.Context, original reflect.Value, ct *cTag) error {
	return t.setByFieldInternal(ctx, "", original, ct, reflect.Value{}, nil)
}

func (t *MoldTransformer) setByFieldWithContainer(ctx context.Context, name string, original reflect.Value, ct *cTag, structValue reflect.Value, structCache *cStruct) error {
	return t.setByFieldInternal(ctx, name, original, ct, structValue, structCache)
}

func (t *MoldTransformer) setByFieldInternal(ctx context.Context, name string, original reflect.Value, ct *cTag, structValue reflect.Value, structCache *cStruct) (err error) {
	current, kind := t.extractType(original)

	if ct != nil && ct.hasTag {
		for ct != nil {
			switch ct.typeof {
			case typeEndKeys:
				return nil

			case typeDive:
				ct = ct.next

				return t.handleDive(ctx, current, kind, ct)

			default:
				if current, kind, err = t.applyTransformation(ctx, name, original, current, ct, structValue, structCache); err != nil {
					return err
				}

				ct = ct.next
			}
		}
	}

	return t.traverseStruct(ctx, current, original)
}

func (t *MoldTransformer) handleDive(ctx context.Context, current reflect.Value, kind reflect.Kind, ct *cTag) error {
	switch kind {
	case reflect.Slice, reflect.Array:
		return t.setByIterable(ctx, current, ct)
	case reflect.Map:
		return t.setByMap(ctx, current, ct)
	case reflect.Pointer:
		innerKind := current.Type().Elem().Kind()
		if innerKind == reflect.Slice || innerKind == reflect.Map {
			return nil
		}

		fallthrough

	default:
		return ErrInvalidDive
	}
}

func (t *MoldTransformer) applyTransformation(
	ctx context.Context,
	name string,
	original, current reflect.Value,
	ct *cTag,
	structValue reflect.Value,
	structCache *cStruct,
) (reflect.Value, reflect.Kind, error) {
	fl := MoldFieldLevel{
		transformer: t,
		name:        name,
		parent:      original,
		current:     current,
		param:       ct.param,
		container:   structValue,
		sc:          structCache,
	}

	if !current.CanAddr() {
		newVal := reflect.New(current.Type()).Elem()
		newVal.Set(current)
		fl.current = newVal

		if err := ct.fn(ctx, fl); err != nil {
			return current, current.Kind(), err
		}

		original.Set(reflect.Indirect(newVal))
		newCurrent, newKind := t.extractType(original)

		return newCurrent, newKind, nil
	}

	if err := ct.fn(ctx, fl); err != nil {
		return current, current.Kind(), err
	}

	newCurrent, newKind := t.extractType(current)

	return newCurrent, newKind, nil
}

func (t *MoldTransformer) traverseStruct(ctx context.Context, current, original reflect.Value) error {
	original2 := current
	current, kind := t.extractType(current)

	if kind != reflect.Struct {
		return nil
	}

	typ := current.Type()
	if typ == timeType {
		return nil
	}

	if !current.CanAddr() {
		newVal := reflect.New(typ).Elem()
		newVal.Set(current)

		if err := t.setByStruct(ctx, original, newVal, typ); err != nil {
			return err
		}

		original.Set(reflect.Indirect(newVal))

		return nil
	}

	return t.setByStruct(ctx, original2, current, typ)
}

func (t *MoldTransformer) setByIterable(ctx context.Context, current reflect.Value, ct *cTag) error {
	for i := range current.Len() {
		if err := t.setByField(ctx, current.Index(i), ct); err != nil {
			return err
		}
	}

	return nil
}

func (t *MoldTransformer) setByMap(ctx context.Context, current reflect.Value, ct *cTag) error {
	for _, key := range current.MapKeys() {
		newVal := reflect.New(current.Type().Elem()).Elem()
		newVal.Set(current.MapIndex(key))

		if ct != nil && ct.typeof == typeKeys && ct.keys != nil {
			current.SetMapIndex(key, reflect.Value{})

			newKey := reflect.New(current.Type().Key()).Elem()
			newKey.Set(key)
			key = newKey

			if err := t.setByField(ctx, key, ct.keys); err != nil {
				return err
			}

			if ct.next != nil {
				if err := t.setByField(ctx, newVal, ct.next); err != nil {
					return err
				}
			}
		} else {
			if err := t.setByField(ctx, newVal, ct); err != nil {
				return err
			}
		}

		current.SetMapIndex(key, newVal)
	}

	return nil
}

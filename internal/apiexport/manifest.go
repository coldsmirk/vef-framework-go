package apiexport

import (
	"encoding"
	"encoding/json"
	"reflect"
	"slices"
	"strings"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/api/handler"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
	"github.com/coldsmirk/vef-framework-go/version"
)

// Manifest is the application's API surface, rendered as data.
//
// It carries no timestamp and everything in it is sorted, so the file is
// byte-stable for an unchanged API. That is the point: the manifest is meant
// to be committed, and a reviewer reading its diff is reading the change to
// the contract.
type Manifest struct {
	// Framework is the version of the framework that produced the manifest.
	Framework string `json:"framework"`
	// Resources are the registered API resources, ordered by name.
	Resources []Resource `json:"resources"`
	// Types are the parameter structs the operations reference, keyed by their
	// fully qualified Go name.
	Types map[string]Type `json:"types,omitempty"`
}

// Resource is one registered API resource.
type Resource struct {
	// Name is the resource name operations are addressed under.
	Name string `json:"name"`
	// Kind is "rpc" or "rest".
	Kind string `json:"kind"`
	// Version is the resource's API version.
	Version string `json:"version"`
	// Operations are the resource's operations, ordered by action.
	Operations []Operation `json:"operations"`
}

// Operation is one callable endpoint.
type Operation struct {
	// Action is the operation name.
	Action string `json:"action"`
	// Auth is the authentication strategy this operation is served under. It
	// belongs to the operation rather than the resource because an operation
	// may override it — security/auth mixes a public login with authenticated
	// reads on one resource.
	Auth string `json:"auth"`
	// Permission is the token a caller must hold, empty when unrestricted.
	Permission string `json:"permission,omitempty"`
	// Audit reports whether the operation writes an audit record.
	Audit bool `json:"audit,omitempty"`
	// TimeoutMs is the request timeout in milliseconds.
	TimeoutMs int64 `json:"timeoutMs,omitempty"`
	// RateLimit is the operation's effective rate limit.
	RateLimit *RateLimit `json:"rateLimit,omitempty"`
	// Params names the entry in Manifest.Types describing the request payload,
	// empty for an operation that takes none.
	Params string `json:"params,omitempty"`
}

// RateLimit is an operation's effective request budget.
type RateLimit struct {
	// Max is the number of requests allowed per period.
	Max int `json:"max"`
	// PeriodMs is the window length in milliseconds.
	PeriodMs int64 `json:"periodMs"`
}

// Type describes a parameter struct.
type Type struct {
	// Fields are the struct's exported fields, in declaration order.
	Fields []Field `json:"fields"`
}

// Field is one member of a parameter struct.
type Field struct {
	// Name is the field's wire name, taken from its json tag.
	Name string `json:"name"`
	// Type is the field's Go type, with a named struct rendered as the key it
	// occupies in Manifest.Types.
	Type string `json:"type"`
	// Optional reports whether the field is a pointer or json-omitempty.
	Optional bool `json:"optional,omitempty"`
	// Label is the field's human-readable name from its label tag.
	Label string `json:"label,omitempty"`
	// Validate is the field's validation rule set.
	Validate string `json:"validate,omitempty"`
}

var (
	jsonMarshaler = reflect.TypeFor[json.Marshaler]()
	textMarshaler = reflect.TypeFor[encoding.TextMarshaler]()
)

// paramsSentinel is the embedded marker that identifies a request payload
// struct, which is what makes the parameter recognizable without guessing.
var paramsSentinel = reflect.TypeFor[api.P]()

// Build renders an engine's registered operations into a manifest.
func Build(operations []*api.Operation) Manifest {
	manifest := Manifest{Framework: version.VEFVersion, Types: map[string]Type{}}
	byResource := map[string]*Resource{}

	for _, op := range operations {
		resource := byResource[op.Resource]
		if resource == nil {
			resource = &Resource{
				Name:    op.Resource,
				Kind:    resourceKind(op),
				Version: op.Version,
			}
			byResource[op.Resource] = resource
		}

		resource.Operations = append(resource.Operations, buildOperation(op, manifest.Types))
	}

	manifest.Resources = make([]Resource, 0, len(byResource))
	for _, resource := range byResource {
		slices.SortFunc(resource.Operations, func(a, b Operation) int {
			return strings.Compare(a.Action, b.Action)
		})

		manifest.Resources = append(manifest.Resources, *resource)
	}

	slices.SortFunc(manifest.Resources, func(a, b Resource) int {
		return strings.Compare(a.Name, b.Name)
	})

	return manifest
}

func buildOperation(op *api.Operation, types map[string]Type) Operation {
	operation := Operation{
		Action:     op.Action,
		Auth:       authStrategy(op),
		Permission: requiredPermission(op),
		Audit:      op.EnableAudit,
		TimeoutMs:  op.Timeout.Milliseconds(),
		Params:     describeParams(op.Handler, types),
	}

	if op.RateLimit != nil {
		operation.RateLimit = &RateLimit{Max: op.RateLimit.Max, PeriodMs: op.RateLimit.Period.Milliseconds()}
	}

	return operation
}

// resourceKind reads the resource kind off the operation's captured resource.
func resourceKind(op *api.Operation) string {
	if resource, ok := op.Meta[shared.MetaKeyResource].(api.Resource); ok {
		return resource.Kind().String()
	}

	return ""
}

func authStrategy(op *api.Operation) string {
	if op.Auth == nil {
		return ""
	}

	return op.Auth.Strategy
}

// requiredPermission reads the permission the engine folded into the auth
// options while building the operation.
func requiredPermission(op *api.Operation) string {
	if op.Auth == nil {
		return ""
	}

	permission, _ := op.Auth.Options[shared.AuthOptionRequiredPermission].(string)

	return permission
}

// describeParams finds the request payload in a handler's signature and
// records its shape, returning the key it occupies in types.
//
// Operations reach here unadapted, so the handler is the resolver's wrapper
// rather than a bare function — and it is the wrapper that knows whether the
// function is a factory returning the real handler, which every crud operation
// is. Among the real handler's parameters exactly one is the caller's, the
// struct embedding api.P; the rest are resolved from the request.
func describeParams(operationHandler any, types map[string]Type) string {
	signature := handlerSignature(operationHandler)
	if signature == nil {
		return ""
	}

	for param := range signature.Ins() {
		if !isParamsStruct(param) {
			continue
		}

		return newWalker(types).describeType(param)
	}

	return ""
}

// walker records the types a manifest describes while walking them.
type walker struct {
	types map[string]Type
	// visiting names the structs currently open in the walk.
	//
	// Embedding is flattened into the outer type rather than recorded, so the
	// types map cannot serve as the cycle guard the way it does for named
	// fields — and a struct that anonymously embeds a pointer to itself is
	// legal, compiling Go. Without this set that shape recurses until the
	// stack is exhausted, which is a fatal runtime error no caller can recover
	// from: the export would take the whole process down.
	//
	// The type being described belongs in the set too, not just the ones being
	// flattened beneath it. A struct embedding a pointer to itself would
	// otherwise flatten one full copy of itself into itself and report every
	// field twice — which is also wrong on the wire, since encoding/json
	// resolves such a conflict in favor of the shallower field.
	visiting collections.Set[string]
}

func newWalker(types map[string]Type) *walker {
	return &walker{types: types, visiting: collections.NewHashSet[string]()}
}

// handlerSignature resolves an operation's handler to the function that serves
// a request, following the factory indirection the resolver recorded.
func handlerSignature(operationHandler any) reflect.Type {
	resolved, ok := operationHandler.(handler.Func)
	if !ok {
		return nil
	}

	signature := resolved.H().Type()
	if signature.Kind() != reflect.Func {
		return nil
	}

	if resolved.IsFactory() && signature.NumOut() > 0 && signature.Out(0).Kind() == reflect.Func {
		return signature.Out(0)
	}

	return signature
}

// isParamsStruct reports whether a handler parameter is the caller's payload.
func isParamsStruct(param reflect.Type) bool {
	param = deref(param)
	if param.Kind() != reflect.Struct {
		return false
	}

	for field := range param.Fields() {
		if field.Anonymous && field.Type == paramsSentinel {
			return true
		}
	}

	return false
}

// describeType records a struct's shape in types and returns its key. Types
// already recorded are not walked again, which also terminates recursion on a
// self-referential model.
func (w *walker) describeType(target reflect.Type) string {
	target = deref(target)

	key := typeKey(target)
	if key == "" || target.Kind() != reflect.Struct {
		return ""
	}

	if _, recorded := w.types[key]; recorded {
		return key
	}

	// Reserve the key before walking so a struct referencing itself terminates.
	w.types[key] = Type{}

	w.visiting.Add(key)
	defer w.visiting.Remove(key)

	described := Type{}

	for field := range target.Fields() {
		if field.PkgPath != "" || isSentinel(field) {
			continue
		}

		described.Fields = append(described.Fields, w.describeField(field)...)
	}

	w.types[key] = described

	return key
}

// describeField renders one struct field, flattening an embedded struct into
// the fields it contributes so the manifest describes the wire shape rather
// than the Go declaration.
func (w *walker) describeField(field reflect.StructField) []Field {
	name, omitempty := jsonName(field)
	if name == "-" {
		return nil
	}

	if field.Anonymous && name == "" && deref(field.Type).Kind() == reflect.Struct {
		return w.flattenEmbedded(field.Type)
	}

	if name == "" {
		name = field.Name
	}

	described := Field{
		Name:     name,
		Type:     w.fieldType(field.Type),
		Optional: omitempty || field.Type.Kind() == reflect.Pointer,
		Label:    field.Tag.Get("label"),
		Validate: field.Tag.Get("validate"),
	}

	return []Field{described}
}

// flattenEmbedded returns the fields an embedded struct contributes.
func (w *walker) flattenEmbedded(embedded reflect.Type) []Field {
	embedded = deref(embedded)

	// A type already being flattened further up the stack is a cycle; stopping
	// contributes no fields, which is the only finite answer available.
	key := typeKey(embedded)
	if key != "" {
		if w.visiting.Contains(key) {
			return nil
		}

		w.visiting.Add(key)
		defer w.visiting.Remove(key)
	}

	var fields []Field

	for field := range embedded.Fields() {
		if field.PkgPath != "" || isSentinel(field) {
			continue
		}

		fields = append(fields, w.describeField(field)...)
	}

	return fields
}

// fieldType renders a field's type, recording any named struct it reaches so
// the manifest is self-contained.
func (w *walker) fieldType(target reflect.Type) string {
	switch target.Kind() {
	case reflect.Pointer:
		return w.fieldType(target.Elem())
	case reflect.Slice, reflect.Array:
		return "[]" + w.fieldType(target.Elem())
	case reflect.Map:
		return "map[" + w.fieldType(target.Key()) + "]" + w.fieldType(target.Elem())
	case reflect.Struct:
		if isOpaqueStruct(target) {
			return target.String()
		}

		if key := w.describeType(target); key != "" {
			return key
		}

		return target.String()

	default:
		return target.Kind().String()
	}
}

// isOpaqueStruct reports whether a struct should be named rather than expanded.
//
// The test is whether the type marshals itself, not where it comes from. A type
// with its own MarshalJSON or MarshalText does not reach the wire as an object
// at all — time.Time and the framework's timex.DateTime both serialize to a
// string — so listing their fields would describe an implementation the client
// never sees. Expanding timex.DateTime in particular produced an entry with no
// fields at all, since everything time.Time carries is unexported: a manifest
// consumer would generate an empty object where the API really wants a string.
//
// Deciding this by package path instead — treating the standard library as
// opaque — gets both cases wrong: it misses first-party marshaling types, and
// the usual "no dot in the first path segment" shortcut for detecting the
// standard library also swallows every dotless module path, which is what a
// go.mod saying `module acme` produces for all of its own packages.
func isOpaqueStruct(target reflect.Type) bool {
	if target.PkgPath() == "" {
		return true
	}

	return marshalsItself(target) || marshalsItself(reflect.PointerTo(target))
}

// marshalsItself reports whether a type controls its own JSON representation.
func marshalsItself(target reflect.Type) bool {
	return target.Implements(jsonMarshaler) || target.Implements(textMarshaler)
}

// isSentinel reports whether a field is one of the api marker embeds, which
// describe how a struct is decoded rather than what it carries.
func isSentinel(field reflect.StructField) bool {
	return field.Anonymous && (field.Type == paramsSentinel || field.Type == reflect.TypeFor[api.M]())
}

// jsonName reads a field's wire name and whether it is omitted when empty.
func jsonName(field reflect.StructField) (name string, omitempty bool) {
	tag := field.Tag.Get("json")
	if tag == "" {
		return "", false
	}

	name, options, _ := strings.Cut(tag, ",")

	return name, strings.Contains(options, "omitempty")
}

// typeKey is a struct's fully qualified name, the key it occupies in the
// manifest's type table.
func typeKey(target reflect.Type) string {
	if target.Name() == "" || target.PkgPath() == "" {
		return ""
	}

	return target.PkgPath() + "." + target.Name()
}

func deref(target reflect.Type) reflect.Type {
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}

	return target
}

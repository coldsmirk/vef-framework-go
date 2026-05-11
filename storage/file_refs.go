package storage

import (
	"reflect"
	"strings"

	"github.com/coldsmirk/vef-framework-go/reflectx"
	"github.com/coldsmirk/vef-framework-go/strx"
)

// MetaType classifies how a struct field references uploaded files.
type MetaType string

const (
	// MetaTypeUploadedFile is a direct file key field (string / []string /
	// map[string]string).
	MetaTypeUploadedFile MetaType = "uploaded_file"
	// MetaTypeRichText is HTML content with embedded resource references.
	MetaTypeRichText MetaType = "richtext"
	// MetaTypeMarkdown is Markdown content with embedded resource references.
	MetaTypeMarkdown MetaType = "markdown"
)

const tagMeta = "meta"

// fieldKind classifies the storage shape of an uploaded_file meta field.
type fieldKind int

const (
	fieldKindScalar fieldKind = iota // string / *string
	fieldKindSlice                   // []string
	fieldKindMap                     // map[string]string
)

// metaField is the cached reflect-discovered descriptor of a single
// `meta`-tagged struct field.
type metaField struct {
	index []int
	typ   MetaType
	// Storage shape of the field (only meaningful for uploaded_file;
	// always fieldKindScalar for richtext / markdown).
	kind  fieldKind
	attrs map[string]string
}

// FileRef is a single file reference extracted from a model field tagged
// with `meta:"uploaded_file/richtext/markdown"`. Attrs carries any
// key/value attributes parsed from the tag value (e.g. `category:gallery`).
type FileRef struct {
	Key      string
	MetaType MetaType
	Attrs    map[string]string
}

// FileRefExtractor walks a value of type T and returns every uploaded
// file referenced by its `meta`-tagged fields. Construction performs
// a one-time reflect parse of T's struct shape, so callers should build
// the extractor once per type and reuse it.
type FileRefExtractor[T any] interface {
	// Extract returns every file ref reachable from model. Returns nil
	// for a nil pointer or a non-struct underlying type.
	Extract(model *T) []FileRef
	// Diff partitions refs across two snapshots of the same model type:
	//   toConsume = refs in newModel but not in oldModel (newly referenced)
	//   toDelete  = refs in oldModel but not in newModel (replaced/removed)
	// Either argument may be nil to signal "no model on that side"
	// (newModel=nil ⇒ delete-all; oldModel=nil ⇒ create).
	Diff(newModel, oldModel *T) (toConsume, toDelete []FileRef)
}

// NewFileRefExtractor builds an extractor for type T.
func NewFileRefExtractor[T any]() FileRefExtractor[T] {
	typ := reflectx.Indirect(reflect.TypeFor[T]())
	return &defaultExtractor[T]{fields: parseMetaFields(typ)}
}

type defaultExtractor[T any] struct {
	fields []metaField
}

func (e *defaultExtractor[T]) Extract(model *T) []FileRef {
	if model == nil {
		return nil
	}

	value := reflect.Indirect(reflect.ValueOf(model))
	if value.Kind() != reflect.Struct {
		return nil
	}

	return collectFileRefs(value, e.fields)
}

func (e *defaultExtractor[T]) Diff(newModel, oldModel *T) (toConsume, toDelete []FileRef) {
	newRefs := e.Extract(newModel)
	oldRefs := e.Extract(oldModel)

	newSet := make(map[string]struct{}, len(newRefs))
	for _, r := range newRefs {
		newSet[r.Key] = struct{}{}
	}

	oldSet := make(map[string]struct{}, len(oldRefs))
	for _, r := range oldRefs {
		oldSet[r.Key] = struct{}{}
	}

	for _, r := range newRefs {
		if _, in := oldSet[r.Key]; !in {
			toConsume = append(toConsume, r)
		}
	}

	for _, r := range oldRefs {
		if _, in := newSet[r.Key]; !in {
			toDelete = append(toDelete, r)
		}
	}

	return toConsume, toDelete
}

func collectFileRefs(value reflect.Value, fields []metaField) []FileRef {
	refs := make([]FileRef, 0)

	for _, f := range fields {
		fieldValue := value.FieldByIndex(f.index)
		if !fieldValue.IsValid() {
			continue
		}

		switch f.typ {
		case MetaTypeUploadedFile:
			collectUploadedFileRefs(fieldValue, f, &refs)

		case MetaTypeRichText:
			collectContentRefs(fieldValue, f, extractHtmlURLs, &refs)

		case MetaTypeMarkdown:
			collectContentRefs(fieldValue, f, extractMarkdownURLs, &refs)
		}
	}

	return refs
}

func collectUploadedFileRefs(fieldValue reflect.Value, f metaField, refs *[]FileRef) {
	switch f.kind {
	case fieldKindScalar:
		if key, ok := reflectx.GetStringValue(fieldValue); ok && key != "" {
			*refs = append(*refs, FileRef{Key: key, MetaType: f.typ, Attrs: f.attrs})
		}

	case fieldKindSlice:
		keys, ok := reflectx.GetStringSliceValue(fieldValue)
		if !ok {
			return
		}

		for _, k := range keys {
			if k = strings.TrimSpace(k); k != "" {
				*refs = append(*refs, FileRef{Key: k, MetaType: f.typ, Attrs: f.attrs})
			}
		}

	case fieldKindMap:
		entries, ok := reflectx.GetStringMapValue(fieldValue)
		if !ok {
			return
		}

		for _, k := range entries {
			if k = strings.TrimSpace(k); k != "" {
				*refs = append(*refs, FileRef{Key: k, MetaType: f.typ, Attrs: f.attrs})
			}
		}
	}
}

func collectContentRefs(fieldValue reflect.Value, f metaField, extract func(string) []string, refs *[]FileRef) {
	content, ok := reflectx.GetStringValue(fieldValue)
	if !ok || content == "" {
		return
	}

	for _, url := range extract(content) {
		*refs = append(*refs, FileRef{Key: url, MetaType: f.typ, Attrs: f.attrs})
	}
}

// parseMetaFields walks typ via reflectx and caches every `meta`-tagged
// field's storage shape for later extraction. Called once per
// FileRefExtractor construction.
func parseMetaFields(typ reflect.Type) []metaField {
	if typ.Kind() != reflect.Struct {
		return nil
	}

	fields := make([]metaField, 0)

	visitor := reflectx.TypeVisitor{
		VisitFieldType: func(field reflect.StructField, _ int) reflectx.VisitAction {
			tag, hasTag := field.Tag.Lookup(tagMeta)
			if !hasTag {
				return reflectx.SkipChildren
			}

			var (
				parsed = strx.ParseTag(tag, strx.WithBareValueMode(strx.BareAsKey))

				metaType      MetaType
				metaTypeValue string
				foundMetaType bool
			)

			for key, value := range parsed {
				if foundMetaType {
					break
				}

				switch MetaType(key) {
				case MetaTypeUploadedFile, MetaTypeRichText, MetaTypeMarkdown:
					metaType = MetaType(key)
					metaTypeValue = value
					foundMetaType = true
				}
			}

			if !foundMetaType {
				return reflectx.SkipChildren
			}

			// Format: "category:gallery public:true" -> {"category": "gallery", "public": "true"}
			attrs := strx.ParseTag(
				metaTypeValue,
				strx.WithSpacePairDelimiter(),
				strx.WithValueDelimiter(':'),
			)

			fieldType := field.Type

			var kind fieldKind

			// For "uploaded_file", support scalar (string/*string), slice
			// ([]string) and named map (map[string]string). For
			// "richtext"/"markdown", only scalar types are allowed.
			if metaType == MetaTypeUploadedFile {
				switch {
				case reflectx.IsStringSliceType(fieldType):
					kind = fieldKindSlice
				case reflectx.IsStringMapType(fieldType):
					kind = fieldKindMap
				case reflectx.IsStringType(fieldType):
					kind = fieldKindScalar
				default:
					return reflectx.SkipChildren
				}
			} else {
				if !reflectx.IsStringType(fieldType) {
					return reflectx.SkipChildren
				}

				kind = fieldKindScalar
			}

			fields = append(fields, metaField{
				index: field.Index,
				typ:   metaType,
				kind:  kind,
				attrs: attrs,
			})

			return reflectx.SkipChildren
		},
	}

	reflectx.VisitType(
		typ,
		visitor,
		reflectx.WithDiveTag(tagMeta, "dive"),
		reflectx.WithTraversalMode(reflectx.DepthFirst),
	)

	return fields
}

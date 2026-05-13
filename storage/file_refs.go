package storage

import (
	"reflect"
	"strings"

	"github.com/coldsmirk/go-collections"

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

// diffRefs partitions two ref sets by key:
//
//	toConsume = refs in newRefs whose key is not in oldRefs
//	toDelete  = refs in oldRefs whose key is not in newRefs
//
// Used exclusively by defaultFiles.OnUpdate (and its tests) to drive
// the create / delete partitioning that ConsumeMany / Schedule consume.
func diffRefs(newRefs, oldRefs []FileRef) (toConsume, toDelete []FileRef) {
	newKeys := refKeySet(newRefs)
	oldKeys := refKeySet(oldRefs)

	for _, r := range newRefs {
		if !oldKeys.Contains(r.Key) {
			toConsume = append(toConsume, r)
		}
	}

	for _, r := range oldRefs {
		if !newKeys.Contains(r.Key) {
			toDelete = append(toDelete, r)
		}
	}

	return toConsume, toDelete
}

// refKeySet collects every FileRef.Key into a HashSet. Returns an empty
// (non-nil) set when refs is empty so callers can Contains() safely.
func refKeySet(refs []FileRef) collections.Set[string] {
	keys := collections.NewHashSet[string]()
	for _, r := range refs {
		keys.Add(r.Key)
	}

	return keys
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
				// Return Continue (not SkipChildren) so reflectx's
				// shouldRecurse can honor the WithDiveTag(tagMeta, "dive")
				// configuration registered below: a field tagged
				// `meta:"dive"` reaches this branch (no metaType match)
				// and must allow reflectx to recurse into the nested
				// struct so its meta-tagged fields become reachable.
				return reflectx.Continue
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

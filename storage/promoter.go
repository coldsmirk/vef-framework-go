package storage

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/coldsmirk/go-collections"
	"github.com/coldsmirk/go-streams"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/reflectx"
	"github.com/coldsmirk/vef-framework-go/strx"
)

// Promoter defines the interface for automatic file field promotion and cleanup.
// It supports three types of meta information fields:
// - uploaded_file: Direct file fields (string, *string, []string, map[string]string)
// - richtext: Rich text fields (string, *string), automatically extracts and processes resource references in HTML
// - markdown: Markdown fields (string, *string), automatically extracts and processes resource references in Markdown.
type Promoter[T any] interface {
	// Promote handles file promotion and cleanup based on the scenario:
	// - newModel != nil && oldModel == nil: Create (promote new files)
	// - newModel != nil && oldModel != nil: Update (promote new files + cleanup replaced files)
	// - newModel == nil && oldModel != nil: Delete (cleanup all files)
	Promote(ctx context.Context, newModel, oldModel *T) error
}

// MetaType defines the type of meta information field.
type MetaType string

const (
	// MetaTypeUploadedFile indicates a direct file field (string, []string, or map[string]string).
	MetaTypeUploadedFile MetaType = "uploaded_file"
	// MetaTypeRichText indicates a rich text field containing HTML with resource references.
	MetaTypeRichText MetaType = "richtext"
	// MetaTypeMarkdown indicates a Markdown field containing resource references.
	MetaTypeMarkdown MetaType = "markdown"
)

const (
	tagMeta = "meta"
)

// fieldKind classifies the storage shape of an uploaded_file meta field.
type fieldKind int

const (
	fieldKindScalar fieldKind = iota // string / *string
	fieldKindSlice                   // []string
	fieldKindMap                     // map[string]string
)

// metaField represents the configuration of a meta information field.
type metaField struct {
	index []int
	typ   MetaType
	// Storage shape of the field (only meaningful for uploaded_file; always
	// fieldKindScalar for richtext / markdown).
	kind  fieldKind
	attrs map[string]string
}

type defaultPromoter[T any] struct {
	service   Service
	publisher event.Publisher
	fields    []metaField
}

// NewPromoter creates a new Promoter for type T.
// The publisher parameter is optional; if omitted, no events will be published.
func NewPromoter[T any](service Service, publisher ...event.Publisher) Promoter[T] {
	typ := reflectx.Indirect(reflect.TypeFor[T]())

	var pub event.Publisher
	if len(publisher) > 0 {
		pub = publisher[0]
	}

	return &defaultPromoter[T]{
		service:   service,
		publisher: pub,
		fields:    parseMetaFields(typ),
	}
}

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

			// For "uploaded_file", support scalar (string/*string), slice ([]string)
			// and named map (map[string]string). For "richtext"/"markdown", only
			// scalar types are allowed.
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

func convertToPermanentKey(key string) string {
	return strings.TrimPrefix(key, TempPrefix)
}

func (p *defaultPromoter[T]) publishEvent(evt event.Event) {
	if p.publisher != nil {
		p.publisher.Publish(evt)
	}
}

func (p *defaultPromoter[T]) Promote(ctx context.Context, newModel, oldModel *T) error {
	switch {
	case newModel != nil && oldModel != nil:
		if err := p.promoteFiles(ctx, newModel); err != nil {
			return err
		}

		return p.cleanupReplacedFiles(ctx, newModel, oldModel)

	case newModel != nil:
		return p.promoteFiles(ctx, newModel)

	case oldModel != nil:
		return p.deleteAllFiles(ctx, oldModel)

	default:
		return nil
	}
}

func (p *defaultPromoter[T]) promoteFiles(ctx context.Context, model *T) error {
	value := reflect.Indirect(reflect.ValueOf(model))
	if value.Kind() != reflect.Struct {
		return nil
	}

	return streams.FromSlice(p.fields).ForEachErr(func(field metaField) error {
		fieldValue := value.FieldByIndex(field.index)
		if !fieldValue.CanSet() {
			return nil
		}

		switch field.typ {
		case MetaTypeUploadedFile:
			return p.promoteUploadedFileField(ctx, fieldValue, field.kind, field.typ, field.attrs)

		case MetaTypeRichText:
			return p.promoteContentField(ctx, fieldValue, extractHtmlURLs, replaceHtmlURLs, field.typ, field.attrs)

		case MetaTypeMarkdown:
			return p.promoteContentField(ctx, fieldValue, extractMarkdownURLs, replaceMarkdownURLs, field.typ, field.attrs)

		default:
			return nil
		}
	})
}

func (p *defaultPromoter[T]) promoteUploadedFileField(ctx context.Context, fieldValue reflect.Value, kind fieldKind, metaType MetaType, attrs map[string]string) error {
	switch kind {
	case fieldKindScalar:
		key, valid := reflectx.GetStringValue(fieldValue)
		if !valid || key == "" {
			return nil
		}

		promotedKey, err := p.promoteSingleFile(ctx, key, metaType, attrs)
		if err != nil {
			return err
		}

		reflectx.SetStringValue(fieldValue, promotedKey)

	case fieldKindSlice:
		keys, valid := reflectx.GetStringSliceValue(fieldValue)
		if !valid || len(keys) == 0 {
			return nil
		}

		promoted := make([]string, 0, len(keys))

		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}

			promotedKey, err := p.promoteSingleFile(ctx, key, metaType, attrs)
			if err != nil {
				return err
			}

			promoted = append(promoted, promotedKey)
		}

		reflectx.SetStringSliceValue(fieldValue, promoted)

	case fieldKindMap:
		entries, valid := reflectx.GetStringMapValue(fieldValue)
		if !valid || len(entries) == 0 {
			return nil
		}

		promoted := make(map[string]string, len(entries))

		for name, key := range entries {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}

			promotedKey, err := p.promoteSingleFile(ctx, key, metaType, attrs)
			if err != nil {
				return err
			}

			promoted[name] = promotedKey
		}

		reflectx.SetStringMapValue(fieldValue, promoted)
	}

	return nil
}

func (p *defaultPromoter[T]) promoteContentField(
	ctx context.Context,
	fieldValue reflect.Value,
	extractFunc func(string) []string,
	replaceFunc func(string, map[string]string) string,
	metaType MetaType,
	attrs map[string]string,
) error {
	content, valid := reflectx.GetStringValue(fieldValue)
	if !valid || content == "" {
		return nil
	}

	urls := extractFunc(content)
	if len(urls) == 0 {
		return nil
	}

	// Only promote temp files; permanent files remain unchanged.
	replacements := make(map[string]string)

	err := streams.FromSlice(urls).ForEachErr(func(url string) error {
		if !strings.HasPrefix(url, TempPrefix) {
			return nil
		}

		promotedKey, err := p.promoteSingleFile(ctx, url, metaType, attrs)
		if err != nil {
			return err
		}

		if promotedKey != url {
			replacements[url] = promotedKey
		}

		return nil
	})
	if err != nil {
		return err
	}

	if len(replacements) > 0 {
		newContent := replaceFunc(content, replacements)
		reflectx.SetStringValue(fieldValue, newContent)
	}

	return nil
}

func (p *defaultPromoter[T]) promoteSingleFile(ctx context.Context, key string, metaType MetaType, attrs map[string]string) (string, error) {
	if !strings.HasPrefix(key, TempPrefix) {
		return key, nil
	}

	info, err := p.service.PromoteObject(ctx, key)
	if err != nil {
		if errors.Is(err, ErrObjectNotFound) {
			permanentKey := convertToPermanentKey(key)
			if _, err := p.service.StatObject(ctx, StatObjectOptions{Key: permanentKey}); err == nil {
				return permanentKey, nil
			}
		}

		return "", fmt.Errorf("failed to promote file %q: %w", key, err)
	}

	if info == nil {
		return key, nil
	}

	p.publishEvent(NewFilePromotedEvent(metaType, info.Key, attrs))

	return info.Key, nil
}

func (p *defaultPromoter[T]) cleanupReplacedFiles(ctx context.Context, newModel, oldModel *T) error {
	oldFiles := p.extractAllFileKeysWithInfo(oldModel)
	newKeys := p.extractAllFileKeys(newModel)

	newSet := collections.NewHashSetFrom(newKeys...)

	return streams.FromSlice(oldFiles).ForEachErr(func(fileInfo fileInfo) error {
		if newSet.Contains(fileInfo.key) {
			return nil
		}

		if err := p.service.DeleteObject(ctx, DeleteObjectOptions{Key: fileInfo.key}); err != nil {
			return fmt.Errorf("failed to delete file %q: %w", fileInfo.key, err)
		}

		p.publishEvent(NewFileDeletedEvent(fileInfo.metaType, fileInfo.key, fileInfo.attrs))

		return nil
	})
}

func (p *defaultPromoter[T]) deleteAllFiles(ctx context.Context, model *T) error {
	files := p.extractAllFileKeysWithInfo(model)

	return streams.FromSlice(files).ForEachErr(func(fileInfo fileInfo) error {
		fileInfo.key = strings.TrimSpace(fileInfo.key)
		if fileInfo.key == "" {
			return nil
		}

		if err := p.service.DeleteObject(ctx, DeleteObjectOptions{Key: fileInfo.key}); err != nil {
			return fmt.Errorf("failed to delete file %q: %w", fileInfo.key, err)
		}

		p.publishEvent(NewFileDeletedEvent(fileInfo.metaType, fileInfo.key, fileInfo.attrs))

		return nil
	})
}

type fileInfo struct {
	key      string
	metaType MetaType
	attrs    map[string]string
}

func (p *defaultPromoter[T]) extractAllFileKeysWithInfo(model *T) []fileInfo {
	if model == nil {
		return nil
	}

	value := reflect.Indirect(reflect.ValueOf(model))
	if value.Kind() != reflect.Struct {
		return nil
	}

	allFiles := make([]fileInfo, 0)

	for _, field := range p.fields {
		fieldValue := value.FieldByIndex(field.index)
		if !fieldValue.IsValid() {
			continue
		}

		switch field.typ {
		case MetaTypeUploadedFile:
			switch field.kind {
			case fieldKindScalar:
				if key, valid := reflectx.GetStringValue(fieldValue); valid && key != "" {
					allFiles = append(allFiles, fileInfo{
						key:      key,
						metaType: field.typ,
						attrs:    field.attrs,
					})
				}

			case fieldKindSlice:
				if keys, valid := reflectx.GetStringSliceValue(fieldValue); valid {
					for _, key := range keys {
						allFiles = append(allFiles, fileInfo{
							key:      key,
							metaType: field.typ,
							attrs:    field.attrs,
						})
					}
				}

			case fieldKindMap:
				if entries, valid := reflectx.GetStringMapValue(fieldValue); valid {
					for _, key := range entries {
						allFiles = append(allFiles, fileInfo{
							key:      key,
							metaType: field.typ,
							attrs:    field.attrs,
						})
					}
				}
			}

		case MetaTypeRichText:
			if content, valid := reflectx.GetStringValue(fieldValue); valid && content != "" {
				urls := extractHtmlURLs(content)
				for _, url := range urls {
					allFiles = append(allFiles, fileInfo{
						key:      url,
						metaType: field.typ,
						attrs:    field.attrs,
					})
				}
			}

		case MetaTypeMarkdown:
			if content, valid := reflectx.GetStringValue(fieldValue); valid && content != "" {
				urls := extractMarkdownURLs(content)
				for _, url := range urls {
					allFiles = append(allFiles, fileInfo{
						key:      url,
						metaType: field.typ,
						attrs:    field.attrs,
					})
				}
			}
		}
	}

	return allFiles
}

func (p *defaultPromoter[T]) extractAllFileKeys(model *T) []string {
	files := p.extractAllFileKeysWithInfo(model)

	keys := make([]string, len(files))
	for i, f := range files {
		keys[i] = f.key
	}

	return keys
}

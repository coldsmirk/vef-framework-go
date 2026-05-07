package storage

import (
	"context"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
)

// MockService is a mock implementation of Service for testing.
type MockService struct {
	files map[string]bool
}

func NewMockService() *MockService {
	return &MockService{
		files: make(map[string]bool),
	}
}

func (m *MockService) PutObject(_ context.Context, opts PutObjectOptions) (*ObjectInfo, error) {
	m.files[opts.Key] = true

	return &ObjectInfo{Key: opts.Key}, nil
}

func (*MockService) GetObject(_ context.Context, _ GetObjectOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (m *MockService) DeleteObject(_ context.Context, opts DeleteObjectOptions) error {
	delete(m.files, opts.Key)

	return nil
}

func (m *MockService) DeleteObjects(_ context.Context, opts DeleteObjectsOptions) error {
	for _, key := range opts.Keys {
		delete(m.files, key)
	}

	return nil
}

func (*MockService) ListObjects(_ context.Context, _ ListObjectsOptions) ([]ObjectInfo, error) {
	return nil, nil
}

func (*MockService) GetPresignedURL(_ context.Context, _ PresignedURLOptions) (string, error) {
	return "", nil
}

func (m *MockService) CopyObject(_ context.Context, opts CopyObjectOptions) (*ObjectInfo, error) {
	m.files[opts.DestKey] = true

	return &ObjectInfo{Key: opts.DestKey}, nil
}

func (m *MockService) MoveObject(_ context.Context, opts MoveObjectOptions) (*ObjectInfo, error) {
	m.files[opts.DestKey] = true
	delete(m.files, opts.SourceKey)

	return &ObjectInfo{Key: opts.DestKey}, nil
}

func (*MockService) StatObject(_ context.Context, _ StatObjectOptions) (*ObjectInfo, error) {
	return nil, nil
}

func (m *MockService) PromoteObject(_ context.Context, tempKey string) (*ObjectInfo, error) {
	if !strings.HasPrefix(tempKey, TempPrefix) {
		return nil, nil
	}

	permanentKey := strings.TrimPrefix(tempKey, TempPrefix)
	m.files[permanentKey] = true
	delete(m.files, tempKey)

	return &ObjectInfo{Key: permanentKey}, nil
}

// MockPublisher is a mock implementation of event.Publisher for testing.
type MockPublisher struct {
	Events []event.Event
}

func (m *MockPublisher) Publish(evt event.Event) {
	m.Events = append(m.Events, evt)
}

func (m *MockPublisher) GetFileEvents() []*FileEvent {
	var fileEvents []*FileEvent
	for _, evt := range m.Events {
		if fe, ok := evt.(*FileEvent); ok {
			fileEvents = append(fileEvents, fe)
		}
	}

	return fileEvents
}

type TestModel struct {
	Avatar      string            `meta:"uploaded_file"`
	Attachments []string          `meta:"uploaded_file"`
	Documents   map[string]string `meta:"uploaded_file"`
	Content     string            `meta:"richtext"`
	Summary     string            `meta:"markdown"`
	Ignored     string
}

type TestModelWithPointers struct {
	Avatar  *string `meta:"uploaded_file"`
	Content *string `meta:"richtext"`
	Summary *string `meta:"markdown"`
}

func testModelType() reflect.Type {
	return reflect.TypeFor[TestModel]()
}

func findField(fields []metaField, index []int) *metaField {
	for _, field := range fields {
		if len(field.index) == len(index) {
			match := true
			for i := range index {
				if field.index[i] != index[i] {
					match = false

					break
				}
			}

			if match {
				return &field
			}
		}
	}

	return nil
}

// TestParseMetaFields tests parse meta fields functionality.
func TestParseMetaFields(t *testing.T) {
	t.Run("BasicParsing", func(t *testing.T) {
		fields := parseMetaFields(testModelType())

		assert.Len(t, fields, 5, "Should parse all 5 meta-tagged fields")

		avatarField := findField(fields, []int{0})
		require.NotNil(t, avatarField, "Avatar field should be found")
		assert.Equal(t, MetaTypeUploadedFile, avatarField.typ, "Avatar should be uploaded_file type")
		assert.Equal(t, fieldKindScalar, avatarField.kind, "Avatar should be scalar kind")

		attachmentsField := findField(fields, []int{1})
		require.NotNil(t, attachmentsField, "Attachments field should be found")
		assert.Equal(t, MetaTypeUploadedFile, attachmentsField.typ, "Attachments should be uploaded_file type")
		assert.Equal(t, fieldKindSlice, attachmentsField.kind, "Attachments should be slice kind")

		documentsField := findField(fields, []int{2})
		require.NotNil(t, documentsField, "Documents field should be found")
		assert.Equal(t, MetaTypeUploadedFile, documentsField.typ, "Documents should be uploaded_file type")
		assert.Equal(t, fieldKindMap, documentsField.kind, "Documents should be map kind")

		contentField := findField(fields, []int{3})
		require.NotNil(t, contentField, "Content field should be found")
		assert.Equal(t, MetaTypeRichText, contentField.typ, "Content should be richtext type")
		assert.Equal(t, fieldKindScalar, contentField.kind, "Content should be scalar kind")

		summaryField := findField(fields, []int{4})
		require.NotNil(t, summaryField, "Summary field should be found")
		assert.Equal(t, MetaTypeMarkdown, summaryField.typ, "Summary should be markdown type")
		assert.Equal(t, fieldKindScalar, summaryField.kind, "Summary should be scalar kind")
	})

	t.Run("WithAttrs", func(t *testing.T) {
		type ModelWithAttrs struct {
			Avatar  string   `meta:"uploaded_file=category:avatar public:true"`
			Gallery []string `meta:"uploaded_file=category:gallery"`
			Content string   `meta:"richtext=sanitize:true max_size:10MB"`
		}

		fields := parseMetaFields(reflect.TypeFor[ModelWithAttrs]())

		require.Len(t, fields, 3, "Should parse all 3 fields with attributes")

		avatarField := findField(fields, []int{0})
		require.NotNil(t, avatarField, "Avatar field should be found")
		assert.Equal(t, MetaTypeUploadedFile, avatarField.typ, "Avatar should be uploaded_file type")
		assert.Equal(t, fieldKindScalar, avatarField.kind, "Avatar should be scalar kind")
		assert.Equal(t, map[string]string{"category": "avatar", "public": "true"}, avatarField.attrs,
			"Avatar attrs should be parsed correctly")

		galleryField := findField(fields, []int{1})
		require.NotNil(t, galleryField, "Gallery field should be found")
		assert.Equal(t, MetaTypeUploadedFile, galleryField.typ, "Gallery should be uploaded_file type")
		assert.Equal(t, fieldKindSlice, galleryField.kind, "Gallery should be slice kind")
		assert.Equal(t, map[string]string{"category": "gallery"}, galleryField.attrs,
			"Gallery attrs should be parsed correctly")

		contentField := findField(fields, []int{2})
		require.NotNil(t, contentField, "Content field should be found")
		assert.Equal(t, MetaTypeRichText, contentField.typ, "Content should be richtext type")
		assert.Equal(t, fieldKindScalar, contentField.kind, "Content should be scalar kind")
		assert.Equal(t, map[string]string{"sanitize": "true", "max_size": "10MB"}, contentField.attrs,
			"Content attrs should be parsed correctly")
	})

	t.Run("NoAttrs", func(t *testing.T) {
		type SimpleMeta struct {
			Avatar string `meta:"uploaded_file"`
		}

		fields := parseMetaFields(reflect.TypeFor[SimpleMeta]())

		require.Len(t, fields, 1, "Should parse single field without attrs")
		assert.Empty(t, fields[0].attrs, "Attrs should be empty when not specified")
	})

	t.Run("NonStructType", func(t *testing.T) {
		fields := parseMetaFields(reflect.TypeFor[string]())
		assert.Nil(t, fields, "Should return nil for non-struct types")
	})

	t.Run("InvalidMetaTag", func(t *testing.T) {
		type InvalidModel struct {
			Field1 string `meta:"invalid_type"`
			Field2 string `meta:"uploaded_file"`
		}

		fields := parseMetaFields(reflect.TypeFor[InvalidModel]())

		assert.Len(t, fields, 1, "Should skip invalid meta types")
		assert.Equal(t, MetaTypeUploadedFile, fields[0].typ,
			"Should only parse valid uploaded_file field")
	})

	t.Run("InvalidFieldTypes", func(t *testing.T) {
		type InvalidFieldTypesModel struct {
			IntField       int               `meta:"uploaded_file"`
			ArrayRichtext  []string          `meta:"richtext"`
			ArrayMarkdown  []string          `meta:"markdown"`
			MapRichtext    map[string]string `meta:"richtext"`
			MapMarkdown    map[string]string `meta:"markdown"`
			ValidField     string            `meta:"uploaded_file"`
			ValidArrayFile []string          `meta:"uploaded_file"`
			ValidMapFile   map[string]string `meta:"uploaded_file"`
		}

		fields := parseMetaFields(reflect.TypeFor[InvalidFieldTypesModel]())

		assert.Len(t, fields, 3, "Should only parse valid field type combinations")

		validField := findField(fields, []int{5})
		require.NotNil(t, validField, "Valid string field should be found")
		assert.Equal(t, MetaTypeUploadedFile, validField.typ, "Should be uploaded_file type")
		assert.Equal(t, fieldKindScalar, validField.kind, "Should be scalar kind")

		validArrayField := findField(fields, []int{6})
		require.NotNil(t, validArrayField, "Valid array field should be found")
		assert.Equal(t, MetaTypeUploadedFile, validArrayField.typ, "Should be uploaded_file type")
		assert.Equal(t, fieldKindSlice, validArrayField.kind, "Should be slice kind")

		validMapField := findField(fields, []int{7})
		require.NotNil(t, validMapField, "Valid map field should be found")
		assert.Equal(t, MetaTypeUploadedFile, validMapField.typ, "Should be uploaded_file type")
		assert.Equal(t, fieldKindMap, validMapField.kind, "Should be map kind")
	})

	t.Run("MultipleMetaTypes", func(t *testing.T) {
		type InvalidModel struct {
			Field string `meta:"uploaded_file,richtext"`
		}

		fields := parseMetaFields(reflect.TypeFor[InvalidModel]())

		assert.Len(t, fields, 1, "Should parse field with multiple meta types")
		assert.True(t, fields[0].typ == MetaTypeUploadedFile || fields[0].typ == MetaTypeRichText,
			"Should use first found meta type (map iteration order is random)")
	})

	t.Run("MapType", func(t *testing.T) {
		type ModelWithMap struct {
			Documents map[string]string `meta:"uploaded_file=category:idcard public:false"`
		}

		fields := parseMetaFields(reflect.TypeFor[ModelWithMap]())

		require.Len(t, fields, 1, "Should parse the map field")

		documentsField := findField(fields, []int{0})
		require.NotNil(t, documentsField, "Documents field should be found")
		assert.Equal(t, MetaTypeUploadedFile, documentsField.typ, "Documents should be uploaded_file type")
		assert.Equal(t, fieldKindMap, documentsField.kind, "Documents should be map kind")
		assert.Equal(t, map[string]string{"category": "idcard", "public": "false"}, documentsField.attrs,
			"Documents attrs should be parsed correctly")
	})
}

// TestPromote tests promote functionality.
func TestPromote(t *testing.T) {
	t.Run("CreateUploadedFile", func(t *testing.T) {
		t.Log("Testing file promotion for uploaded_file fields")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		model := &TestModel{
			Avatar:      "temp/2025/01/15/avatar.jpg",
			Attachments: []string{"temp/2025/01/15/doc1.pdf", "temp/2025/01/15/doc2.pdf"},
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Promotion should succeed")

		assert.Equal(t, "2025/01/15/avatar.jpg", model.Avatar,
			"Avatar should be promoted to permanent path")
		assert.Equal(t, []string{"2025/01/15/doc1.pdf", "2025/01/15/doc2.pdf"}, model.Attachments,
			"Attachments should be promoted to permanent paths")

		assert.True(t, service.files["2025/01/15/avatar.jpg"],
			"Avatar file should exist in storage")
		assert.True(t, service.files["2025/01/15/doc1.pdf"],
			"First attachment should exist in storage")
		assert.True(t, service.files["2025/01/15/doc2.pdf"],
			"Second attachment should exist in storage")
	})

	t.Run("CreateRichText", func(t *testing.T) {
		t.Log("Testing URL promotion in richtext fields")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		model := &TestModel{
			Content: `<img src="temp/2025/01/15/pic1.jpg"> <a href="temp/2025/01/15/doc.pdf">Download</a>`,
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Promotion should succeed")

		assert.Contains(t, model.Content, `src="2025/01/15/pic1.jpg"`,
			"Image src should be promoted to permanent path")
		assert.Contains(t, model.Content, `href="2025/01/15/doc.pdf"`,
			"Link href should be promoted to permanent path")

		assert.True(t, service.files["2025/01/15/pic1.jpg"],
			"Image file should exist in storage")
		assert.True(t, service.files["2025/01/15/doc.pdf"],
			"Document file should exist in storage")
	})

	t.Run("CreateMarkdown", func(t *testing.T) {
		t.Log("Testing URL promotion in markdown fields")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		model := &TestModel{
			Summary: `![Image](temp/2025/01/15/pic.jpg) [Document](temp/2025/01/15/doc.pdf)`,
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Promotion should succeed")

		assert.Contains(t, model.Summary, `](2025/01/15/pic.jpg)`,
			"Markdown image URL should be promoted")
		assert.Contains(t, model.Summary, `](2025/01/15/doc.pdf)`,
			"Markdown link URL should be promoted")

		assert.True(t, service.files["2025/01/15/pic.jpg"],
			"Image file should exist in storage")
		assert.True(t, service.files["2025/01/15/doc.pdf"],
			"Document file should exist in storage")
	})

	t.Run("UpdateReplaceFiles", func(t *testing.T) {
		t.Log("Testing file replacement during update")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		service.files["2025/01/15/old_avatar.jpg"] = true
		service.files["2025/01/15/old_doc1.pdf"] = true

		oldModel := &TestModel{
			Avatar:      "2025/01/15/old_avatar.jpg",
			Attachments: []string{"2025/01/15/old_doc1.pdf"},
		}

		newModel := &TestModel{
			Avatar:      "temp/2025/01/16/new_avatar.jpg",
			Attachments: []string{"temp/2025/01/16/new_doc1.pdf", "temp/2025/01/16/new_doc2.pdf"},
		}

		err := promoter.Promote(context.Background(), newModel, oldModel)
		require.NoError(t, err, "Update with file replacement should succeed")

		assert.Equal(t, "2025/01/16/new_avatar.jpg", newModel.Avatar,
			"New avatar should be promoted")
		assert.Equal(t, []string{"2025/01/16/new_doc1.pdf", "2025/01/16/new_doc2.pdf"}, newModel.Attachments,
			"New attachments should be promoted")

		assert.True(t, service.files["2025/01/16/new_avatar.jpg"],
			"New avatar should exist in storage")
		assert.True(t, service.files["2025/01/16/new_doc1.pdf"],
			"New attachment 1 should exist in storage")
		assert.True(t, service.files["2025/01/16/new_doc2.pdf"],
			"New attachment 2 should exist in storage")

		assert.False(t, service.files["2025/01/15/old_avatar.jpg"],
			"Old avatar should be deleted from storage")
		assert.False(t, service.files["2025/01/15/old_doc1.pdf"],
			"Old attachment should be deleted from storage")
	})

	t.Run("UpdatePartialChange", func(t *testing.T) {
		t.Log("Testing partial file updates")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		service.files["2025/01/15/avatar.jpg"] = true
		service.files["2025/01/15/doc1.pdf"] = true
		service.files["2025/01/15/doc2.pdf"] = true

		oldModel := &TestModel{
			Avatar:      "2025/01/15/avatar.jpg",
			Attachments: []string{"2025/01/15/doc1.pdf", "2025/01/15/doc2.pdf"},
		}

		newModel := &TestModel{
			Avatar:      "2025/01/15/avatar.jpg",
			Attachments: []string{"2025/01/15/doc1.pdf", "temp/2025/01/16/doc3.pdf"},
		}

		err := promoter.Promote(context.Background(), newModel, oldModel)
		require.NoError(t, err, "Partial update should succeed")

		assert.Equal(t, "2025/01/15/avatar.jpg", newModel.Avatar,
			"Unchanged avatar should remain the same")
		assert.True(t, service.files["2025/01/15/avatar.jpg"],
			"Unchanged avatar should still exist in storage")

		assert.True(t, service.files["2025/01/15/doc1.pdf"],
			"Retained attachment should still exist in storage")

		assert.False(t, service.files["2025/01/15/doc2.pdf"],
			"Removed attachment should be deleted from storage")

		assert.Equal(t, []string{"2025/01/15/doc1.pdf", "2025/01/16/doc3.pdf"}, newModel.Attachments,
			"Attachments should contain retained and new files")
		assert.True(t, service.files["2025/01/16/doc3.pdf"],
			"New attachment should exist in storage")
	})

	t.Run("UpdateRichtextUrlChange", func(t *testing.T) {
		t.Log("Testing richtext URL updates")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		service.files["2025/01/15/old_pic.jpg"] = true

		oldModel := &TestModel{
			Content: `<img src="2025/01/15/old_pic.jpg">`,
		}

		newModel := &TestModel{
			Content: `<img src="temp/2025/01/16/new_pic.jpg">`,
		}

		err := promoter.Promote(context.Background(), newModel, oldModel)
		require.NoError(t, err, "Richtext URL update should succeed")

		assert.Contains(t, newModel.Content, `src="2025/01/16/new_pic.jpg"`,
			"New image URL should be promoted in richtext")

		assert.True(t, service.files["2025/01/16/new_pic.jpg"],
			"New image should exist in storage")

		assert.False(t, service.files["2025/01/15/old_pic.jpg"],
			"Old image should be deleted from storage")
	})

	t.Run("Delete", func(t *testing.T) {
		t.Log("Testing file deletion when model is deleted")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		service.files["2025/01/15/avatar.jpg"] = true
		service.files["2025/01/15/doc1.pdf"] = true
		service.files["2025/01/15/doc2.pdf"] = true

		oldModel := &TestModel{
			Avatar:      "2025/01/15/avatar.jpg",
			Attachments: []string{"2025/01/15/doc1.pdf", "2025/01/15/doc2.pdf"},
		}

		err := promoter.Promote(context.Background(), nil, oldModel)
		require.NoError(t, err, "Delete operation should succeed")

		assert.False(t, service.files["2025/01/15/avatar.jpg"],
			"Avatar should be deleted from storage")
		assert.False(t, service.files["2025/01/15/doc1.pdf"],
			"Attachment 1 should be deleted from storage")
		assert.False(t, service.files["2025/01/15/doc2.pdf"],
			"Attachment 2 should be deleted from storage")
	})

	t.Run("NonTempFiles", func(t *testing.T) {
		t.Log("Testing that non-temp files are not promoted")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		model := &TestModel{
			Avatar:      "2025/01/15/avatar.jpg",
			Attachments: []string{"2025/01/15/doc1.pdf"},
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Non-temp file handling should succeed")

		assert.Equal(t, "2025/01/15/avatar.jpg", model.Avatar,
			"Non-temp avatar should remain unchanged")
		assert.Equal(t, []string{"2025/01/15/doc1.pdf"}, model.Attachments,
			"Non-temp attachments should remain unchanged")
	})

	t.Run("EmptyFields", func(t *testing.T) {
		t.Log("Testing handling of empty fields")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		model := &TestModel{
			Avatar:      "",
			Attachments: []string{},
			Content:     "",
			Summary:     "",
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Empty fields should be handled gracefully")

		assert.Empty(t, model.Avatar, "Empty avatar should remain empty")
		assert.Empty(t, model.Attachments, "Empty attachments should remain empty")
		assert.Empty(t, model.Content, "Empty content should remain empty")
		assert.Empty(t, model.Summary, "Empty summary should remain empty")
	})

	t.Run("ArrayWithEmptyStrings", func(t *testing.T) {
		t.Log("Testing array with empty string elements")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		model := &TestModel{
			Attachments: []string{"temp/2025/01/15/doc1.pdf", "", "  ", "temp/2025/01/15/doc2.pdf"},
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Array with empty strings should be cleaned")

		assert.Equal(t, []string{"2025/01/15/doc1.pdf", "2025/01/15/doc2.pdf"}, model.Attachments,
			"Empty strings should be filtered out from array")
		assert.True(t, service.files["2025/01/15/doc1.pdf"],
			"Valid attachment 1 should exist in storage")
		assert.True(t, service.files["2025/01/15/doc2.pdf"],
			"Valid attachment 2 should exist in storage")
	})

	t.Run("MixedTempAndPermanentUrls", func(t *testing.T) {
		t.Log("Testing mixed temp and permanent URLs in content")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		model := &TestModel{
			Content: `<img src="temp/2025/01/15/temp.jpg"> <img src="2025/01/10/permanent.jpg">`,
			Summary: `![Temp](temp/2025/01/15/temp.jpg) ![Permanent](2025/01/10/permanent.jpg)`,
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Mixed temp/permanent URLs should be handled correctly")

		assert.Contains(t, model.Content, `src="2025/01/15/temp.jpg"`,
			"Temp URL should be promoted in richtext")
		assert.Contains(t, model.Content, `src="2025/01/10/permanent.jpg"`,
			"Permanent URL should remain unchanged in richtext")
		assert.Contains(t, model.Summary, `](2025/01/15/temp.jpg)`,
			"Temp URL should be promoted in markdown")
		assert.Contains(t, model.Summary, `](2025/01/10/permanent.jpg)`,
			"Permanent URL should remain unchanged in markdown")

		assert.True(t, service.files["2025/01/15/temp.jpg"],
			"Temp file should be promoted to storage")
		assert.False(t, service.files["2025/01/10/permanent.jpg"],
			"Permanent file should not be touched")
	})

	t.Run("ContentWithoutUrls", func(t *testing.T) {
		t.Log("Testing content without any URLs")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		model := &TestModel{
			Content: `<p>This is plain text without any URLs</p>`,
			Summary: `This is plain markdown without any links`,
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Content without URLs should be handled")

		assert.Equal(t, `<p>This is plain text without any URLs</p>`, model.Content,
			"Richtext without URLs should remain unchanged")
		assert.Equal(t, `This is plain markdown without any links`, model.Summary,
			"Markdown without URLs should remain unchanged")
	})

	t.Run("BothModelsNil", func(t *testing.T) {
		t.Log("Testing nil model handling")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		err := promoter.Promote(context.Background(), nil, nil)
		require.NoError(t, err, "Nil models should be handled gracefully")

		assert.Empty(t, service.files, "No files should be affected when both models are nil")
	})

	t.Run("CreateUploadedFileMap", func(t *testing.T) {
		t.Log("Testing file promotion for map[string]string uploaded_file fields")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		model := &TestModel{
			Documents: map[string]string{
				"front": "temp/2025/01/15/idcard_front.jpg",
				"back":  "temp/2025/01/15/idcard_back.jpg",
			},
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Map promotion should succeed")

		assert.Equal(t, map[string]string{
			"front": "2025/01/15/idcard_front.jpg",
			"back":  "2025/01/15/idcard_back.jpg",
		}, model.Documents, "Map values should be promoted with keys preserved")

		assert.True(t, service.files["2025/01/15/idcard_front.jpg"],
			"Front file should exist in storage")
		assert.True(t, service.files["2025/01/15/idcard_back.jpg"],
			"Back file should exist in storage")
	})

	t.Run("UpdateMapReplaceFiles", func(t *testing.T) {
		t.Log("Testing map value replacement during update")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		service.files["2025/01/15/old_front.jpg"] = true
		service.files["2025/01/15/old_back.jpg"] = true

		oldModel := &TestModel{
			Documents: map[string]string{
				"front": "2025/01/15/old_front.jpg",
				"back":  "2025/01/15/old_back.jpg",
			},
		}

		newModel := &TestModel{
			Documents: map[string]string{
				"front": "temp/2025/01/16/new_front.jpg",
				"back":  "temp/2025/01/16/new_back.jpg",
			},
		}

		err := promoter.Promote(context.Background(), newModel, oldModel)
		require.NoError(t, err, "Map update should succeed")

		assert.Equal(t, map[string]string{
			"front": "2025/01/16/new_front.jpg",
			"back":  "2025/01/16/new_back.jpg",
		}, newModel.Documents, "New map values should be promoted")

		assert.True(t, service.files["2025/01/16/new_front.jpg"], "New front should exist")
		assert.True(t, service.files["2025/01/16/new_back.jpg"], "New back should exist")
		assert.False(t, service.files["2025/01/15/old_front.jpg"], "Old front should be deleted")
		assert.False(t, service.files["2025/01/15/old_back.jpg"], "Old back should be deleted")
	})

	t.Run("UpdateMapPartialChange", func(t *testing.T) {
		t.Log("Testing partial map updates")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		service.files["2025/01/15/front.jpg"] = true
		service.files["2025/01/15/back.jpg"] = true

		oldModel := &TestModel{
			Documents: map[string]string{
				"front": "2025/01/15/front.jpg",
				"back":  "2025/01/15/back.jpg",
			},
		}

		newModel := &TestModel{
			Documents: map[string]string{
				"front": "2025/01/15/front.jpg",
				"back":  "temp/2025/01/16/new_back.jpg",
			},
		}

		err := promoter.Promote(context.Background(), newModel, oldModel)
		require.NoError(t, err, "Partial map update should succeed")

		assert.Equal(t, map[string]string{
			"front": "2025/01/15/front.jpg",
			"back":  "2025/01/16/new_back.jpg",
		}, newModel.Documents, "Unchanged value should remain, replaced value should be promoted")

		assert.True(t, service.files["2025/01/15/front.jpg"], "Unchanged front should remain")
		assert.True(t, service.files["2025/01/16/new_back.jpg"], "New back should exist")
		assert.False(t, service.files["2025/01/15/back.jpg"], "Old back should be deleted")
	})

	t.Run("DeleteMap", func(t *testing.T) {
		t.Log("Testing deletion of all map values when model is deleted")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		service.files["2025/01/15/front.jpg"] = true
		service.files["2025/01/15/back.jpg"] = true

		oldModel := &TestModel{
			Documents: map[string]string{
				"front": "2025/01/15/front.jpg",
				"back":  "2025/01/15/back.jpg",
			},
		}

		err := promoter.Promote(context.Background(), nil, oldModel)
		require.NoError(t, err, "Delete with map field should succeed")

		assert.False(t, service.files["2025/01/15/front.jpg"], "Front should be deleted")
		assert.False(t, service.files["2025/01/15/back.jpg"], "Back should be deleted")
	})

	t.Run("MapWithEmptyValues", func(t *testing.T) {
		t.Log("Testing map with empty/whitespace values")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		model := &TestModel{
			Documents: map[string]string{
				"front": "temp/2025/01/15/front.jpg",
				"back":  "",
				"side":  "  ",
			},
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Map with empty values should be cleaned")

		assert.Equal(t, map[string]string{
			"front": "2025/01/15/front.jpg",
		}, model.Documents, "Only keys with non-empty values should remain")

		assert.True(t, service.files["2025/01/15/front.jpg"], "Front should be promoted")
	})

	t.Run("NilMap", func(t *testing.T) {
		t.Log("Testing nil map handling")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		model := &TestModel{
			Documents: nil,
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Nil map should be handled gracefully")

		assert.Nil(t, model.Documents, "Nil map should remain nil")
	})
}

// TestPromoterTypes tests promoter types functionality.
func TestPromoterTypes(t *testing.T) {
	t.Run("PointerTypesCreate", func(t *testing.T) {
		t.Log("Testing file promotion with pointer field types")

		service := NewMockService()
		promoter := NewPromoter[TestModelWithPointers](service)

		avatar := "temp/2025/01/15/avatar.jpg"
		content := `<img src="temp/2025/01/15/pic.jpg">`

		model := TestModelWithPointers{
			Avatar:  &avatar,
			Content: &content,
		}

		err := promoter.Promote(context.Background(), &model, nil)
		require.NoError(t, err, "Promotion with pointer types should succeed")

		assert.Equal(t, "2025/01/15/avatar.jpg", *model.Avatar,
			"Pointer avatar should be promoted")
		assert.Contains(t, *model.Content, `src="2025/01/15/pic.jpg"`,
			"Pointer content URL should be promoted")
	})

	t.Run("PointerTypesUpdate", func(t *testing.T) {
		t.Log("Testing file updates with pointer field types")

		service := NewMockService()
		promoter := NewPromoter[TestModelWithPointers](service)

		service.files["2025/01/15/old_avatar.jpg"] = true

		oldAvatar := "2025/01/15/old_avatar.jpg"
		oldModel := TestModelWithPointers{
			Avatar: &oldAvatar,
		}

		newAvatar := "temp/2025/01/16/new_avatar.jpg"
		newModel := TestModelWithPointers{
			Avatar: &newAvatar,
		}

		err := promoter.Promote(context.Background(), &newModel, &oldModel)
		require.NoError(t, err, "Update with pointer types should succeed")

		assert.Equal(t, "2025/01/16/new_avatar.jpg", *newModel.Avatar,
			"New pointer avatar should be promoted")
		assert.True(t, service.files["2025/01/16/new_avatar.jpg"],
			"New avatar file should exist in storage")

		assert.False(t, service.files["2025/01/15/old_avatar.jpg"],
			"Old avatar file should be deleted")
	})

	t.Run("PointerTypesNilPointers", func(t *testing.T) {
		t.Log("Testing nil pointer handling")

		service := NewMockService()
		promoter := NewPromoter[TestModelWithPointers](service)

		model := TestModelWithPointers{
			Avatar:  nil,
			Content: nil,
		}

		err := promoter.Promote(context.Background(), &model, nil)
		require.NoError(t, err, "Nil pointers should be handled gracefully")

		assert.Nil(t, model.Avatar, "Nil avatar pointer should remain nil")
		assert.Nil(t, model.Content, "Nil content pointer should remain nil")
	})
}

// TestPromoterEvents tests promoter events functionality.
func TestPromoterEvents(t *testing.T) {
	t.Run("WithEventPublisher", func(t *testing.T) {
		t.Log("Testing event publishing during promotion")

		service := NewMockService()
		publisher := &MockPublisher{}
		promoter := NewPromoter[TestModel](service, publisher)

		model := &TestModel{
			Avatar:      "temp/2025/01/15/avatar.jpg",
			Attachments: []string{"temp/2025/01/15/doc1.pdf", "temp/2025/01/15/doc2.pdf"},
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Promotion with event publisher should succeed")

		assert.Equal(t, "2025/01/15/avatar.jpg", model.Avatar,
			"Avatar should be promoted")
		assert.Equal(t, []string{"2025/01/15/doc1.pdf", "2025/01/15/doc2.pdf"}, model.Attachments,
			"Attachments should be promoted")

		events := publisher.GetFileEvents()
		assert.Len(t, events, 3, "Should publish 3 promotion events")

		for _, evt := range events {
			assert.Equal(t, OperationPromote, evt.Operation,
				"Event operation should be promote")
			assert.Equal(t, MetaTypeUploadedFile, evt.MetaType,
				"Event meta type should be uploaded_file")
			assert.NotEmpty(t, evt.FileKey,
				"Event file key should not be empty")
			assert.False(t, strings.HasPrefix(evt.FileKey, TempPrefix),
				"Event file key should not have temp prefix")
		}
	})

	t.Run("WithoutEventPublisher", func(t *testing.T) {
		t.Log("Testing promotion without event publisher")

		service := NewMockService()
		promoter := NewPromoter[TestModel](service)

		model := &TestModel{
			Avatar: "temp/2025/01/15/avatar.jpg",
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Promotion without publisher should succeed")

		assert.Equal(t, "2025/01/15/avatar.jpg", model.Avatar,
			"Avatar should be promoted even without publisher")
	})

	t.Run("DeleteEvents", func(t *testing.T) {
		t.Log("Testing delete event publishing")

		service := NewMockService()
		publisher := &MockPublisher{}
		promoter := NewPromoter[TestModel](service, publisher)

		service.files["2025/01/15/avatar.jpg"] = true
		service.files["2025/01/15/doc1.pdf"] = true

		oldModel := &TestModel{
			Avatar:      "2025/01/15/avatar.jpg",
			Attachments: []string{"2025/01/15/doc1.pdf"},
		}

		err := promoter.Promote(context.Background(), nil, oldModel)
		require.NoError(t, err, "Delete with event publisher should succeed")

		events := publisher.GetFileEvents()
		assert.Len(t, events, 2, "Should publish 2 deletion events")

		for _, evt := range events {
			assert.Equal(t, OperationDelete, evt.Operation,
				"Event operation should be delete")
			assert.Equal(t, MetaTypeUploadedFile, evt.MetaType,
				"Event meta type should be uploaded_file")
			assert.NotEmpty(t, evt.FileKey,
				"Event file key should not be empty")
		}

		assert.False(t, service.files["2025/01/15/avatar.jpg"],
			"Avatar should be deleted")
		assert.False(t, service.files["2025/01/15/doc1.pdf"],
			"Attachment should be deleted")
	})

	t.Run("CleanupEvents", func(t *testing.T) {
		t.Log("Testing cleanup event publishing during updates")

		service := NewMockService()
		publisher := &MockPublisher{}
		promoter := NewPromoter[TestModel](service, publisher)

		service.files["2025/01/15/old_avatar.jpg"] = true
		service.files["2025/01/15/old_doc1.pdf"] = true

		oldModel := &TestModel{
			Avatar:      "2025/01/15/old_avatar.jpg",
			Attachments: []string{"2025/01/15/old_doc1.pdf"},
		}

		newModel := &TestModel{
			Avatar:      "temp/2025/01/16/new_avatar.jpg",
			Attachments: []string{"temp/2025/01/16/new_doc1.pdf"},
		}

		err := promoter.Promote(context.Background(), newModel, oldModel)
		require.NoError(t, err, "Update with cleanup should succeed")

		events := publisher.GetFileEvents()

		promoteEvents := 0

		deleteEvents := 0
		for _, evt := range events {
			switch evt.Operation {
			case OperationPromote:
				promoteEvents++
			case OperationDelete:
				deleteEvents++
			}
		}

		assert.Equal(t, 2, promoteEvents, "Should publish 2 promotion events")
		assert.Equal(t, 2, deleteEvents, "Should publish 2 deletion events")

		assert.False(t, service.files["2025/01/15/old_avatar.jpg"],
			"Old avatar should be deleted")
		assert.False(t, service.files["2025/01/15/old_doc1.pdf"],
			"Old attachment should be deleted")
		assert.True(t, service.files["2025/01/16/new_avatar.jpg"],
			"New avatar should exist")
		assert.True(t, service.files["2025/01/16/new_doc1.pdf"],
			"New attachment should exist")
	})

	t.Run("RichtextWithAttrs", func(t *testing.T) {
		t.Log("Testing event publishing for richtext with attributes")

		type ModelWithRichtext struct {
			Content string `meta:"richtext=sanitize:true max_size:10MB"`
		}

		service := NewMockService()
		publisher := &MockPublisher{}
		promoter := NewPromoter[ModelWithRichtext](service, publisher)

		model := &ModelWithRichtext{
			Content: `<img src="temp/2025/01/15/pic.jpg">`,
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Richtext promotion with attrs should succeed")

		events := publisher.GetFileEvents()
		assert.Len(t, events, 1, "Should publish 1 promotion event")

		evt := events[0]
		assert.Equal(t, OperationPromote, evt.Operation,
			"Event operation should be promote")
		assert.Equal(t, MetaTypeRichText, evt.MetaType,
			"Event meta type should be richtext")
		assert.Equal(t, "2025/01/15/pic.jpg", evt.FileKey,
			"Event file key should be the promoted path")
		assert.Equal(t, map[string]string{"sanitize": "true", "max_size": "10MB"}, evt.Attrs,
			"Event attrs should match field attrs")
	})

	t.Run("MapPromoteEvents", func(t *testing.T) {
		t.Log("Testing event publishing for map[string]string promotion")

		service := NewMockService()
		publisher := &MockPublisher{}
		promoter := NewPromoter[TestModel](service, publisher)

		model := &TestModel{
			Documents: map[string]string{
				"front": "temp/2025/01/15/front.jpg",
				"back":  "temp/2025/01/15/back.jpg",
			},
		}

		err := promoter.Promote(context.Background(), model, nil)
		require.NoError(t, err, "Map promotion with publisher should succeed")

		events := publisher.GetFileEvents()
		assert.Len(t, events, 2, "Should publish one promote event per map entry")

		promotedKeys := make(map[string]bool, len(events))
		for _, evt := range events {
			assert.Equal(t, OperationPromote, evt.Operation,
				"Event operation should be promote")
			assert.Equal(t, MetaTypeUploadedFile, evt.MetaType,
				"Event meta type should be uploaded_file")
			assert.NotEmpty(t, evt.FileKey, "Event file key should not be empty")
			assert.False(t, strings.HasPrefix(evt.FileKey, TempPrefix),
				"Event file key should not have temp prefix")
			promotedKeys[evt.FileKey] = true
		}

		assert.True(t, promotedKeys["2025/01/15/front.jpg"], "Front event should be published")
		assert.True(t, promotedKeys["2025/01/15/back.jpg"], "Back event should be published")
	})
}

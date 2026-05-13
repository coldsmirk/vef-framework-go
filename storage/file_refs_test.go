package storage

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/reflectx"
)

// extractRefs is a test-only white-box helper that mirrors the
// behavior of the now-deleted FileRefExtractor[T]: it parses T's
// `meta` tags once and returns every reachable FileRef. Production
// code never needs this — it goes through Files (files.go), which
// composes parseMetaFields + collectFileRefs through the URLKeyMapper.
func extractRefs[T any](model *T) []FileRef {
	if model == nil {
		return nil
	}

	typ := reflectx.Indirect(reflect.TypeFor[T]())
	fields := parseMetaFields(typ)

	value := reflect.Indirect(reflect.ValueOf(model))
	if value.Kind() != reflect.Struct {
		return nil
	}

	return collectFileRefs(value, fields)
}

// diffRefsFor is a test-only white-box helper that runs extractRefs on
// both snapshots and feeds the result through the production diffRefs
// partitioner. Used by the OnUpdate-style diff tests below.
func diffRefsFor[T any](newModel, oldModel *T) (toConsume, toDelete []FileRef) {
	return diffRefs(extractRefs(newModel), extractRefs(oldModel))
}

// ── Models for FileRefExtractor tests ───────────────────────────────────
//
// Each model isolates one feature of the meta-tag extraction surface so
// individual sub-tests can fail with a precise scope. Model names avoid
// the storage_test package's existing FileModel from files_test.go.

type UploadedScalarModel struct {
	Cover string `meta:"uploaded_file"`
}

type UploadedScalarPtrModel struct {
	Cover *string `meta:"uploaded_file"`
}

type UploadedSliceModel struct {
	Files []string `meta:"uploaded_file"`
}

type UploadedMapModel struct {
	FilesByName map[string]string `meta:"uploaded_file"`
}

type RichTextOnlyModel struct {
	Body string `meta:"richtext"`
}

type MarkdownOnlyModel struct {
	Body string `meta:"markdown"`
}

type WithAttrsModel struct {
	Cover string `meta:"uploaded_file=category:gallery public:true"`
}

type UnknownMetaModel struct {
	Note string `meta:"foo"`
}

type UnsupportedScalarModel struct {
	Count int `meta:"uploaded_file"`
}

type RichTextOnNonStringModel struct {
	Bodies []string `meta:"richtext"`
}

type MixedModel struct {
	Cover string `meta:"uploaded_file"`
	Body  string `meta:"richtext"`
	Notes string `meta:"markdown"`
}

type PlainModel struct {
	Title string
}

// ── Dive (nested struct) models ─────────────────────────────────────────
//
// `meta:"dive"` on a struct field instructs the extractor to recurse into
// the nested struct so its meta-tagged fields become reachable. Below
// covers: single-level dive, multi-level dive, dive into a struct that
// mixes all three meta types, and dive applied to fields whose underlying
// type is not a struct (which must be a no-op rather than a panic).

type DiveInner struct {
	Cover string `meta:"uploaded_file"`
}

type DiveOuter struct {
	Inner DiveInner `meta:"dive"`
}

type DivePtrOuter struct {
	Inner *DiveInner `meta:"dive"`
}

type DiveDeepest struct {
	Cover string `meta:"uploaded_file"`
}

type DiveMiddle struct {
	Deepest DiveDeepest `meta:"dive"`
}

type DiveMultiLevelOuter struct {
	Middle DiveMiddle `meta:"dive"`
}

type DiveMixedInner struct {
	Cover string `meta:"uploaded_file"`
	Body  string `meta:"richtext"`
	Notes string `meta:"markdown"`
}

type DiveMixedOuter struct {
	Inner DiveMixedInner `meta:"dive"`
}

// DiveNonStructModel applies `meta:"dive"` to a non-struct field. The
// extractor must not panic and must not produce any refs — the dive tag
// only triggers recursion on struct-shaped fields.
type DiveNonStructModel struct {
	Files string `meta:"dive"`
}

// DiveAlongsideUnknownModel mixes a sibling field with an unknown meta
// type and a dive field. The dive sibling must still be honored even
// when other siblings carry tags reflectx can't interpret.
type DiveAlongsideUnknownModel struct {
	Note  string    `meta:"foo"`
	Inner DiveInner `meta:"dive"`
}

func TestExtractRefs(t *testing.T) {
	t.Run("NonStructTypeYieldsNoRefs", func(t *testing.T) {
		// Calling extractRefs on a non-struct type must not panic and
		// must report no refs.
		v := 42
		assert.Empty(t, extractRefs(&v), "Non-struct T must yield no refs")
	})

	t.Run("StructWithoutMetaTagsYieldsNoRefs", func(t *testing.T) {
		assert.Empty(t, extractRefs(&PlainModel{Title: "hello"}), "Struct without any meta tags must yield no refs")
	})
}

func TestExtractRefsByMetaTag(t *testing.T) {
	t.Run("NilModelYieldsNil", func(t *testing.T) {
		assert.Nil(t, extractRefs[UploadedScalarModel](nil), "extractRefs(nil) must return nil, not an empty slice")
	})

	t.Run("UploadedFileScalarPopulated", func(t *testing.T) {
		refs := extractRefs(&UploadedScalarModel{Cover: "priv/cover.png"})

		require.Len(t, refs, 1, "One non-empty scalar field must produce one ref")
		assert.Equal(t, "priv/cover.png", refs[0].Key, "Ref key must match field value")
		assert.Equal(t, MetaTypeUploadedFile, refs[0].MetaType, "Ref meta type must reflect the uploaded_file tag")
	})

	t.Run("UploadedFileScalarEmptyStringSkipped", func(t *testing.T) {
		assert.Empty(t, extractRefs(&UploadedScalarModel{Cover: ""}), "Empty scalar field must yield no ref")
	})

	t.Run("UploadedFileScalarPtrNilSkipped", func(t *testing.T) {
		assert.Empty(t, extractRefs(&UploadedScalarPtrModel{Cover: nil}), "Nil *string field must yield no ref")
	})

	t.Run("UploadedFileScalarPtrPopulated", func(t *testing.T) {
		key := "priv/ptr.png"
		refs := extractRefs(&UploadedScalarPtrModel{Cover: &key})

		require.Len(t, refs, 1, "Non-nil *string field must produce one ref")
		assert.Equal(t, key, refs[0].Key, "Ref key must dereference the pointer value")
	})

	t.Run("UploadedFileSliceProducesOneRefPerEntry", func(t *testing.T) {
		refs := extractRefs(&UploadedSliceModel{Files: []string{"priv/a", "priv/b"}})

		keys := refKeys(refs)
		assert.ElementsMatch(t, []string{"priv/a", "priv/b"}, keys, "Each slice entry must produce one ref")
	})

	t.Run("UploadedFileSliceTrimsWhitespaceAndSkipsEmpty", func(t *testing.T) {
		refs := extractRefs(&UploadedSliceModel{Files: []string{"  priv/a  ", "", "   ", "priv/b"}})

		keys := refKeys(refs)
		assert.ElementsMatch(t, []string{"priv/a", "priv/b"}, keys, "Empty / whitespace-only entries must be skipped, surviving entries trimmed")
	})

	t.Run("UploadedFileSliceNilYieldsNoRefs", func(t *testing.T) {
		assert.Empty(t, extractRefs(&UploadedSliceModel{Files: nil}), "Nil slice must yield no refs")
	})

	t.Run("UploadedFileMapUsesValuesAsKeys", func(t *testing.T) {
		refs := extractRefs(&UploadedMapModel{FilesByName: map[string]string{
			"avatar": "priv/avatar.png",
			"cover":  "priv/cover.png",
		}})

		keys := refKeys(refs)
		assert.ElementsMatch(t, []string{"priv/avatar.png", "priv/cover.png"}, keys, "Map values (not keys) must become FileRef keys")
	})

	t.Run("UploadedFileMapTrimsAndSkipsEmptyValues", func(t *testing.T) {
		refs := extractRefs(&UploadedMapModel{FilesByName: map[string]string{
			"avatar": "  priv/avatar.png  ",
			"cover":  "",
			"empty":  "   ",
		}})

		keys := refKeys(refs)
		assert.ElementsMatch(t, []string{"priv/avatar.png"}, keys, "Empty / whitespace-only map values must be skipped, surviving values trimmed")
	})

	t.Run("RichTextExtractsRelativeImageSources", func(t *testing.T) {
		refs := extractRefs(&RichTextOnlyModel{
			Body: `<p>Look:</p><img src="priv/a.png"><img src='priv/b.png'>`,
		})

		keys := refKeys(refs)
		assert.ElementsMatch(t, []string{"priv/a.png", "priv/b.png"}, keys, "Relative img src URLs must be extracted regardless of quote style")

		for _, r := range refs {
			assert.Equal(t, MetaTypeRichText, r.MetaType, "Every richtext-derived ref must carry MetaTypeRichText")
		}
	})

	t.Run("RichTextExtractsAbsoluteAndRelativeURLs", func(t *testing.T) {
		// New contract: the extractor surfaces every URL it sees,
		// including http(s) ones. Scheme-based filtering moved to
		// URLKeyMapper, which Files invokes after extraction; the
		// extractor itself stays scheme-agnostic so business modules
		// with a custom mapper (e.g. recognizing a CDN host) can
		// resolve absolute URLs to managed keys.
		refs := extractRefs(&RichTextOnlyModel{
			Body: `<img src="https://cdn.example.com/abs.png"><img src="priv/rel.png">`,
		})

		assert.ElementsMatch(t,
			[]string{"https://cdn.example.com/abs.png", "priv/rel.png"},
			refKeys(refs),
			"Extractor must surface every URL; URLKeyMapper is responsible for deciding which ones are managed",
		)
	})

	t.Run("RichTextDeduplicatesIdenticalURLs", func(t *testing.T) {
		refs := extractRefs(&RichTextOnlyModel{
			Body: `<img src="priv/dup.png"><a href="priv/dup.png">link</a>`,
		})

		keys := refKeys(refs)
		assert.Equal(t, []string{"priv/dup.png"}, keys, "Duplicate URLs across multiple tags must produce a single ref")
	})

	t.Run("RichTextEmptyBodyYieldsNoRefs", func(t *testing.T) {
		assert.Empty(t, extractRefs(&RichTextOnlyModel{Body: ""}), "Empty richtext body must yield no refs")
	})

	t.Run("MarkdownExtractsImageAndLinkURLs", func(t *testing.T) {
		refs := extractRefs(&MarkdownOnlyModel{
			Body: `Image: ![alt](priv/img.png) and link [text](priv/doc.pdf).`,
		})

		keys := refKeys(refs)
		assert.ElementsMatch(t, []string{"priv/img.png", "priv/doc.pdf"}, keys, "Both ![](url) image and [text](url) link forms must be extracted")

		for _, r := range refs {
			assert.Equal(t, MetaTypeMarkdown, r.MetaType, "Every markdown-derived ref must carry MetaTypeMarkdown")
		}
	})

	t.Run("MarkdownStripsOptionalTitle", func(t *testing.T) {
		refs := extractRefs(&MarkdownOnlyModel{
			Body: `![alt](priv/img.png "title text")`,
		})

		keys := refKeys(refs)
		assert.Equal(t, []string{"priv/img.png"}, keys, `Markdown title syntax (url "title") must be stripped from the extracted URL`)
	})

	t.Run("MarkdownExtractsAbsoluteAndRelativeURLs", func(t *testing.T) {
		// Same scheme-agnostic contract as RichText: the markdown
		// extractor surfaces every URL; URLKeyMapper decides which
		// ones are managed objects.
		refs := extractRefs(&MarkdownOnlyModel{
			Body: `![](https://cdn.example.com/abs.png) and ![](priv/rel.png)`,
		})

		assert.ElementsMatch(t,
			[]string{"https://cdn.example.com/abs.png", "priv/rel.png"},
			refKeys(refs),
			"Markdown extractor must surface every URL; URLKeyMapper handles scheme decisions downstream",
		)
	})

	t.Run("AttrsParsedFromMetaTagValue", func(t *testing.T) {
		refs := extractRefs(&WithAttrsModel{Cover: "priv/cover.png"})

		require.Len(t, refs, 1, "Tagged field with value must yield one ref")
		assert.Equal(t, "gallery", refs[0].Attrs["category"], "category attr must be parsed from the meta tag")
		assert.Equal(t, "true", refs[0].Attrs["public"], "public attr must be parsed from the meta tag")
	})

	t.Run("AttrsEmptyWhenTagHasNoExtraAttributes", func(t *testing.T) {
		refs := extractRefs(&UploadedScalarModel{Cover: "priv/x.png"})

		require.Len(t, refs, 1, "Plain uploaded_file tag must still yield a ref")
		assert.Empty(t, refs[0].Attrs, "Plain meta tag (no value section) must yield empty Attrs")
	})

	t.Run("UnknownMetaTypeIgnored", func(t *testing.T) {
		// Tags whose key does not match any known MetaType should be
		// silently ignored — the framework refuses to invent semantics
		// for unrecognized meta categories.
		assert.Empty(t, extractRefs(&UnknownMetaModel{Note: "anything"}), "Unknown meta type must be silently dropped")
	})

	t.Run("UnsupportedScalarTypeIgnored", func(t *testing.T) {
		// uploaded_file only supports string-shaped fields; an int
		// tagged uploaded_file must be ignored rather than panic.
		assert.Empty(t, extractRefs(&UnsupportedScalarModel{Count: 42}), "uploaded_file on a non-string field must be silently dropped")
	})

	t.Run("RichTextOnNonStringIgnored", func(t *testing.T) {
		// richtext / markdown require a string field; a []string body
		// is a misuse and must be skipped.
		assert.Empty(t, extractRefs(&RichTextOnNonStringModel{Bodies: []string{"<img src=\"priv/x.png\">"}}), "richtext on a non-string field must be silently dropped")
	})

	t.Run("MixedModelExtractsAllThreeMetaTypes", func(t *testing.T) {
		refs := extractRefs(&MixedModel{
			Cover: "priv/cover.png",
			Body:  `<img src="priv/body.png">`,
			Notes: `![](priv/notes.png)`,
		})

		keys := refKeys(refs)
		assert.ElementsMatch(t, []string{"priv/cover.png", "priv/body.png", "priv/notes.png"}, keys, "All three meta types must be extracted in one Extract call")

		byKey := indexByKey(refs)
		assert.Equal(t, MetaTypeUploadedFile, byKey["priv/cover.png"].MetaType, "Cover ref must carry MetaTypeUploadedFile")
		assert.Equal(t, MetaTypeRichText, byKey["priv/body.png"].MetaType, "Body ref must carry MetaTypeRichText")
		assert.Equal(t, MetaTypeMarkdown, byKey["priv/notes.png"].MetaType, "Notes ref must carry MetaTypeMarkdown")
	})

	t.Run("DiveTagRecursesIntoNestedStruct", func(t *testing.T) {
		refs := extractRefs(&DiveOuter{Inner: DiveInner{Cover: "priv/inner.png"}})

		require.Len(t, refs, 1, "Single-level dive must surface the inner uploaded_file ref")
		assert.Equal(t, "priv/inner.png", refs[0].Key, "Nested ref key must match inner field value")
		assert.Equal(t, MetaTypeUploadedFile, refs[0].MetaType, "Nested ref must inherit its own meta type tag")
	})

	t.Run("DiveTagRecursesIntoPointerStruct", func(t *testing.T) {
		// reflectx.Indirect strips pointer types before checking Kind,
		// so `*DiveInner` with `meta:"dive"` must behave identically
		// to a value-typed struct field.
		refs := extractRefs(&DivePtrOuter{Inner: &DiveInner{Cover: "priv/ptr-inner.png"}})

		require.Len(t, refs, 1, "Dive on a *struct field must recurse through the pointer")
		assert.Equal(t, "priv/ptr-inner.png", refs[0].Key, "Nested ref key must match inner field value through pointer dive")
	})

	t.Run("DiveTagChainsAcrossMultipleLevels", func(t *testing.T) {
		// Each level wears `meta:"dive"`; the extractor must keep
		// recursing until it reaches a leaf field with a real meta
		// type, no matter how many dive hops separate them.
		refs := extractRefs(&DiveMultiLevelOuter{
			Middle: DiveMiddle{Deepest: DiveDeepest{Cover: "priv/deepest.png"}},
		})

		require.Len(t, refs, 1, "Multi-level dive must surface the deepest uploaded_file ref")
		assert.Equal(t, "priv/deepest.png", refs[0].Key, "Deepest nested ref key must propagate through every dive level")
	})

	t.Run("DiveTagSurfacesAllMetaTypesFromNestedStruct", func(t *testing.T) {
		refs := extractRefs(&DiveMixedOuter{Inner: DiveMixedInner{
			Cover: "priv/cover.png",
			Body:  `<img src="priv/body.png">`,
			Notes: `![](priv/notes.png)`,
		}})

		keys := refKeys(refs)
		assert.ElementsMatch(t, []string{"priv/cover.png", "priv/body.png", "priv/notes.png"}, keys, "Dive must surface every meta type defined inside the nested struct")

		byKey := indexByKey(refs)
		assert.Equal(t, MetaTypeUploadedFile, byKey["priv/cover.png"].MetaType, "Nested Cover ref must carry MetaTypeUploadedFile")
		assert.Equal(t, MetaTypeRichText, byKey["priv/body.png"].MetaType, "Nested Body ref must carry MetaTypeRichText")
		assert.Equal(t, MetaTypeMarkdown, byKey["priv/notes.png"].MetaType, "Nested Notes ref must carry MetaTypeMarkdown")
	})

	t.Run("DiveOnNonStructFieldIsSilentNoOp", func(t *testing.T) {
		// `meta:"dive"` only triggers recursion when the field's
		// underlying type is a struct (reflectx.shouldRecurse). On a
		// string field it must be a silent no-op — never a panic.
		assert.Empty(t, extractRefs(&DiveNonStructModel{Files: "priv/should-be-ignored.png"}), "Dive on a non-struct field must yield no refs")
	})

	t.Run("DiveSiblingHonoredAlongsideUnknownMetaSibling", func(t *testing.T) {
		// A field with an unrecognized meta type must not prevent
		// reflectx from honoring a dive tag on a sibling field.
		refs := extractRefs(&DiveAlongsideUnknownModel{
			Note:  "anything",
			Inner: DiveInner{Cover: "priv/sibling-dive.png"},
		})

		require.Len(t, refs, 1, "Unknown meta sibling must not block dive recursion on another sibling")
		assert.Equal(t, "priv/sibling-dive.png", refs[0].Key, "Dive-sibling ref key must match inner field value")
	})
}

func TestDiffRefsByMetaTag(t *testing.T) {
	t.Run("OnlyNewModelTreatsAllRefsAsConsume", func(t *testing.T) {
		toConsume, toDelete := diffRefsFor(
			&UploadedSliceModel{Files: []string{"priv/a", "priv/b"}},
			nil,
		)

		assert.ElementsMatch(t, []string{"priv/a", "priv/b"}, refKeys(toConsume), "All new refs must be marked toConsume when oldModel is nil (create path)")
		assert.Empty(t, toDelete, "No refs to delete when oldModel is nil")
	})

	t.Run("OnlyOldModelTreatsAllRefsAsDelete", func(t *testing.T) {
		toConsume, toDelete := diffRefsFor(
			nil,
			&UploadedSliceModel{Files: []string{"priv/a", "priv/b"}},
		)

		assert.Empty(t, toConsume, "No refs to consume when newModel is nil")
		assert.ElementsMatch(t, []string{"priv/a", "priv/b"}, refKeys(toDelete), "All old refs must be marked toDelete when newModel is nil (delete path)")
	})

	t.Run("BothNilYieldsEmptyPartitions", func(t *testing.T) {
		toConsume, toDelete := diffRefsFor[UploadedSliceModel](nil, nil)
		assert.Empty(t, toConsume, "Both nil must yield empty toConsume")
		assert.Empty(t, toDelete, "Both nil must yield empty toDelete")
	})

	t.Run("IdenticalSnapshotsYieldEmptyPartitions", func(t *testing.T) {
		model := &UploadedSliceModel{Files: []string{"priv/a", "priv/b"}}
		toConsume, toDelete := diffRefsFor(model, model)

		assert.Empty(t, toConsume, "Unchanged refs must not be re-consumed")
		assert.Empty(t, toDelete, "Unchanged refs must not be deleted")
	})

	t.Run("MixedDiffPartitionsByKeyMembership", func(t *testing.T) {
		// old: {a, b, c}
		// new: {b, c, d, e}
		// expected toConsume = {d, e} (in new but not old)
		// expected toDelete  = {a}    (in old but not new)
		toConsume, toDelete := diffRefsFor(
			&UploadedSliceModel{Files: []string{"priv/b", "priv/c", "priv/d", "priv/e"}},
			&UploadedSliceModel{Files: []string{"priv/a", "priv/b", "priv/c"}},
		)

		assert.ElementsMatch(t, []string{"priv/d", "priv/e"}, refKeys(toConsume), "Newly added keys must be in toConsume")
		assert.ElementsMatch(t, []string{"priv/a"}, refKeys(toDelete), "Removed keys must be in toDelete")
	})
}

// refKeys projects a slice of FileRef into its Key field for compact
// assertions like ElementsMatch.
func refKeys(refs []FileRef) []string {
	keys := make([]string, len(refs))
	for i, r := range refs {
		keys[i] = r.Key
	}

	return keys
}

// indexByKey lifts a flat ref slice into a Key→FileRef map so tests can
// assert per-ref properties (MetaType, Attrs) without depending on slice
// ordering.
func indexByKey(refs []FileRef) map[string]FileRef {
	out := make(map[string]FileRef, len(refs))
	for _, r := range refs {
		out[r.Key] = r
	}

	return out
}

package modelschema

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractStructTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		key  string
		want string
	}{
		{"PlainSingleKey", `bun:"name"`, "bun", "name"},
		{"WithBackticks", "`bun:\"name\"`", "bun", "name"},
		{"ValueWithSpaces", `label:"User Name" bun:"name"`, "label", "User Name"},
		{"MultipleKeys", `json:"id" bun:"id,pk"`, "bun", "id,pk"},
		{"MissingKey", `bun:"name"`, "json", ""},
		{"EmptyTag", "", "bun", ""},
		{"EscapedQuote", `label:"User \"Name\""`, "label", `User "Name"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractStructTag(tt.tag, tt.key)
			assert.Equal(t, tt.want, got, "Tag value should match")
		})
	}
}

func TestIsRelationFieldFromTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{"NoTag", "", false},
		{"BelongsTo", `bun:"rel:belongs-to,join:user_id=id"`, true},
		{"HasOne", `bun:"rel:has-one,join:id=user_id"`, true},
		{"HasMany", `bun:"rel:has-many,join:id=user_id"`, true},
		{"ManyToMany", `bun:"rel:many-to-many"`, true},
		{"NormalColumn", `bun:"name"`, false},
		{"ColumnContainingRelLiteral", `bun:"my_rel"`, false},
		{"M2MWithoutRelPrefix", `bun:"m2m:user_tags,join:User=Tag"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRelationFieldFromTag(tt.tag)
			assert.Equal(t, tt.want, got, "Relation detection should match")
		})
	}
}

func TestHasScanonlyTagFromTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{"NoTag", "", false},
		{"ScanonlyOnly", `bun:",scanonly"`, true},
		{"ScanonlyWithColumn", `bun:"col_name,scanonly"`, true},
		{"ScanonlyWithSpaces", `bun:" , scanonly "`, true},
		{"NoScanonly", `bun:"name,notnull"`, false},
		{"ScanonlySubstringNotMatch", `bun:"scanonlyish"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasScanonlyTagFromTag(tt.tag)
			assert.Equal(t, tt.want, got, "scanonly detection should match")
		})
	}
}

func TestExtractEmbedPrefixFromTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{"NoTag", "", ""},
		{"NoEmbed", `bun:"name"`, ""},
		{"EmbedAtStart", `bun:"embed:addr_"`, "addr_"},
		{"EmbedAfterFlag", `bun:",embed:addr_"`, "addr_"},
		{"EmbedAmongFlags", `bun:"name,notnull,embed:p_"`, "p_"},
		{"EmptyEmbedPrefix", `bun:",embed:"`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractEmbedPrefixFromTag(tt.tag)
			assert.Equal(t, tt.want, got, "Embed prefix should match")
		})
	}
}

func TestExtractColumnNameFromTag(t *testing.T) {
	tests := []struct {
		name      string
		tag       string
		fieldName string
		want      string
	}{
		{"NoTag", "", "UserName", "user_name"},
		{"DashIgnore", `bun:"-"`, "X", "-"},
		{"ExplicitColumn", `bun:"my_col"`, "Foo", "my_col"},
		{"ExplicitWithFlags", `bun:"my_col,notnull,pk"`, "Foo", "my_col"},
		{"OnlyFlagsScanonly", `bun:",scanonly"`, "Total", "total"},
		{"OnlyFlagsNotnull", `bun:",notnull"`, "CreatedAt", "created_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractColumnNameFromTag(tt.tag, tt.fieldName)
			assert.Equal(t, tt.want, got, "Column name should match")
		})
	}
}

func TestParseBunTag(t *testing.T) {
	tests := []struct {
		name      string
		tag       string
		wantTable string
		wantAlias string
	}{
		{"Empty", "", "", ""},
		{"TableOnly", `bun:"table:users"`, "users", ""},
		{"TableAndAlias", `bun:"table:users,alias:u"`, "users", "u"},
		{"AliasOnly", `bun:"alias:u"`, "", "u"},
		{"WithExtras", `bun:"table:users,alias:u,select:active_users"`, "users", "u"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTable, gotAlias := parseBunTag(tt.tag)
			assert.Equal(t, tt.wantTable, gotTable, "table should match")
			assert.Equal(t, tt.wantAlias, gotAlias, "alias should match")
		})
	}
}

func TestFieldTag(t *testing.T) {
	t.Run("NilTagReturnsEmpty", func(t *testing.T) {
		got := fieldTag(&ast.Field{})
		assert.Empty(t, got, "Field without tag should return empty")
	})

	t.Run("PreservesRawTagLiteral", func(t *testing.T) {
		raw := "`bun:\"name,notnull\"`"
		got := fieldTag(&ast.Field{Tag: &ast.BasicLit{Kind: token.STRING, Value: raw}})
		assert.Equal(t, raw, got, "Field tag should be returned verbatim")
	})
}

// TestGenerateFile covers all GenerateFile scenarios:
//   - Integration: end-to-end parsing + code generation against
//     testdata/models/sample.go, asserting scanonly retention with Columns()
//     exclusion, rel skipping, embed prefixes, label preservation, reserved
//     name handling, and the unexported-type / exported-var contract.
//   - NonExistentInput: missing input file surfaces an error and writes no output.
//   - NoModelsLeavesNoOutput: a valid file with no orm.BaseModel-embedding types
//     returns nil silently and writes no output.
//
// Note on naming convention in the generated code:
//   - Schema type names are unexported (e.g. `userSchema`). The generator
//     exposes each model only through the exported instance variable
//     (e.g. `User`), so the concrete type stays an implementation detail of
//     the schemas package and callers cannot construct or alias it directly.
//   - Schema struct field names go through lo.CamelCase, producing
//     **unexported** identifiers (e.g. `id`, `addrCity`, `__type`).
//     Users access columns through methods (e.g. `User.ID()`), not field reads.
//   - Schema method names retain the original Go field name's PascalCase
//     (e.g. `ID`, `AddrCity`), with `Col` prefix when the name collides with
//     the reserved set {Table, Alias, As, Columns}.
func TestGenerateFile(t *testing.T) {
	t.Run("Integration", func(t *testing.T) {
		inputFile := filepath.Join("testdata", "models", "sample.go")
		outputFile := filepath.Join(t.TempDir(), "schemas.go")

		err := GenerateFile(inputFile, outputFile, "schemas")
		require.NoError(t, err, "GenerateFile should succeed")

		content, err := os.ReadFile(outputFile)
		require.NoError(t, err, "Output file should be readable")

		code := string(content)

		fset := token.NewFileSet()
		parsedFile, err := parser.ParseFile(fset, outputFile, content, parser.ParseComments)
		require.NoError(t, err, "Generated file should parse as valid Go")

		require.Equal(t, "schemas", parsedFile.Name.Name, "Package name should match -p flag")

		t.Run("OnlyModelsWithBaseModelAreEmitted", func(t *testing.T) {
			assert.Contains(t, code, "type userSchema struct", "User should be emitted as an unexported schema type")
			assert.Contains(t, code, "type profileSchema struct", "Profile should be emitted as an unexported schema type")
			assert.NotContains(t, code, "NotAModel", "Structs without orm.BaseModel must be skipped")
		})

		t.Run("SchemaTypeIsUnexportedAndOnlyVarIsExported", func(t *testing.T) {
			assert.Contains(t, code, "var User = &userSchema{", "Exported var should reference the unexported schema type")
			assert.Contains(t, code, "var Profile = &profileSchema{", "Exported var should reference the unexported schema type")
			assert.NotRegexp(t, `\btype\s+UserSchema\b`, code, "Schema type must not be exported")
			assert.NotRegexp(t, `\btype\s+ProfileSchema\b`, code, "Schema type must not be exported")
		})

		t.Run("RelationFieldsSkipped", func(t *testing.T) {
			// Relation field names (Profile, Posts) lower-cased should not appear as struct fields.
			assert.NotRegexp(t, `\bprofile\s+string`, code, "User.Profile rel field must be skipped")
			assert.NotRegexp(t, `\bposts\s+string`, code, "User.Posts rel field must be skipped")
		})

		t.Run("BunDashFieldSkipped", func(t *testing.T) {
			assert.NotRegexp(t, `\binternal\s+string`, code, `bun:"-" field Internal must be skipped`)
		})

		t.Run("UnexportedFieldSkipped", func(t *testing.T) {
			assert.NotContains(t, code, "internalNote", "Unexported fields must be skipped")
		})

		t.Run("ScanonlyAppearsAsAccessor", func(t *testing.T) {
			assert.Contains(t, code, `computed: "computed"`, "scanonly column name should default to snake_case fieldName")
			assert.Contains(t, code, "func (s *userSchema) Computed(raw ...bool) string", "scanonly accessor must be generated")
		})

		t.Run("ScanonlyExcludedFromColumns", func(t *testing.T) {
			columnsBody := methodBodyText(t, fset, parsedFile, "userSchema", "Columns")
			assert.NotContains(t, columnsBody, "s.computed", "Computed must not be returned by Columns()")
			assert.NotContains(t, columnsBody, "s.createdByName", "Embedded scanonly CreatedByName must not be returned by Columns()")
			assert.Contains(t, columnsBody, "s.id", "Real columns should still be returned by Columns()")
			assert.Contains(t, columnsBody, "s.addrCity", "Embedded real columns should still be returned by Columns()")
		})

		t.Run("EmbedPrefixApplied", func(t *testing.T) {
			assert.Contains(t, code, `addrCity:`, "embed:addr_ should produce camelCase struct field addrCity")
			assert.Contains(t, code, `"addr_city"`, "embed:addr_ should prefix City column to addr_city")
			assert.Contains(t, code, `"addr_street"`, "embed:addr_ should prefix Street column to addr_street")
			assert.Contains(t, code, "func (s *userSchema) AddrCity(raw ...bool) string", "Embedded field must produce AddrCity accessor")
		})

		t.Run("AnonymousEmbedFlattens", func(t *testing.T) {
			assert.Contains(t, code, `createdBy:`, "Anonymous embed AuditInfo should flatten CreatedBy as createdBy field")
			assert.Contains(t, code, `createdByName:`, "Anonymous embed AuditInfo should flatten CreatedByName as createdByName field")
			assert.Contains(t, code, `"created_by"`, "CreatedBy column name should be created_by")
			assert.Contains(t, code, `"created_by_name"`, "CreatedByName column name should default to created_by_name")
		})

		t.Run("LabelWithSpacesPreserved", func(t *testing.T) {
			// label:"User Name" must survive struct tag parsing (regression test for the
			// previous strings.Fields-based extractor that truncated values at whitespace).
			assert.Contains(t, code, "// Name User Name", "Label with spaces should appear verbatim in method doc")
		})

		t.Run("GoKeywordFieldEscaped", func(t *testing.T) {
			assert.Contains(t, code, "__type:", "Go keyword field name (Type) should be prefixed with __ in goName")
		})

		t.Run("ReservedMethodNameEscaped", func(t *testing.T) {
			assert.Contains(t, code, "func (s *userSchema) ColTable(raw ...bool) string", "Field named Table should produce ColTable method to avoid collision with schema.Table()")
		})

		t.Run("TableAndAliasParsedFromBaseModelTag", func(t *testing.T) {
			assert.Contains(t, code, `_table: "users"`, "Table name from BaseModel tag should be applied")
			assert.Contains(t, code, `_alias: "u"`, "Alias from BaseModel tag should be applied")
		})

		t.Run("DefaultTableAliasFromModelName", func(t *testing.T) {
			// Profile uses bun:"table:profiles" without alias, so alias defaults to table.
			assert.Contains(t, code, `_alias: "profiles"`, "Alias should default to table when not specified")
		})
	})

	t.Run("NonExistentInput", func(t *testing.T) {
		dir := t.TempDir()
		err := GenerateFile(filepath.Join(dir, "does-not-exist.go"), filepath.Join(dir, "out.go"), "schemas")
		require.Error(t, err, "Missing input file should produce an error")

		_, statErr := os.Stat(filepath.Join(dir, "out.go"))
		assert.True(t, os.IsNotExist(statErr), "No output file should be created when parsing fails")
	})

	t.Run("NoModelsLeavesNoOutput", func(t *testing.T) {
		// testdata/empty/empty.go is a valid Go file with no orm.BaseModel types,
		// so GenerateFile must return nil silently and write nothing.
		inputFile := filepath.Join("testdata", "empty", "empty.go")
		outputFile := filepath.Join(t.TempDir(), "out.go")

		err := GenerateFile(inputFile, outputFile, "schemas")
		require.NoError(t, err, "Source with no models should not error")

		_, statErr := os.Stat(outputFile)
		assert.True(t, os.IsNotExist(statErr), "No output file should be written when there is nothing to generate")
	})
}

// methodBodyText returns the source text of method (*receiverType).methodName
// from the parsed file using go/printer for fidelity, or fails the test if
// the method is missing.
func methodBodyText(t *testing.T, fset *token.FileSet, file *ast.File, receiverType, methodName string) string {
	t.Helper()

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != methodName {
			continue
		}

		star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}

		ident, ok := star.X.(*ast.Ident)
		if !ok || ident.Name != receiverType {
			continue
		}

		var buf bytes.Buffer
		require.NoError(t, printer.Fprint(&buf, fset, fn.Body), "Method body should print successfully")

		return buf.String()
	}

	t.Fatalf("method (*%s).%s not found in generated file", receiverType, methodName)

	return ""
}

func TestGenerateDirectory(t *testing.T) {
	t.Run("HappyPath", func(t *testing.T) {
		inputDir := filepath.Join("testdata", "models")
		outputDir := t.TempDir()

		err := GenerateDirectory(inputDir, outputDir, "schemas")
		require.NoError(t, err, "GenerateDirectory should succeed on the models fixture")

		outputFile := filepath.Join(outputDir, "sample.go")
		content, err := os.ReadFile(outputFile)
		require.NoError(t, err, "Output should be created with the same basename as the input file")

		code := string(content)
		assert.Contains(t, code, "package schemas", "Generated file should use the requested package name")
		assert.Contains(t, code, "type userSchema struct", "Schema type should remain unexported")
		assert.Contains(t, code, "type profileSchema struct", "Schema type should remain unexported")
		assert.Contains(t, code, "var User = &userSchema{", "Exported var should expose the unexported schema type")
	})

	t.Run("EmptyDirectory", func(t *testing.T) {
		err := GenerateDirectory(t.TempDir(), t.TempDir(), "schemas")
		require.ErrorIs(t, err, ErrNoGoFilesFound, "Empty input directory should yield ErrNoGoFilesFound")
	})

	t.Run("NonExistentDirectory", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "does-not-exist")

		err := GenerateDirectory(missing, t.TempDir(), "schemas")
		require.ErrorIs(t, err, ErrNoGoFilesFound, "Glob over a missing directory returns no files → ErrNoGoFilesFound")
	})

	t.Run("SkipsFilesWithoutModels", func(t *testing.T) {
		// testdata/empty/empty.go has no orm.BaseModel types, so GenerateFile
		// no-ops; GenerateDirectory must therefore succeed and produce no output.
		inputDir := filepath.Join("testdata", "empty")
		outputDir := t.TempDir()

		err := GenerateDirectory(inputDir, outputDir, "schemas")
		require.NoError(t, err, "Directory with only no-model files should succeed silently")

		_, statErr := os.Stat(filepath.Join(outputDir, "empty.go"))
		assert.True(t, os.IsNotExist(statErr), "No output should be written for files that have no models")
	})
}

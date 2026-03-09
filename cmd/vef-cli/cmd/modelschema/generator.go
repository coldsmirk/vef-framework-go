package modelschema

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/coldsmirk/go-streams"
	"github.com/samber/lo"
	"golang.org/x/tools/go/packages"
)

var (
	// ErrNoGoFilesFound indicates no .go files were found in the directory.
	ErrNoGoFilesFound = errors.New("no .go files found in directory")
	// ErrNoPackagesFound indicates no packages were found during parsing.
	ErrNoPackagesFound = errors.New("no packages found")
	// ErrMultiplePackages indicates multiple packages were found when expecting one.
	ErrMultiplePackages = errors.New("expected 1 package, found multiple")
	// ErrFileNotFoundInPackage indicates the target file was not found in the package.
	ErrFileNotFoundInPackage = errors.New("file not found in package")
)

var (
	goKeywords = map[string]bool{
		"break": true, "case": true, "chan": true, "const": true, "continue": true,
		"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
		"func": true, "go": true, "goto": true, "if": true, "import": true,
		"interface": true, "map": true, "package": true, "range": true, "return": true,
		"select": true, "struct": true, "switch": true, "type": true, "var": true,
	}

	reservedMethodNames = map[string]bool{
		"Table": true, "Alias": true, "As": true, "Columns": true,
	}
)

// ModelField represents a single field in a model struct with its Go and database metadata.
type ModelField struct {
	GoName     string
	ColumnName string
	MethodName string
	Label      string
}

// ModelSchemaInfo contains complete metadata for generating a model schema helper.
type ModelSchemaInfo struct {
	PackageName    string
	ModelName      string
	SchemaTypeName string
	VarName        string
	TableName      string
	AliasName      string
	Fields         []ModelField
}

// GenerateFile processes a single model file and generates its schema file.
func GenerateFile(inputFile, outputFile, packageName string) error {
	schemas, err := parseModelFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to parse model file %s: %w", inputFile, err)
	}

	if len(schemas) == 0 {
		return nil
	}

	for _, schema := range schemas {
		schema.PackageName = packageName
	}

	code := generateSchemaCode(schemas)

	dir := filepath.Dir(outputFile)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	if err := os.WriteFile(outputFile, []byte(code), 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// GenerateDirectory processes all .go files in a directory and generates corresponding schemas.
func GenerateDirectory(inputDir, outputDir, packageName string) error {
	files, err := filepath.Glob(filepath.Join(inputDir, "*.go"))
	if err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("%w: %s", ErrNoGoFilesFound, inputDir)
	}

	return streams.FromSlice(files).ForEachErr(func(inputFile string) error {
		outputFile := filepath.Join(outputDir, filepath.Base(inputFile))

		return GenerateFile(inputFile, outputFile, packageName)
	})
}

func parseModelFile(filename string) ([]*ModelSchemaInfo, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
	}

	ps, err := packages.Load(cfg, "file="+filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load package: %w", err)
	}

	if len(ps) == 0 {
		return nil, ErrNoPackagesFound
	}

	if len(ps) > 1 {
		return nil, fmt.Errorf("%w: %d", ErrMultiplePackages, len(ps))
	}

	pkg := ps[0]
	if len(pkg.Errors) > 0 {
		return nil, fmt.Errorf("package load error: %w", pkg.Errors[0])
	}

	absFilename, _ := filepath.Abs(filename)

	var targetFile *ast.File
	for i, goFile := range pkg.GoFiles {
		absGoFile, _ := filepath.Abs(goFile)
		if absGoFile == absFilename && i < len(pkg.Syntax) {
			targetFile = pkg.Syntax[i]

			break
		}
	}

	if targetFile == nil {
		return nil, ErrFileNotFoundInPackage
	}

	var schemas []*ModelSchemaInfo
	for _, decl := range targetFile.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			modelName := typeSpec.Name.Name

			hasBaseModel, tableName, aliasName := extractTableMetadata(structType, modelName, pkg)
			if !hasBaseModel {
				continue
			}

			schemaInfo := &ModelSchemaInfo{
				ModelName:      modelName,
				SchemaTypeName: lo.CamelCase(modelName + "Schema"),
				VarName:        modelName,
				TableName:      tableName,
				AliasName:      aliasName,
				Fields:         parseStructFields(structType, pkg),
			}

			schemas = append(schemas, schemaInfo)
		}
	}

	return schemas, nil
}

func extractTableMetadata(structType *ast.StructType, modelName string, pkg *packages.Package) (hasBaseModel bool, tableName, aliasName string) {
	for _, f := range structType.Fields.List {
		if len(f.Names) > 0 {
			continue
		}

		if !isOrmBaseModel(f.Type, pkg) {
			continue
		}

		if f.Tag != nil {
			tableName, aliasName = parseBunTag(f.Tag.Value)
		}

		if tableName == "" {
			tableName = lo.SnakeCase(modelName)
		}

		if aliasName == "" {
			aliasName = tableName
		}

		return true, tableName, aliasName
	}

	return false, "", ""
}

func isOrmBaseModel(expr ast.Expr, pkg *packages.Package) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}

	if ident.Name != "orm" || selector.Sel.Name != "BaseModel" {
		return false
	}

	obj := pkg.TypesInfo.Uses[ident]
	if obj == nil {
		return false
	}

	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}

	return pkgName.Imported().Path() == "github.com/coldsmirk/vef-framework-go/orm"
}

func parseStructFields(structType *ast.StructType, pkg *packages.Package) []ModelField {
	var fields []ModelField

	for _, f := range structType.Fields.List {
		if len(f.Names) == 0 {
			if isOrmBaseModel(f.Type, pkg) {
				continue
			}

			if hasIgnoreTag(f) {
				continue
			}

			inheritedFields := parseInheritedFields(f.Type, "", pkg)
			fields = append(fields, inheritedFields...)

			continue
		}

		for _, name := range f.Names {
			fieldName := name.Name

			if !unicode.IsUpper(rune(fieldName[0])) {
				continue
			}

			if embedPrefix := extractEmbedPrefix(f); embedPrefix != "" {
				embeddedFields := parseInheritedFields(f.Type, embedPrefix, pkg)
				fields = append(fields, embeddedFields...)

				continue
			}

			columnName := extractColumnName(f, fieldName)
			if columnName == "-" {
				continue
			}

			goName := lo.CamelCase(fieldName)
			if goKeywords[goName] {
				goName = "__" + goName
			}

			label := extractLabel(f)

			methodName := fieldName
			if reservedMethodNames[fieldName] {
				methodName = "Col" + fieldName
			}

			fields = append(fields, ModelField{
				GoName:     goName,
				ColumnName: columnName,
				MethodName: methodName,
				Label:      label,
			})
		}
	}

	return fields
}

// hasIgnoreTag checks if a field has bun:"-" tag.
func hasIgnoreTag(f *ast.Field) bool {
	if f.Tag == nil {
		return false
	}

	bunTag := extractStructTag(f.Tag.Value, "bun")

	return bunTag == "-"
}

// parseInheritedFields recursively parses inherited fields from embedded structs with optional prefix accumulation.
func parseInheritedFields(typeExpr ast.Expr, prefix string, pkg *packages.Package) []ModelField {
	tv, ok := pkg.TypesInfo.Types[typeExpr]
	if !ok {
		return nil
	}

	return parseInheritedFieldsFromType(tv.Type, prefix)
}

// parseInheritedFieldsFromType recursively parses inherited fields from a types.Type.
func parseInheritedFieldsFromType(typ types.Type, prefix string) []ModelField {
	var fields []ModelField

	if alias, ok := typ.(*types.Alias); ok {
		typ = alias.Rhs()
	}

	named, ok := typ.(*types.Named)
	if !ok {
		return fields
	}

	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		return fields
	}

	for i := range structType.NumFields() {
		field := structType.Field(i)

		if !field.Exported() {
			continue
		}

		tag := structType.Tag(i)

		bunTag := extractStructTag(tag, "bun")
		if bunTag == "-" {
			continue
		}

		if field.Anonymous() {
			nestedFields := parseInheritedFieldsFromType(field.Type(), prefix)
			fields = append(fields, nestedFields...)

			continue
		}

		if embedPrefix := extractEmbedPrefixFromTag(tag); embedPrefix != "" {
			nestedPrefix := prefix + embedPrefix
			nestedFields := parseInheritedFieldsFromType(field.Type(), nestedPrefix)
			fields = append(fields, nestedFields...)

			continue
		}

		fieldName := field.Name()
		columnName := extractColumnNameFromTag(tag, fieldName)

		if columnName == "-" {
			continue
		}

		finalColumnName := prefix + columnName

		goName := lo.CamelCase(prefix + fieldName)
		if goKeywords[goName] {
			goName = "__" + goName
		}

		label := extractLabelFromTag(tag)

		methodName := lo.PascalCase(prefix) + fieldName
		if reservedMethodNames[methodName] {
			methodName = "Col" + methodName
		}

		fields = append(fields, ModelField{
			GoName:     goName,
			ColumnName: finalColumnName,
			MethodName: methodName,
			Label:      label,
		})
	}

	return fields
}

// extractEmbedPrefixFromTag extracts the embed prefix from a bun struct tag.
func extractEmbedPrefixFromTag(tag string) string {
	bunTag := extractStructTag(tag, "bun")
	if bunTag == "" {
		return ""
	}

	parts := strings.SplitSeq(bunTag, ",")
	for part := range parts {
		part = strings.TrimSpace(part)
		if prefix, ok := strings.CutPrefix(part, "embed:"); ok {
			return prefix
		}
	}

	return ""
}

func extractColumnNameFromTag(tag, fieldName string) string {
	bunTag := extractStructTag(tag, "bun")
	if bunTag == "" {
		return lo.SnakeCase(fieldName)
	}

	if bunTag == "-" {
		return "-"
	}

	parts := strings.Split(bunTag, ",")
	for _, part := range parts {
		if strings.TrimSpace(part) == "scanonly" {
			return "-"
		}
	}

	if len(parts) > 0 && parts[0] != "" {
		if strings.Contains(parts[0], "\n") {
			return lo.SnakeCase(fieldName)
		}

		return parts[0]
	}

	return lo.SnakeCase(fieldName)
}

func extractLabelFromTag(tag string) string {
	return extractStructTag(tag, "label")
}

func extractStructTag(tag, key string) string {
	tag = strings.Trim(tag, "`")

	parts := strings.Fields(tag)
	prefix := key + `:"`

	for _, part := range parts {
		if after, ok := strings.CutPrefix(part, prefix); ok {
			return strings.TrimSpace(strings.TrimSuffix(after, "\""))
		}
	}

	return ""
}

func extractEmbedPrefix(f *ast.Field) string {
	if f.Tag == nil {
		return ""
	}

	bunTag := extractStructTag(f.Tag.Value, "bun")
	if bunTag == "" {
		return ""
	}

	parts := strings.SplitSeq(bunTag, ",")
	for part := range parts {
		part = strings.TrimSpace(part)
		if prefix, ok := strings.CutPrefix(part, "embed:"); ok {
			return prefix
		}
	}

	return ""
}

func extractLabel(f *ast.Field) string {
	if f.Tag == nil {
		return ""
	}

	return extractLabelFromTag(f.Tag.Value)
}

func extractColumnName(f *ast.Field, fieldName string) string {
	if f.Tag == nil {
		return lo.SnakeCase(fieldName)
	}

	return extractColumnNameFromTag(f.Tag.Value, fieldName)
}

func parseBunTag(tagValue string) (table, alias string) {
	bunTag := extractStructTag(tagValue, "bun")
	if bunTag == "" {
		return table, alias
	}

	parts := strings.SplitSeq(bunTag, ",")
	for part := range parts {
		part = strings.TrimSpace(part)
		if after, ok := strings.CutPrefix(part, "table:"); ok {
			table = after
		} else if after, ok := strings.CutPrefix(part, "alias:"); ok {
			alias = after
		}
	}

	return table, alias
}

func generateSchemaCode(schemas []*ModelSchemaInfo) string {
	if len(schemas) == 0 {
		return ""
	}

	fset := token.NewFileSet()
	file := fset.AddFile("", -1, 1000)
	commentPos := file.Pos(1)
	packagePos := file.Pos(2)

	astFile := &ast.File{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{
				{
					Slash: commentPos,
					Text:  "// Code generated by vef-cli. DO NOT EDIT.",
				},
			},
		},
		Package: packagePos,
		Name:    ast.NewIdent(schemas[0].PackageName),
		Decls:   []ast.Decl{buildImportDecl()},
	}

	for _, schema := range schemas {
		astFile.Decls = append(astFile.Decls, buildVarDecl(schema))
		astFile.Decls = append(astFile.Decls, buildTypeDecl(schema))
		astFile.Decls = append(astFile.Decls, buildFieldMethods(schema)...)
		astFile.Decls = append(astFile.Decls, buildTableMethod(schema))
		astFile.Decls = append(astFile.Decls, buildAliasMethod(schema))
		astFile.Decls = append(astFile.Decls, buildAsMethod(schema))
		astFile.Decls = append(astFile.Decls, buildColumnsMethod(schema))
		astFile.Decls = append(astFile.Decls, buildColumnMethod(schema))
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, astFile); err != nil {
		panic(fmt.Sprintf("failed to format code: %v", err))
	}

	return buf.String()
}

func buildImportDecl() *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.IMPORT,
		Specs: []ast.Spec{
			&ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: `"github.com/coldsmirk/vef-framework-go/dbx"`,
				},
			},
		},
	}
}

func buildVarDecl(schema *ModelSchemaInfo) *ast.GenDecl {
	var elements []ast.Expr

	elements = append(elements, &ast.KeyValueExpr{
		Key:   ast.NewIdent("_table"),
		Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", schema.TableName)},
	})

	elements = append(elements, &ast.KeyValueExpr{
		Key:   ast.NewIdent("_alias"),
		Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", schema.AliasName)},
	})

	for _, f := range schema.Fields {
		elements = append(elements, &ast.KeyValueExpr{
			Key:   ast.NewIdent(f.GoName),
			Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", f.ColumnName)},
		})
	}

	return &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{ast.NewIdent(schema.VarName)},
				Values: []ast.Expr{
					&ast.UnaryExpr{
						Op: token.AND,
						X: &ast.CompositeLit{
							Type: ast.NewIdent(schema.SchemaTypeName),
							Elts: elements,
						},
					},
				},
			},
		},
	}
}

func buildTypeDecl(schema *ModelSchemaInfo) *ast.GenDecl {
	var fields []*ast.Field

	fields = append(fields, &ast.Field{
		Names: []*ast.Ident{ast.NewIdent("_table")},
		Type:  ast.NewIdent("string"),
	})

	fields = append(fields, &ast.Field{
		Names: []*ast.Ident{ast.NewIdent("_alias")},
		Type:  ast.NewIdent("string"),
	})

	for _, f := range schema.Fields {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(f.GoName)},
			Type:  ast.NewIdent("string"),
		})
	}

	return &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ast.NewIdent(schema.SchemaTypeName),
				Type: &ast.StructType{
					Fields: &ast.FieldList{
						List: fields,
					},
				},
			},
		},
	}
}

func buildFieldMethods(schema *ModelSchemaInfo) []ast.Decl {
	var decls []ast.Decl

	for _, f := range schema.Fields {
		var doc *ast.CommentGroup
		if f.Label != "" {
			doc = &ast.CommentGroup{
				List: []*ast.Comment{
					{Text: fmt.Sprintf("// %s %s", f.MethodName, f.Label)},
				},
			}
		}

		decls = append(decls, &ast.FuncDecl{
			Doc: doc,
			Recv: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("s")},
						Type: &ast.StarExpr{
							X: ast.NewIdent(schema.SchemaTypeName),
						},
					},
				},
			},
			Name: ast.NewIdent(f.MethodName),
			Type: &ast.FuncType{
				Params: &ast.FieldList{
					List: []*ast.Field{
						{
							Names: []*ast.Ident{ast.NewIdent("raw")},
							Type: &ast.Ellipsis{
								Elt: ast.NewIdent("bool"),
							},
						},
					},
				},
				Results: &ast.FieldList{
					List: []*ast.Field{
						{Type: ast.NewIdent("string")},
					},
				},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.ReturnStmt{
						Results: []ast.Expr{
							&ast.CallExpr{
								Fun: &ast.SelectorExpr{
									X:   ast.NewIdent("s"),
									Sel: ast.NewIdent("column"),
								},
								Args: []ast.Expr{
									&ast.SelectorExpr{
										X:   ast.NewIdent("s"),
										Sel: ast.NewIdent(f.GoName),
									},
									ast.NewIdent("raw..."),
								},
							},
						},
					},
				},
			},
		})
	}

	return decls
}

func buildTableMethod(schema *ModelSchemaInfo) *ast.FuncDecl {
	return &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{
				{Text: "// Table returns the table name."},
			},
		},
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("s")},
					Type: &ast.StarExpr{
						X: ast.NewIdent(schema.SchemaTypeName),
					},
				},
			},
		},
		Name: ast.NewIdent("Table"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("string")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.SelectorExpr{
							X:   ast.NewIdent("s"),
							Sel: ast.NewIdent("_table"),
						},
					},
				},
			},
		},
	}
}

func buildAliasMethod(schema *ModelSchemaInfo) *ast.FuncDecl {
	return &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{
				{Text: "// Alias returns the table alias."},
			},
		},
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("s")},
					Type: &ast.StarExpr{
						X: ast.NewIdent(schema.SchemaTypeName),
					},
				},
			},
		},
		Name: ast.NewIdent("Alias"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("string")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.SelectorExpr{
							X:   ast.NewIdent("s"),
							Sel: ast.NewIdent("_alias"),
						},
					},
				},
			},
		},
	}
}

func buildAsMethod(schema *ModelSchemaInfo) *ast.FuncDecl {
	return &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{
				{Text: "// As creates a copy with a new alias."},
			},
		},
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("s")},
					Type: &ast.StarExpr{
						X: ast.NewIdent(schema.SchemaTypeName),
					},
				},
			},
		},
		Name: ast.NewIdent("As"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("alias")},
						Type:  ast.NewIdent("string"),
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.StarExpr{
							X: ast.NewIdent(schema.SchemaTypeName),
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("copied")},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						&ast.StarExpr{
							X: ast.NewIdent("s"),
						},
					},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   ast.NewIdent("copied"),
							Sel: ast.NewIdent("_alias"),
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						ast.NewIdent("alias"),
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.UnaryExpr{
							Op: token.AND,
							X:  ast.NewIdent("copied"),
						},
					},
				},
			},
		},
	}
}

func buildColumnsMethod(schema *ModelSchemaInfo) *ast.FuncDecl {
	var elements []ast.Expr
	for _, f := range schema.Fields {
		elements = append(elements, &ast.SelectorExpr{
			X:   ast.NewIdent("s"),
			Sel: ast.NewIdent(f.GoName),
		})
	}

	return &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{
				{Text: "// Columns returns all column names."},
			},
		},
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("s")},
					Type: &ast.StarExpr{
						X: ast.NewIdent(schema.SchemaTypeName),
					},
				},
			},
		},
		Name: ast.NewIdent("Columns"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.ArrayType{
							Elt: ast.NewIdent("string"),
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CompositeLit{
							Type: &ast.ArrayType{
								Elt: ast.NewIdent("string"),
							},
							Elts: elements,
						},
					},
				},
			},
		},
	}
}

func buildColumnMethod(schema *ModelSchemaInfo) *ast.FuncDecl {
	return &ast.FuncDecl{
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("s")},
					Type: &ast.StarExpr{
						X: ast.NewIdent(schema.SchemaTypeName),
					},
				},
			},
		},
		Name: ast.NewIdent("column"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("name")},
						Type:  ast.NewIdent("string"),
					},
					{
						Names: []*ast.Ident{ast.NewIdent("raw")},
						Type: &ast.Ellipsis{
							Elt: ast.NewIdent("bool"),
						},
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: ast.NewIdent("string")},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.BinaryExpr{
							X: &ast.CallExpr{
								Fun:  ast.NewIdent("len"),
								Args: []ast.Expr{ast.NewIdent("raw")},
							},
							Op: token.GTR,
							Y:  &ast.BasicLit{Kind: token.INT, Value: "0"},
						},
						Op: token.LAND,
						Y: &ast.IndexExpr{
							X:     ast.NewIdent("raw"),
							Index: &ast.BasicLit{Kind: token.INT, Value: "0"},
						},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{
								Results: []ast.Expr{ast.NewIdent("name")},
							},
						},
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.CallExpr{
							Fun: &ast.SelectorExpr{
								X:   ast.NewIdent("dbx"),
								Sel: ast.NewIdent("ColumnWithAlias"),
							},
							Args: []ast.Expr{
								ast.NewIdent("name"),
								&ast.SelectorExpr{
									X:   ast.NewIdent("s"),
									Sel: ast.NewIdent("_alias"),
								},
							},
						},
					},
				},
			},
		},
	}
}

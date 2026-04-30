# Tabular Dynamic Import / Export

The `tabular` package bridges the CSV / Excel import/export engines with any
row model via a single `tabular.RowAdapter` abstraction. Two first-party
adapters ship out of the box, covering both the static (struct) and dynamic
(map) cases.

## Adapters

- `tabular.NewStructAdapterFor[T]()` — parses struct fields tagged with
  `tabular:"..."`, enforces `validate:"..."` tags via the framework validator,
  and returns `[]T` from the importer.
- `tabular.NewMapAdapter(schema)` — operates on `[]map[string]any` rows.
  Schemas are constructed from `[]tabular.ColumnSpec` at runtime, so the
  column set can depend on tenant, configuration, or other runtime state.

Custom adapters can implement `tabular.RowAdapter` directly to wrap any row
representation (e.g. `chan map[string]any`, domain-specific types).

## Static usage

```go
type User struct {
    ID   int    `tabular:"ID"   validate:"required"`
    Name string `tabular:"Name" validate:"required"`
}

exp := csv.NewExporterFor[User]()
imp := csv.NewImporterFor[User]()

buf, err := exp.Export(users)              // buf has CSV bytes
result, errs, err := imp.Import(buf)        // result is []User
```

## Dynamic usage

```go
specs := []tabular.ColumnSpec{
    {Key: "id",       Name: "用户ID", Type: reflect.TypeFor[int](),       Required: true},
    {Key: "name",     Name: "姓名",  Type: reflect.TypeFor[string](),    Required: true},
    {Key: "birthday", Name: "生日",  Type: reflect.TypeFor[time.Time](), Format: "2006-01-02"},
    {Key: "active",   Name: "激活",  Type: reflect.TypeFor[bool](),      Default: "false"},
}

exp, err := excel.NewMapExporter(specs, excel.WithSheetName("Users"))
imp, err := csv.NewMapImporter(specs)

rows := []map[string]any{
    {"id": 1, "name": "张三", "birthday": time.Now(), "active": true},
}
buf, err := exp.Export(rows)

res, errs, err := imp.Import(strings.NewReader("..."))
users := res.([]map[string]any)
```

### Validation hooks

- `Required: true` rejects empty / absent cells with `ErrRequiredMissing`.
- `Validators []CellValidator` runs per-cell after parsing.
- `WithRowValidator(RowValidator)` runs after the full row is populated:

```go
imp, _ := csv.NewMapImporterWithOptions(
    specs,
    []tabular.MapOption{tabular.WithRowValidator(func(row map[string]any) error {
        if row["name"] == "" {
            return errors.New("name must not be empty")
        }
        return nil
    })},
)
```

### Custom formatters / parsers

Every column resolves a `Formatter` / `ValueParser` with a fixed precedence:

1. `Column.FormatterFn` / `Column.ParserFn` (direct instance on the column).
2. `Column.Formatter` / `Column.Parser` (name lookup in exporter / importer
   registries, i.e. what `RegisterFormatter` / `RegisterParser` set up).
3. `tabular.NewDefaultFormatter(Column.Format)` /
   `tabular.NewDefaultParser(Column.Format)`.

The default implementations understand common Go primitives, `time.Time`,
`timex.DateTime/Date/Time`, and `decimal.Decimal`.

## Entry points

Both `csv` and `excel` expose mirrored entry points:

- `NewImporter(adapter, opts...)`
- `NewExporter(adapter, opts...)`
- `NewImporterFor[T](opts...)`
- `NewExporterFor[T](opts...)`
- `NewMapImporter(specs, opts...)`
- `NewMapImporterWithOptions(specs, mapOpts, opts...)`
- `NewMapExporter(specs, opts...)`

The spec-based constructors return an `error` when
`tabular.NewSchemaFromSpecs(specs)` fails (missing Key / Type, duplicate keys,
etc.), giving callers an immediate, deterministic failure mode.

## Writing a custom adapter

A minimal `RowAdapter` only needs to satisfy three methods:

```go
func (a *myAdapter) Schema() *tabular.Schema
func (a *myAdapter) Reader(data any) (tabular.RowReader, error)
func (a *myAdapter) Writer(capacity int) tabular.RowWriter
```

`RowReader` yields `RowView` values via `iter.Seq2[int, RowView]`; each
`RowView.Get(col)` returns the raw cell value for the column. `RowWriter`
creates `RowBuilder` instances, accepts `Set(col, value)` calls, and defines
`Validate()` for per-row checks. Implementing these three interfaces plugs
your custom model directly into every existing csv / excel importer and
exporter.

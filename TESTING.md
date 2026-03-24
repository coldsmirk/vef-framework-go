# VEF Framework Go — Testing Guidelines

Testing stack: `testify/suite` + `testify/assert` + `testify/require` + Testcontainers (PostgreSQL, MySQL, SQLite).

## Suite vs Simple Tests

**Use `testify/suite`** when you need lifecycle hooks (`SetupSuite`/`TearDownSuite`), shared state (DB connections, containers), or base suite inheritance.

**Use simple table-driven tests** for pure functions with no external dependencies or shared state.

## Naming Conventions

### Project-Specific Rules

- **Consecutive acronyms**: keep the semantically important one in standard form, Pascal-case the other for readability.
  - `HTTPSUrl` (emphasizing HTTPS) or `HttpsURL` (emphasizing URL)
  - `JSONApi` (emphasizing JSON) or `JsonAPI` (emphasizing API)
  - Avoid all-caps: ~~`HTTPSURL`~~, ~~`JSONAPI`~~

### Test Names

| Element                        | Pattern              | Examples                                         |
| ------------------------------ | -------------------- | ------------------------------------------------ |
| Suite                          | `<Feature>TestSuite` | `BasicExpressionsTestSuite`                      |
| Method                         | `Test<Feature>`      | `TestColumn`, `TestDecode`                       |
| Sub-test (`t.Run`/`suite.Run`) | PascalCase           | `"SimpleColumnReference"`, `"NullValueHandling"` |
| Table-driven `name` field      | PascalCase           | `"EmptyInput"`, `"TransferSuperiorFound"`        |

Sub-test naming: start with verb for actions (`"CreateUser"`), descriptive nouns for scenarios (`"ComplexCaseWithMultipleConditions"`). Prefer concise names (typically 3-8 words), but prioritize semantic accuracy when tradeoffs arise.

**Important**: never encode sub-scenarios as underscores in test method names (`TestFoo_Bar`). Instead, define a single `TestFoo` method with nested `s.Run("Bar", ...)` subtests. This keeps test hierarchy clean: `TestSuite/TestFoo/Bar`.

## Test Structure

### Suite Tests

```go
func (suite *YourTestSuite) TestFeature() {
    suite.Run("ScenarioOne", func() {
        // Define inline result struct, build query, assert
    })
}
```

For **cross-database suite tests** (e.g., ORM tests running on PostgreSQL/MySQL/SQLite), log the database type at method start to distinguish output:

```go
func (suite *YourTestSuite) TestFeature() {
    suite.T().Logf("Testing Feature for %s", suite.dbKind)
    // ...
}
```

Non-cross-database suite tests do **not** need this log — the test name already provides sufficient context.

**Data isolation**: testify suite's `TearDownTest`/`SetupTest` only run between top-level `Test*` methods — **not** between `s.Run()` subtests. When subtests share mutable state (e.g., database rows), add per-subtest cleanup via `defer` to prevent data leaking between subtests.

### Simple Table-Driven Tests

```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"EmptyInput", "", ""},
        {"NormalInput", "hello", "HELLO"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Transform(tt.input)
            assert.Equal(t, tt.expected, got, "Should return expected result")
        })
    }
}
```

### Key Patterns

- Define result structs **inline** within sub-tests, name as `<Purpose>Result`.
- Cover: happy path, edge cases, null handling, complex scenarios.
- Log result details for debugging: `suite.T().Logf(...)`.

## Assertions

### `require` vs `assert`

- **`require`**: fails immediately and stops the test. Use for **preconditions** — if this fails, subsequent assertions are meaningless.
- **`assert`**: records the failure but continues. Use for **verifications** — you want to see all failures at once.

**Rule of thumb**: use `require` for error checks, nil checks, and length checks that guard subsequent access. Use `assert` for everything else.

### Suite Tests — Use Instance Methods

Suite 测试中**必须使用实例方法**，不要导入独立的 `assert`/`require` 包：

- 前置条件：`s.Require().NoError(err, "msg")`、`s.Require().NotNil(result, "msg")`
- 验证断言：`s.Equal(expected, actual, "msg")`、`s.NotEmpty(value, "msg")`

禁止使用 `require.NoError(s.T(), ...)` 或 `assert.Equal(s.T(), ...)` 形式。

### Simple Tests — Standalone Functions

非 Suite 测试使用独立的 `require`/`assert` 包：`require.NoError(t, err, "msg")`、`assert.Equal(t, expected, actual, "msg")`。

### Message Rules

**Every assertion must have a descriptive message** — describe what _should_ happen, not what failed.

```go
// Good
assert.Equal(t, expected, actual, "Published status should have priority 1")
assert.NotEmpty(t, result.Name, "Name should not be empty")

// Bad — missing message
assert.Equal(t, expected, actual)
assert.NotEmpty(t, result.Name)
```

## Cross-Database Testing

### Core Principle

The ORM abstracts database differences. **Do not skip tests** just because a database lacks native support — the framework may simulate it.

**Only skip** when a feature is fundamentally impossible to simulate. When skipping, use `suite.T().Skipf()` with a reason.

```go
// Skip only when truly unsimulatable
if suite.dbKind == config.MySQL {
    suite.T().Skipf("FILTER clause not supported on %s (cannot be simulated)", suite.dbKind)
    return
}
```

**Key rule**: if a test passes on one DB but fails on another, it's likely a framework bug, not a reason to skip.

### Running Multi-DB Tests

The ORM test suite uses a unified entry point: `TestAll`. Database matrix execution is handled by `registry.RunAll(...)` in `orm_test.go`, and suite factories are registered via `registry.Add(...)` in each suite file.

```bash
# Run specific DB
go test ./internal/orm -run TestAll/Postgres -v

# Run specific suite on specific DB
go test ./internal/orm -run TestAll/Postgres/EBAggregationFunctions -v

# Run specific test method
go test ./internal/orm -run TestAll/Postgres/EBAggregationFunctions/TestBitOr -v

# Quick iteration (SQLite, fastest)
go test ./internal/orm -run TestAll/SQLite -v

# Full validation (all DBs + race detection)
go test ./internal/orm -v -race
```

Test path hierarchy: `TestAll/<DBDisplayName>/<SuiteName>/<TestMethodName>/<SubTestName>`.

**Note**: running `go test -run TestBitOr` directly won't work — must include the `TestAll/...` prefix.

## Comments

- One-line doc comment is sufficient for most test methods.
- Only add inline comments for complex logic, non-obvious behavior, or cross-database differences.
- Don't repeat what the code or test name already says.
- Document known framework bugs with `// FRAMEWORK BUG:` and a TODO referencing the issue.

## Verification Mindset

When encountering uncertain behavior or assumptions:

1. **Question** — existing comments, docs, and tests can be outdated.
2. **Research** — check official database docs, library docs, language specs.
3. **Test** — validate hypotheses empirically by running code.
4. **Fix** — correct bugs wherever they exist (framework, tests, or docs).
5. **Document** — leave concise findings for future developers.

Never assume database limitations without verification. Technology evolves — check current version capabilities.

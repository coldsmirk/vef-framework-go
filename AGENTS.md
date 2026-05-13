# VEF Framework Go — Agent Guide

Read this file before touching code. It covers architecture, conventions, and workflow requirements.

This file is the shared, self-contained source of truth for repository-wide agent instructions. `TESTING.md` may provide fuller examples, but any rule that must be followed reliably should also be summarized here.

## Essential Commands

```bash
go test ./...                  # Run all tests (required before submitting)
go test -race ./...            # Race detection
golangci-lint run              # Lint (auto-fix: golangci-lint run --fix)
go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -test ./...  # Modernize checks
```

## Task Workflow

1. **Simple tasks**: directly implement, write tests, run verification.
2. **Complex tasks**: plan architecture first and get confirmation before wide refactors, public API changes, or architecture changes. If the user explicitly asks for parallel agent work, split the work into independent scopes with clear file ownership.
3. **Code review**: after implementation, review code to ensure it hasn't drifted from the task goal.
4. **Tests**: all general tasks must include corresponding test code. `TESTING.md` has fuller examples, but the critical rules are summarized in this file.
5. **Simplification**: after each task, do a simplification pass yourself. Prefer smaller, clearer code over extra abstractions.
6. **Verification**: run the narrowest relevant checks during development, and finish with `go test ./...`, `golangci-lint run`, and `go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -test ./...` when the task scope allows it. All required checks green = feature complete.

## Definition of Done

- The requested behavior or analysis is complete and scoped correctly.
- Relevant tests or documentation are added or updated when needed.
- Verification was run and the result was reported.
- If you changed workflow-critical conventions, `AGENTS.md` is updated as well.

## Architecture Overview

- **Stack**: Go 1.26.0 + Fiber v3 + Uber FX + Bun ORM. Default language: Simplified Chinese (`VEF_I18N_LANGUAGE`).
- **Structure**: public packages at root (`api`, `crud`, `orm`, `security`, `result`, etc.), internal implementations under `internal/`.
- **Boot sequence** (`vef.Run()` in `bootstrap.go`): `config → database → orm → middleware → api → security → event → cqrs → cron → redis → mold → storage → schema → monitor → mcp → app`.
- **Modules**: each exposes `fx.Module` in `internal/<module>/module.go`, constructors annotated into FX groups.
- **DI helpers** (`di.go`): `vef.ProvideAPIResource(...)`, `vef.ProvideMiddleware(...)`, `vef.ProvideSPAConfig(...)`, `vef.SupplySPAConfigs(...)`, `vef.ProvideCQRSBehavior(...)`, `vef.ProvideMCPTools(...)`, etc.

## API Patterns

- **Resources**: `api.NewRPCResource(name, api.WithOperations(...))` or `api.NewRESTResource(name, opts...)` with optional CRUD generics (`crud.FindAll[M,S]`, `crud.Create[M,P]`, etc.).
- **Registration**: `vef.ProvideAPIResource(constructor)`.
- **Handlers**: PascalCase auto-resolution (`Action: "create_user"` → `CreateUser`), or explicit `Handler` in `api.OperationSpec`.
- **Parameter binding**: sentinel types `api.P` (params) and `api.M` (meta), `search` tags for queries (`search:"eq"`, `search:"contains,column=name|description"`). Built-in resolvers: `fiber.Ctx`, `orm.DB`, `log.Logger`, `*security.Principal`, `mold.Transformer`. Custom: `group:"vef:api:handler_param_resolvers"`.
- **Response**: `result.Ok(data)`, `result.Err("msg", result.WithCode(code))`.

## Request Lifecycle (`/api`)

Request parsing → Authentication (JWT/signature/password) → Context enrichment (DB, logger, principal) → Authorization (`PermToken`) → Rate limiting (100 req/5min default) → Handler dispatch (30s timeout).  
RPC uses `POST /api`; REST routes are mounted under `/api/<resource>`.

## Data Access

- `orm.DB` for queries and `db.NewRaw(...)` for raw SQL when needed.
- Models embed `bun.BaseModel` (table tag) + `orm.FullAuditedModel` (id + audit: `created_at/by`, `updated_at/by`). Variants: `orm.Model` (id only), `orm.CreationTrackedModel` (creation audit, no id), `orm.FullTrackedModel` (full audit, no id), `orm.CreationAuditedModel` (id + creation audit). IDs: `id.Generate()` → 20-char XID.
- Transactions: `db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error { ... })`.
- Search: `search.Applier[T]` with struct tags.

## Security

- `security.Module`: JWT, password, OpenAPI authenticators + `AuthManager` aggregator.
- `security.Principal`: `Type`, `Id`, `Name`, `Roles`, `Details`. Config in `vef.security`.
- RBAC via `NewRBACPermissionChecker` + user-provided `RolePermissionsLoader`.

## Infrastructure Modules

| Module | Package | Key API |
|--------|---------|---------|
| Cache | `cache/` | `cache.NewMemory[T]()`, `cache.NewRedis[T]()` |
| Redis | `internal/redis` | `config.RedisConfig` |
| Events | `internal/event` | Memory bus, `group:"vef:event:middlewares"` |
| Cron | `internal/cron` | `gocron.Scheduler` via DI |
| Storage | `internal/storage` | `storage.Provider` (memory / MinIO). Object keys follow `pub/...` (anonymous-readable via direct backend URL — MinIO bucket policy grants `s3:GetObject` on `pub/*` only) or `priv/...` (authenticated reads via `/storage/files/<key>` proxy + `FileACL`). Richtext / markdown URL ↔ key translation runs through `storage.URLKeyMapper` (`URLToKey(url) (key, ok)` rejects non-managed URLs); the extractor is scheme-agnostic so http(s) URLs reach the mapper. |
| Schema | `internal/schema` | Schema inspection resource |
| Monitor | `internal/monitor` | Health check, build info |
| MCP | `internal/mcp` | MCP server, tools, prompts |
| Mold | `internal/mold` | `mold.Transformer` for data cleansing |

## Configuration

- `application.toml` from `./configs`, `./`, or `VEF_CONFIG_PATH`. Sections: `vef.app`, `vef.data_source`, `vef.cors`, `vef.security`, `vef.redis`, `vef.cache`, `vef.storage`.
- `config.Config.Unmarshal` with `config:""` struct tags. Env overrides: `VEF_CONFIG_PATH`, `VEF_LOG_LEVEL`, `VEF_NODE_ID`, `VEF_I18N_LANGUAGE`.

## Middleware Stack (by `Order()`)

Compression (`-1000`) → Headers (`-900`) → CORS (`-800`) → Content-Type (`-700`) → Request ID (`-650`) → Logger (`-600`) → Recovery (`-500`) → Recorder (`-100`) → [routes] → SPA (`+1000`). Custom: `vef.ProvideMiddleware(...)`.

## Development Conventions

- **Commits**: [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/). Single logical change per commit, message **strictly one line**, concise. No co-author trailers. Split by feature granularity. Types: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`. Scopes optional: `refactor(test):`, `feat(crud):`.
- **Releasing**: update `version/version.go` (`VEFVersion` constant) → commit with `chore: bump version to vX.Y.Z` → `git tag vX.Y.Z` on that commit → push. The tag must always point at the version-bump commit, not an earlier one.
- **Code style**: lean handlers (delegate to services), composable FX modules, `fx.Annotate` with precise tags.
- **Identifier naming**: when a name contains consecutive acronyms, keep the semantically more important acronym in standard form and Pascal-case the other for readability. Prefer `HTTPSUrl`, `HttpsURL`, `JSONApi`, or `JsonAPI`; avoid fully stacked forms like `HTTPSURL` or `JSONAPI`.
- **Unused parameters & receivers**: omit the name entirely for unused receivers (`func (*Type) Method(...)`) and for parameter lists where every entry is unused (`func F(context.Context, string) error`). Do **not** write `_` in those cases. Only fall back to `_` when at least one parameter in the list is used — Go syntax requires every entry in a list to be either all-named or all-unnamed, so a partially-unused list must keep `_` for the unused entries (e.g. `func (s *Svc) Get(_ context.Context, key string) (...)`).
- **Empty struct methods**: use pointer receivers (`func (*T) Method(...)`) even for zero-size structs, for consistency with the rest of the codebase.
- **Empty struct pointer initialization**: use `new(T)` instead of `&T{}` when constructing a pointer to a zero-value struct.- **Comments**: only for exported types, complex logic, and non-obvious details. Explain "what" and "why", not "how". **Interface methods must always have comments** — interfaces are extension points for framework users.
- **Test style**: use `testify/suite` only when lifecycle hooks or shared state are required; prefer simple table-driven tests for pure functions.
- **Test file layout**: tests for `foo.go` belong in `foo_test.go`. If tests span multiple source files, split them by source file. Put test helpers immediately after imports.
- **Test quality**: cover behavior, not just line coverage. Include boundary cases such as zero/default values, whitespace inputs, nil vs empty, option override precedence, and argument passthrough correctness when relevant.
- **Assertions**: suite tests must use instance methods such as `s.Require()` and `s.Equal()`. Non-suite tests use standalone `require` and `assert`. Every assertion needs a descriptive message.
- **Test naming**: never use underscores to encode sub-scenarios in test method names (`TestFoo_Bar`). Use nested subtests: `s.Run("Bar", func() { ... })` inside a single `TestFoo` method, producing clean hierarchies like `TestSuite/TestFoo/Bar`.
- **Interface placement**: interfaces live next to their most relevant code, never in a centralized `interfaces.go`. Place each interface in the file that defines its feature context (e.g., `PasswordChangeChecker` in `password_change.go`). When 3+ related interfaces form a sub-domain cluster, create a dedicated file (e.g., `permission.go`). Same rule applies to types — no centralized `types.go`.
- **Collections & Streams**: prefer `github.com/coldsmirk/go-collections` and `github.com/coldsmirk/go-streams` when they improve clarity. For validation or lookup sets, use `collections.NewHashSetFrom(...)` + `.Contains()` instead of `map[T]struct{}`. Keep a plain `for` loop when it is already the clearest option; streams are best for multi-step transforms, nested iteration, or error propagation.
- **Integration tests**: use `internal/apptest.NewTestApp` for app-level integration testing. For multi-database tests, only skip cases that are fundamentally impossible to simulate.

## Extensibility

- **DI**: `fx.Decorate` (wrap), `fx.Replace` (test overrides), `fx.Populate` (grab refs).
- **Event middleware**: `group:"vef:event:middlewares"`.
- **Rate limits & audit**: `OperationSpec.RateLimit`, `OperationSpec.EnableAudit` per endpoint.

## Gotchas

- `db.RunInTX` — uppercase TX, not `RunInTx`.
- `SupplySPAConfigs` — all caps SPA, not `SupplySpaConfigs`.
- `api.OperationSpec` — not `api.Spec`.
- CRUD embedding uses interface names as field names: `crud.FindAll[M,S]` not `*crud.FindAllApi`. Constructor: `crud.NewFindAll[M,S]()`.
- Boot sequence includes `schema` and `mcp` modules — often missed when listing module order.
- Import cycles: when package A imports B which imports A, move shared types to the lower-level package.
- No centralized `interfaces.go` or `types.go` — if found, refactor to co-locate with related code.
- Edit tool `replace_all` replaces ALL occurrences including strings/comments — scope carefully.
- `_test.go` types: always use exported PascalCase (`TestCmd`, not `testCmd`).
- Testify suite `TearDownTest`/`SetupTest` only run between top-level `Test*` methods — NOT between `s.Run()` subtests. Add per-subtest cleanup (e.g., `defer cleanup()`) for data isolation.

## CLI Tools

`cmd/vef-cli`: `generate-build-info` (build metadata), `create` (currently placeholder and returns not implemented).

- Keep Cobra wiring in `command.go`; move reusable generation logic into `generator.go` or `templates.go` only when it improves clarity.
- Multi-word command packages follow the existing snake_case style (for example `build_info`, `model_schema`), and the exported constructor is `Command() *cobra.Command`.

## Quick Reference

| Area | Location |
|------|----------|
| Entry point | `bootstrap.go`, `start.go`, `di.go` |
| API internals | `internal/api/*` |
| ORM | `internal/orm/*` |
| Security | `internal/security/*`, `security/` |
| Testing | `internal/apptest`, `crud/*_test.go` |
| Docs | `README.md`, `TESTING.md` |

When uncertain about a pattern, search the repo for existing usage and mirror it.

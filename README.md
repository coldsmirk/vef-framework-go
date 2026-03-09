# VEF Framework Go

📖 [English](./README.md) | [简体中文](./README.zh-CN.md)

[![GitHub Release](https://img.shields.io/github/v/release/coldsmirk/vef-framework-go)](https://github.com/coldsmirk/vef-framework-go/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/coldsmirk/vef-framework-go/test.yml?branch=main)](https://github.com/coldsmirk/vef-framework-go/actions/workflows/test.yml)
[![Coverage](https://img.shields.io/codecov/c/github/coldsmirk/vef-framework-go)](https://codecov.io/gh/coldsmirk/vef-framework-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/coldsmirk/vef-framework-go.svg)](https://pkg.go.dev/github.com/coldsmirk/vef-framework-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/coldsmirk/vef-framework-go)](https://goreportcard.com/report/github.com/coldsmirk/vef-framework-go)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/coldsmirk/vef-framework-go)
[![License](https://img.shields.io/github/license/coldsmirk/vef-framework-go)](https://github.com/coldsmirk/vef-framework-go/blob/main/LICENSE)

A modern Go web development framework built on Uber FX dependency injection and Fiber, designed for rapid enterprise application development with opinionated conventions and comprehensive built-in features.

> **Development Status**: VEF Framework Go is under active development and has not yet reached a stable 1.0 release. Breaking changes may occur as we refine best practices. Please exercise caution in production environments.

## Features

- **RPC + REST API Routing** - RPC requests via `POST /api`, REST requests via standard HTTP methods under `/api/<resource>`
- **Generic CRUD APIs** - Pre-built type-safe CRUD operations with minimal boilerplate
- **Type-Safe ORM** - Bun-based ORM with fluent query builder and automatic audit tracking
- **Multi-Strategy Authentication** - JWT, OpenAPI signature, and password authentication out of the box
- **Modular Design** - Uber FX dependency injection with pluggable modules
- **Built-in Features** - Cache, event bus, cron scheduler, object storage, data validation, i18n
- **RBAC & Data Permissions** - Row-level security with customizable data scopes

## Quick Start

### Installation

```bash
go get github.com/coldsmirk/vef-framework-go
```

**Requirements:** Go 1.25.0 or higher

**Troubleshooting:** If you encounter ambiguous import errors with `google.golang.org/genproto` during `go mod tidy`, run:

```bash
go get google.golang.org/genproto@latest
go mod tidy
```

### Minimal Example

Create `main.go`:

```go
package main

import "github.com/coldsmirk/vef-framework-go"

func main() {
    vef.Run()
}
```

Create `configs/application.toml`:

```toml
[vef.app]
name = "my-app"
port = 8080

[vef.data_source]
type = "postgres"
host = "localhost"
port = 5432
user = "postgres"
password = "password"
database = "mydb"
schema = "public"
```

Run the application:

```bash
go run main.go
```

Your API server is now running at `http://localhost:8080`.

## Project Structure

VEF applications follow a modular architecture where business domains are organized into self-contained modules.

```
my-app/
├── cmd/
│   └── server/
│       └── main.go           # Application entry - composes all modules
├── configs/
│   └── application.toml       # Configuration file
└── internal/
    ├── auth/                  # Authentication providers
    │   ├── module.go
    │   ├── user_loader.go
    │   └── user_info_loader.go
    ├── sys/                   # System/admin features
    │   ├── models/
    │   ├── payloads/
    │   ├── resources/
    │   ├── schemas/           # Generated from models (via vef-cli)
    │   └── module.go
    ├── [domain]/              # Business domains (e.g., order, inventory)
    │   ├── models/
    │   ├── payloads/
    │   ├── resources/
    │   ├── schemas/
    │   └── module.go
    ├── vef/                   # VEF framework integrations
    │   ├── module.go
    │   ├── build_info.go
    │   ├── *_subscriber.go
    │   └── *_loader.go
    └── web/                   # SPA frontend integration (optional)
        ├── dist/
        └── module.go
```

Each module exports a `vef.Module()` that encapsulates its dependencies. The main.go composes them:

```go
package main

import (
    "github.com/coldsmirk/vef-framework-go"
    "my-app/internal/auth"
    "my-app/internal/sys"
    ivef "my-app/internal/vef"
    "my-app/internal/web"
)

func main() {
    vef.Run(
        ivef.Module,     // Framework integrations
        web.Module,      // SPA serving (optional)
        auth.Module,     // Authentication providers
        sys.Module,      // System resources
        // Add your business domain modules here
    )
}
```

**Module definition example:**

```go
// internal/sys/module.go
package sys

import (
    "github.com/coldsmirk/vef-framework-go"
    "my-app/internal/sys/resources"
)

var Module = vef.Module(
    "app:sys",
    vef.ProvideAPIResource(resources.NewUserResource),
    vef.ProvideAPIResource(resources.NewRoleResource),
)
```

## Architecture

### RPC and REST Routing

VEF supports two routing strategies that can be used side by side:

- **RPC**: Single endpoint `POST /api` with a unified request/response format
- **REST**: Standard HTTP verbs under `/api/<resource>`. External apps can authenticate with OpenAPI signatures on these endpoints.

**RPC Request Format:**

```json
{
  "resource": "sys/user",
  "action": "find_page",
  "version": "v1",
  "params": {
    "keyword": "john"
  },
  "meta": {
    "page": 1,
    "size": 20
  }
}
```

**RPC Response Format:**

```json
{
  "code": 0,
  "message": "Success",
  "data": {
    "page": 1,
    "size": 20,
    "total": 100,
    "items": [...]
  }
}
```

**REST Example:** `GET /api/sys/user/page?page=1&size=20&keyword=john`

Params vs Meta:
- `params` carries business data (search filters, create/update fields). Define structs embedding `api.P`.
- `meta` carries request-level options (pagination, export format). Define structs embedding `api.M` (e.g., `page.Pageable`).
  - For REST, `params` can come from path/query/body and `meta` via `X-Meta-*` headers.

### Dependency Injection

VEF leverages Uber FX for dependency injection. Register components using helper functions:

```go
vef.Run(
    vef.ProvideAPIResource(NewUserResource),
    vef.Provide(NewUserService),
)
```

## Defining Models

All models should embed `orm.Model` for automatic audit field management:

```go
package models

import (
    "github.com/coldsmirk/vef-framework-go/null"
    "github.com/coldsmirk/vef-framework-go/orm"
)

type User struct {
    orm.BaseModel `bun:"table:sys_user,alias:su"`
    orm.Model

    Username string      `json:"username" validate:"required,alphanum,max=32" label:"Username"`
    Email    null.String `json:"email" validate:"omitempty,email,max=64" label:"Email"`
    IsActive bool        `json:"isActive"`
}
```

**Field Tags:** `bun` (ORM config), `json` (serialization), `validate` (validation rules), `label` (human-readable name for errors).

**Audit Fields** (automatically maintained by `orm.Model`):
- `id` - Primary key (20-character XID in base32 encoding)
- `created_at`, `created_by` - Creation timestamp and user ID
- `created_by_name` - Creator name (scan-only, not stored in database)
- `updated_at`, `updated_by` - Last update timestamp and user ID
- `updated_by_name` - Updater name (scan-only, not stored in database)

Note: Database columns use snake_case (e.g., `created_at`), JSON fields use camelCase (e.g., `createdAt`).

**Null Types:** Use `null.String`, `null.Int`, `null.Bool`, etc. for nullable fields.

### Field Types for Boolean Columns

| Use Case | Preferred Type | Database Column |
|----------|----------------|-----------------|
| Non-nullable boolean on DBs with native boolean | `bool` | boolean |
| Nullable boolean (tri-state) | `null.Bool` | boolean or smallint/tinyint |
| Target DB without native boolean or require numeric 0/1 | `sql.Bool` (non-null) / `null.Bool` (nullable) | smallint/tinyint |
| Go-only/computed (not stored) | `bool` with `bun:"-"` | N/A |

```go
type User struct {
    orm.Model
    IsActive        bool      `json:"isActive"`                                    // Native boolean (recommended)
    IsLocked        sql.Bool  `json:"isLocked" bun:"type:smallint,notnull,default:0"` // Numeric 0/1 for compatibility
    IsEmailVerified null.Bool `json:"isEmailVerified" bun:"type:smallint"`         // Tri-state: NULL/0/1
    HasPermissions  bool      `json:"hasPermissions" bun:"-"`                      // Computed, not stored
}
```

`null.Bool` tri-state: `{Valid: false}` → NULL, `{Valid: true, Bool: false}` → 0, `{Valid: true, Bool: true}` → 1.

## Building CRUD APIs

### Step 1: Define Parameter Structures

**Search Parameters:**

```go
package payloads

import "github.com/coldsmirk/vef-framework-go/api"

type UserSearch struct {
    api.P
    Keyword  string `json:"keyword" search:"contains,column=username|email"`
    IsActive *bool  `json:"isActive" search:"eq"`
}
```

**Create/Update Parameters:**

```go
type UserParams struct {
    api.P
    ID       string      `json:"id"` // Required for updates
    Username string      `json:"username" validate:"required,alphanum,max=32" label:"Username"`
    Email    null.String `json:"email" validate:"omitempty,email,max=64" label:"Email"`
    IsActive bool        `json:"isActive"`
}
```

**Separate Create and Update Parameters** (when validation differs):

```go
type UserParams struct {
    api.P
    ID       string
    Username string      `json:"username" validate:"required,alphanum,max=32" label:"Username"`
    Email    null.String `json:"email" validate:"omitempty,email,max=64" label:"Email"`
    IsActive bool        `json:"isActive"`
}

type UserCreateParams struct {
    UserParams      `json:",inline"`
    Password        string `json:"password" validate:"required,min=6,max=16" label:"Password"`
    PasswordConfirm string `json:"passwordConfirm" validate:"required,eqfield=Password" label:"Confirm Password"`
}

type UserUpdateParams struct {
    UserParams      `json:",inline"`
    Password        null.String `json:"password" validate:"omitempty,min=6,max=16" label:"Password"`
    PasswordConfirm null.String `json:"passwordConfirm" validate:"omitempty,eqfield=Password" label:"Confirm Password"`
}
```

### Step 2: Create API Resource

```go
package resources

import (
    "github.com/coldsmirk/vef-framework-go/api"
    "github.com/coldsmirk/vef-framework-go/crud"
)

type UserResource struct {
    api.Resource
    crud.FindAll[models.User, payloads.UserSearch]
    crud.FindPage[models.User, payloads.UserSearch]
    crud.Create[models.User, payloads.UserParams]
    crud.Update[models.User, payloads.UserParams]
    crud.Delete[models.User]
}

func NewUserResource() api.Resource {
    return &UserResource{
        Resource: api.NewRPCResource("smp/sys/user"),
        FindAll:  crud.NewFindAll[models.User, payloads.UserSearch](),
        FindPage: crud.NewFindPage[models.User, payloads.UserSearch](),
        Create:   crud.NewCreate[models.User, payloads.UserParams](),
        Update:   crud.NewUpdate[models.User, payloads.UserParams](),
        Delete:   crud.NewDelete[models.User](),
    }
}
```

**Resource naming:** Use `{app}/{domain}/{entity}` pattern (e.g., `smp/sys/user`). RPC uses `snake_case`, REST uses `kebab-case`. Reserved namespaces: `security/auth`, `sys/storage`, `sys/monitor`.

### Step 3: Register Resource

```go
func main() {
    vef.Run(
        vef.ProvideAPIResource(resources.NewUserResource),
    )
}
```

### Pre-built APIs

| API | Description | Action |
|-----|-------------|--------|
| FindOne | Find single record | find_one |
| FindAll | Find all records | find_all |
| FindPage | Paginated query | find_page |
| Create | Create record | create |
| Update | Update record | update |
| Delete | Delete record | delete |
| CreateMany | Batch create | create_many |
| UpdateMany | Batch update | update_many |
| DeleteMany | Batch delete | delete_many |
| FindTree | Hierarchical query | find_tree |
| FindOptions | Options list (label/value) | find_options |
| FindTreeOptions | Tree options | find_tree_options |
| Import | Import from Excel/CSV | import |
| Export | Export to Excel/CSV | export |

**Note:** The actions above are **RPC** action names. For **REST** resources, actions are expressed as HTTP methods and sub-paths (e.g., `GET /`, `GET /page`, `POST /`, `PUT /:id`).

### API Builder Methods

Configure API behavior with fluent builder methods:

```go
Create: crud.NewCreate[User, UserParams]().
    Action("create_user").             // Custom action name
    Public().                          // No authentication required
    PermToken("sys.user.create").      // Permission token
    EnableAudit().                     // Enable audit logging
    Timeout(10 * time.Second).         // Request timeout
    RateLimit(10, 1*time.Minute).      // 10 requests per minute
```

### Find Operation Configuration Methods

All find operation types (FindOne, FindAll, FindPage, FindTree, FindOptions, FindTreeOptions, Export) support a unified query configuration system.

| Method | Description |
|--------|-------------|
| `WithProcessor` | Post-processing function for query results |
| `WithSelect` / `WithSelectAs` | Add columns to SELECT clause |
| `WithDefaultSort` | Set default sorting |
| `WithCondition` | Add WHERE condition using ConditionBuilder |
| `WithRelation` | Add relation join |
| `WithAuditUserNames` | Fetch audit user names (created_by_name, updated_by_name) |
| `WithQueryApplier` | Custom query applier function |
| `DisableDataPerm` | Disable data permission filtering |

**WithProcessor** - Transform results after query, before returning to client:

```go
FindAll: crud.NewFindAll[User, UserSearch]().
    WithProcessor(func(users []User, search UserSearch, ctx fiber.Ctx) any {
        for i := range users {
            users[i].Password = "***"
        }
        return users
    }),
```

**WithDefaultSort:**

```go
FindPage: crud.NewFindPage[User, UserSearch]().
    WithDefaultSort(&sortx.OrderSpec{
        Column:    "created_at",
        Direction: sortx.OrderDesc,
    }),
```

**WithCondition:**

```go
FindAll: crud.NewFindAll[User, UserSearch]().
    WithCondition(func(cb orm.ConditionBuilder) {
        cb.Equals("is_deleted", false)
        cb.Equals("is_active", true)
    }),
```

**WithRelation:**

```go
FindAll: crud.NewFindAll[User, UserSearch]().
    WithRelation(&orm.RelationSpec{
        Model: (*Profile)(nil),
        SelectedColumns: []orm.ColumnInfo{
            {Name: "name", AutoAlias: true},
            {Name: "email", AutoAlias: true},
        },
    }),
```

**WithAuditUserNames:**

```go
FindAll: crud.NewFindAll[User, UserSearch]().
    WithAuditUserNames(&User{}),           // Uses "name" column by default
    // Or: WithAuditUserNames(&User{}, "username")
```

#### QueryPart System (for Tree Queries)

The `parts` parameter specifies which part of the query an option applies to. This matters for tree APIs using recursive CTEs.

| QueryPart | Description |
|-----------|-------------|
| `QueryRoot` | Outer/root query (sorting, final filtering) |
| `QueryBase` | Base query in CTE (initial conditions, starting nodes) |
| `QueryRecursive` | Recursive query in CTE (traversal configuration) |
| `QueryAll` | All query parts |

**Tree query example:**

```go
FindTree: crud.NewFindTree[Category, CategorySearch](buildTree).
    WithSelect("sort", crud.QueryBase, crud.QueryRecursive).
    WithCondition(func(cb orm.ConditionBuilder) {
        cb.IsNull("parent_id")
    }, crud.QueryBase).
    WithCondition(func(cb orm.ConditionBuilder) {
        cb.Equals("is_active", true)
    }, crud.QueryRecursive).
    WithDefaultSort(&sortx.OrderSpec{
        Column:    "sort",
        Direction: sortx.OrderAsc,
    }),
```

#### API-Specific Configuration

**FindPage:**

```go
FindPage: crud.NewFindPage[User, UserSearch]().
    WithDefaultPageSize(20),
```

**FindOptions:**

```go
FindOptions: crud.NewFindOptions[User, UserSearch]().
    WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
        LabelColumn:       "name",
        ValueColumn:       "id",
        DescriptionColumn: "description",
    }),
```

**FindTree:**

```go
import "github.com/coldsmirk/vef-framework-go/tree"

FindTree: crud.NewFindTree[models.Organization, payloads.OrganizationSearch](
    buildOrganizationTree,
).
    WithIDColumn("id").
    WithParentIDColumn("parent_id").
    WithDefaultSort(&sortx.OrderSpec{
        Column:    "sort_order",
        Direction: sortx.OrderAsc,
    })

func buildOrganizationTree(flatModels []models.Organization) []models.Organization {
    return tree.Build(
        flatModels,
        tree.Adapter[models.Organization]{
            GetID:       func(m models.Organization) string { return m.ID },
            GetParentID: func(m models.Organization) string { return m.ParentID.ValueOrZero() },
            SetChildren: func(m *models.Organization, children []models.Organization) {
                m.Children = children
            },
        },
    )
}
```

Your model needs a parent ID field (typically `null.String`) and a children field (`bun:"-"`):

```go
type Organization struct {
    orm.Model
    Name     string          `json:"name"`
    ParentID null.String     `json:"parentID" bun:"type:varchar(20)"`
    Children []Organization  `json:"children" bun:"-"`
}
```

**Export:**

```go
Export: crud.NewExport[User, UserSearch]().
    WithDefaultFormat("excel").
    WithPreExport(func(users []User, search UserSearch, ctx fiber.Ctx, db orm.DB) error {
        for i := range users {
            users[i].Password = "***"
        }
        return nil
    }).
    WithFilenameBuilder(func(search UserSearch, ctx fiber.Ctx) string {
        return fmt.Sprintf("users_%s", time.Now().Format("20060102"))
    }),
```

### Pre/Post Hooks

Add custom business logic before/after CRUD operations:

```go
Create: crud.NewCreate[User, UserParams]().
    WithPreCreate(func(model *User, params *UserParams, ctx fiber.Ctx, db orm.DB) error {
        hashed, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
        if err != nil {
            return err
        }
        model.Password = string(hashed)
        return nil
    }).
    WithPostCreate(func(model *User, params *UserParams, ctx fiber.Ctx, tx orm.DB) error {
        return sendWelcomeEmail(model.Email) // Runs in transaction
    }),
```

Available hooks:

- **Single record:** `WithPreCreate`/`WithPostCreate`, `WithPreUpdate`/`WithPostUpdate` (receives old + new model), `WithPreDelete`/`WithPostDelete`
- **Batch:** `WithPreCreateMany`/`WithPostCreateMany`, `WithPreUpdateMany`/`WithPostUpdateMany`, `WithPreDeleteMany`/`WithPostDeleteMany`
- **Import/Export:** `WithPreImport`/`WithPostImport`, `WithPreExport`

All `Post*` hooks run within the transaction.

### Custom Handlers

#### Mixing Generated and Custom APIs

Combine CRUD APIs with custom actions using `api.WithOperations()`. For **RPC**, the handler is resolved by mapping `action` (snake_case) to a PascalCase method (e.g., `find_role_permissions` → `FindRolePermissions`). For **REST**, `OperationSpec.Handler` is required.

```go
type RoleResource struct {
    api.Resource
    crud.FindPage[models.Role, payloads.RoleSearch]
    crud.Create[models.Role, payloads.RoleParams]
    crud.Update[models.Role, payloads.RoleParams]
    crud.Delete[models.Role]
}

func NewRoleResource() api.Resource {
    return &RoleResource{
        Resource: api.NewRPCResource(
            "app/sys/role",
            api.WithOperations(
                api.OperationSpec{Action: "find_role_permissions"},
                api.OperationSpec{Action: "save_role_permissions", EnableAudit: true},
            ),
        ),
        FindPage: crud.NewFindPage[models.Role, payloads.RoleSearch](),
        Create:   crud.NewCreate[models.Role, payloads.RoleParams](),
        Update:   crud.NewUpdate[models.Role, payloads.RoleParams](),
        Delete:   crud.NewDelete[models.Role](),
    }
}

func (r *RoleResource) FindRolePermissions(ctx fiber.Ctx, db orm.DB, params payloads.RolePermissionQuery) error {
    // Custom business logic
    return result.Ok(permissions).Response(ctx)
}

func (r *RoleResource) SaveRolePermissions(ctx fiber.Ctx, db orm.DB, params payloads.RolePermissionParams) error {
    return db.RunInTx(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
        // Save permissions in transaction
        return nil
    })
}
```

#### REST Resource Example

```go
func NewRoleRestResource() api.Resource {
    return &RoleRestResource{
        Resource: api.NewRESTResource("sys/role", api.WithOperations(
            api.OperationSpec{Action: "get /:id", Handler: "GetRole"},
            api.OperationSpec{Action: "post /", Handler: "CreateRole"},
        )),
    }
}
```

#### Injectable Parameters

Handler methods support automatic parameter injection:

- `fiber.Ctx`, `orm.DB`, `log.Logger`, `mold.Transformer`, `*security.Principal`, `page.Pageable`
- Custom structs embedding `api.P` (params) or `api.M` (meta)
- Resource struct fields (direct fields, `api:"in"` tagged fields, or embedded structs)

If your service implements `log.LoggerConfigurable[T]`, the framework auto-injects a request-scoped logger via `WithLogger`.

## Database Operations

### Query Builder

```go
var users []models.User
err := db.NewSelect().
    Model(&users).
    Where(func(cb orm.ConditionBuilder) {
        cb.Equals("is_active", true)
        cb.GreaterThan("age", 18)
        cb.Contains("username", keyword)
    }).
    Relation("Profile").
    OrderByDesc("created_at").
    Limit(10).
    Scan(ctx)
```

### Condition Builder Methods

- `Equals`, `NotEquals` - Equality
- `GreaterThan`, `GreaterThanOrEquals`, `LessThan`, `LessThanOrEquals` - Comparison
- `Contains`, `StartsWith`, `EndsWith` - String matching (LIKE)
- `In`, `Between` - Range/set
- `IsNull`, `IsNotNull` - Null checks
- `Or(conditions...)` - OR multiple conditions

### Search Tags

Automatically apply query conditions using `search` tags:

```go
type UserSearch struct {
    api.P
    Username string `search:"eq"`                                    // username = ?
    Email    string `search:"contains"`                              // email LIKE ?
    Age      int    `search:"gte"`                                   // age >= ?
    Status   string `search:"in"`                                    // status IN (?)
    Keyword  string `search:"contains,column=username|email|name"`   // Search multiple columns
}
```

**Supported operators:**

| Tag | SQL | Tag | SQL |
|-----|-----|-----|-----|
| `eq` | = | `neq` | != |
| `gt` | > | `gte` | >= |
| `lt` | < | `lte` | <= |
| `between` | BETWEEN | `notBetween` | NOT BETWEEN |
| `in` | IN | `notIn` | NOT IN |
| `isNull` | IS NULL | `isNotNull` | IS NOT NULL |
| `contains` | LIKE %?% | `notContains` | NOT LIKE %?% |
| `startsWith` | LIKE ?% | `endsWith` | LIKE %? |
| `iContains` | ILIKE %?% | `iStartsWith` | ILIKE ?% |

Case-insensitive variants: prefix with `i` (e.g., `iContains`, `iEndsWith`). Negation variants: prefix with `not` (e.g., `notStartsWith`, `iNotContains`).

### Transactions

```go
err := db.RunInTx(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
    _, err := tx.NewInsert().Model(&user).Exec(txCtx)
    if err != nil {
        return err // Auto-rollback
    }
    _, err = tx.NewUpdate().Model(&profile).WherePK().Exec(txCtx)
    return err // Auto-commit on nil, rollback on error
})
```

## Authentication & Authorization

### Authentication Methods

VEF supports multiple authentication strategies:

1. **JWT Authentication** (default) - Bearer token or query parameter `?__accessToken=xxx`
2. **OpenAPI Signature** - For external applications using HMAC signature
3. **Password Authentication** - Username/password login

### Implementing User Loader

Implement `security.UserLoader` to integrate with your user system:

```go
type MyUserLoader struct {
    db orm.DB
}

func (l *MyUserLoader) LoadByUsername(ctx context.Context, username string) (*security.Principal, string, error) {
    var user models.User
    if err := l.db.NewSelect().
        Model(&user).
        Where(func(cb orm.ConditionBuilder) {
            cb.Equals("username", username)
        }).
        Scan(ctx); err != nil {
        return nil, "", err
    }

    principal := &security.Principal{
        Type:  security.PrincipalTypeUser,
        ID:    user.ID,
        Name:  user.Name,
        Roles: []string{"user"},
    }

    return principal, user.Password, nil
}

func (l *MyUserLoader) LoadById(ctx context.Context, id string) (*security.Principal, error) {
    // Similar implementation
}

// Register in main.go
func main() {
    vef.Run(vef.Provide(NewMyUserLoader))
}
```

### Permission Control

Set permission tokens on APIs:

```go
Create: crud.NewCreate[User, UserParams]().
    PermToken("sys.user.create"),
```

#### Using Built-in RBAC (Recommended)

Implement `security.RolePermissionsLoader` to enable RBAC:

```go
type MyRolePermissionsLoader struct {
    db orm.DB
}

func (l *MyRolePermissionsLoader) LoadPermissions(ctx context.Context, role string) (map[string]security.DataScope, error) {
    var permissions []RolePermission
    if err := l.db.NewSelect().
        Model(&permissions).
        Where(func(cb orm.ConditionBuilder) {
            cb.Equals("role_code", role)
        }).
        Scan(ctx); err != nil {
        return nil, err
    }

    result := make(map[string]security.DataScope)
    for _, perm := range permissions {
        switch perm.DataScopeType {
        case "all":
            result[perm.PermissionToken] = security.NewAllDataScope()
        case "self":
            result[perm.PermissionToken] = security.NewSelfDataScope("")
        }
    }
    return result, nil
}

// Register in main.go
func main() {
    vef.Run(vef.Provide(NewMyRolePermissionsLoader))
}
```

The framework automatically uses your `RolePermissionsLoader` to initialize the RBAC permission checker and data permission resolver.

#### Custom Permission Control

For non-RBAC scenarios, implement `security.PermissionChecker` and replace via `vef.Replace(vef.Annotate(..., vef.As(new(security.PermissionChecker))))`.

### Data Permissions

Data permissions implement row-level access control. Built-in scopes:

- **AllDataScope** - Unrestricted access (for administrators)
- **SelfDataScope** - Access only self-created data

```go
allScope := security.NewAllDataScope()
selfScope := security.NewSelfDataScope("")       // Defaults to created_by column
selfScope := security.NewSelfDataScope("creator_id") // Custom column
```

In your `RolePermissionsLoader`, assign scopes per permission:

```go
result["sys.user.view"] = security.NewAllDataScope()
result["sys.user.edit"] = security.NewSelfDataScope("")
```

**Data Scope Priority** (when user has multiple roles): `PrioritySelf` (10) < `PriorityDepartment` (20) < `PriorityDepartmentAndSub` (30) < `PriorityOrganization` (40) < `PriorityOrganizationAndSub` (50) < `PriorityCustom` (60) < `PriorityAll` (10000). The highest priority scope wins.

For custom data scopes, implement the `security.DataScope` interface (`Key()`, `Priority()`, `Supports()`, `Apply()`).

## Configuration

### Configuration File

Place `application.toml` in `./configs/` or `./` directory, or specify via `VEF_CONFIG_PATH` environment variable.

```toml
[vef.app]
name = "my-app"          # Application name
port = 8080              # HTTP port
body_limit = "10MB"      # Request body size limit

[vef.data_source]
type = "postgres"        # Database type: postgres, mysql, sqlite, oracle, sqlserver
host = "localhost"
port = 5432
user = "postgres"
password = "password"
database = "mydb"
schema = "public"        # PostgreSQL schema
# path = "./data.db"    # SQLite database file path

[vef.security]
token_expires = "2h"     # JWT token expiration time

[vef.storage]
provider = "minio"       # Storage provider: memory, filesystem, minio (default: memory)

[vef.storage.minio]
endpoint = "localhost:9000"
access_key = "minioadmin"
secret_key = "minioadmin"
use_ssl = false
region = "us-east-1"
bucket = "mybucket"

[vef.storage.filesystem]
root = "./storage"

[vef.redis]
host = "localhost"
port = 6379
user = ""
password = ""
database = 0
network = "tcp"

[vef.cors]
enabled = true
allow_origins = ["*"]
```

### Environment Variables

- `VEF_CONFIG_PATH` - Configuration file path
- `VEF_LOG_LEVEL` - Log level (debug, info, warn, error)
- `VEF_NODE_ID` - XID node identifier for ID generation
- `VEF_I18N_LANGUAGE` - Language (en, zh-CN)

## More Features

### Cache

```go
import "github.com/coldsmirk/vef-framework-go/cache"

// In-memory cache
memCache := cache.NewMemory[models.User](
    cache.WithMemMaxSize(1000),
    cache.WithMemDefaultTTL(5 * time.Minute),
)

// Redis cache
redisCache := cache.NewRedis[models.User](
    redisClient, "users",
    cache.WithRdsDefaultTTL(10 * time.Minute),
)

// Usage
user, err := memCache.GetOrLoad(ctx, "user:123", func(ctx context.Context) (models.User, error) {
    return loadUserFromDB(ctx, "123")
})
```

### Event Bus

```go
import "github.com/coldsmirk/vef-framework-go/event"

// Publishing
bus.Publish(event.NewBaseEvent("user.created",
    event.WithSource("user-service"),
    event.WithMeta("userID", user.ID),
))

// Subscribing
vef.Invoke(func(bus event.Bus, logger log.Logger) {
    unsubscribe := bus.Subscribe("user.created", func(ctx context.Context, e event.Event) {
        logger.Infof("User created: %s", e.Meta()["userID"])
    })
    _ = unsubscribe
})
```

### Cron Scheduler

Based on [gocron](https://github.com/go-co-op/gocron):

```go
import "github.com/coldsmirk/vef-framework-go/cron"

vef.Invoke(func(scheduler cron.Scheduler) {
    // Cron expression (5-field: min hour day month weekday)
    scheduler.NewJob(cron.NewCronJob(
        "0 0 * * *", false,
        cron.WithName("daily-cleanup"),
        cron.WithTask(func(ctx context.Context) {
            // Task logic
        }),
    ))

    // Fixed interval
    scheduler.NewJob(cron.NewDurationJob(
        5*time.Minute,
        cron.WithName("health-check"),
        cron.WithTask(func() {
            // Every 5 minutes
        }),
    ))
})
```

Job options: `WithTags(...)`, `WithConcurrent()`, `WithStartImmediately()`, `WithStartAt(t)`, `WithStopAt(t)`, `WithLimitedRuns(n)`.

### File Storage

Built-in `sys/storage` resource provides: `upload`, `get_presigned_url`, `delete_temp`, `stat`, `list`.

**Custom file upload:**

```go
func (r *UserResource) UploadAvatar(
    ctx fiber.Ctx,
    service storage.Service,
    params UploadAvatarParams,
) error {
    reader, err := params.File.Open()
    if err != nil {
        return err
    }
    defer reader.Close()

    info, err := service.PutObject(ctx.Context(), storage.PutObjectOptions{
        Key:         "avatars/" + params.File.Filename,
        Reader:      reader,
        Size:        params.File.Size,
        ContentType: params.File.Header.Get("Content-Type"),
    })
    if err != nil {
        return err
    }
    return result.Ok(info).Response(ctx)
}
```

Configure storage provider in `application.toml` with `vef.storage.provider` set to `minio`, `filesystem`, or `memory` (default).

### Data Validation

Use [go-playground/validator](https://github.com/go-playground/validator) tags:

```go
type UserParams struct {
    Username string `validate:"required,alphanum,min=3,max=32" label:"Username"`
    Email    string `validate:"required,email" label:"Email"`
    Age      int    `validate:"min=18,max=120" label:"Age"`
    Password string `validate:"required,min=8,containsany=!@#$%^&*" label:"Password"`
}
```

Common rules: `required`, `omitempty`, `min`, `max`, `len`, `email`, `url`, `uuid`, `alpha`, `alphanum`, `numeric`, `contains`, `startswith`, `endswith`.

### CLI Tools

**Generate Build Info:**

```bash
go run github.com/coldsmirk/vef-framework-go/cmd/vef-cli@latest generate-build-info -o internal/vef/build_info.go -p vef
```

**Generate Model Schema** (type-safe field accessors):

```bash
go run github.com/coldsmirk/vef-framework-go/cmd/vef-cli@latest generate-model-schema -i ./models -o ./schemas -p schemas
```

Usage in queries:

```go
import "my-app/internal/sys/schemas"

db.NewSelect().Model(&users).
    Where(func(cb orm.ConditionBuilder) {
        cb.Equals(schemas.User.Username(), "admin")
    }).
    OrderBy(schemas.User.CreatedAt(true) + " DESC").
    Scan(ctx)
```

### Lifecycle Hooks

Use `vef.Lifecycle` for startup/shutdown hooks, essential for resource cleanup:

```go
vef.Invoke(func(lc vef.Lifecycle, subscriber event.Subscriber) {
    auditSub := NewAuditEventSubscriber(subscriber)
    lc.Append(vef.StopHook(func() {
        auditSub.Unsubscribe()
    }))
})
```

### Context Helpers

The `contextx` package provides access to request-scoped resources when DI is not available:

- `contextx.DB(ctx)` - Request-scoped `orm.DB` with audit fields pre-configured
- `contextx.Principal(ctx)` - Current `*security.Principal`
- `contextx.Logger(ctx)` - Request-scoped `log.Logger` with request ID
- `contextx.DataPermApplier(ctx)` - Data permission applier

Prefer parameter injection in handler methods; use `contextx` in utility functions that only receive `fiber.Ctx`.

## Best Practices

### Naming Conventions

- **Models:** Singular PascalCase (e.g., `User`, `Order`)
- **Resources:** Lowercase with slashes (e.g., `sys/user`, `shop/order`)
- **Parameters:** `XxxParams` (Create/Update), `XxxSearch` (Query)
- **Actions:** Lowercase snake_case (e.g., `find_page`, `create_user`)

### Error Handling

```go
import "github.com/coldsmirk/vef-framework-go/result"

return result.Ok(data).Response(ctx)
return result.Err("Operation failed")
return result.Err("Invalid parameters", result.WithCode(result.ErrCodeBadRequest))
return result.Errf("User %s not found", username)
```

### Logging

```go
func (r *UserResource) Handler(ctx fiber.Ctx, logger log.Logger) error {
    logger.Infof("Processing request from %s", ctx.IP())
    return nil
}
```

## Documentation & Resources

- [Fiber Web Framework](https://gofiber.io/) - Underlying HTTP framework
- [Bun ORM](https://bun.uptrace.dev/) - Database ORM
- [Go Playground Validator](https://github.com/go-playground/validator) - Data validation
- [Uber FX](https://uber-go.github.io/fx/) - Dependency injection

## License

This project is licensed under the [Apache License 2.0](LICENSE).

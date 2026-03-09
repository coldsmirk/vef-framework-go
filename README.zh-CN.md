# VEF Framework Go

📖 [English](./README.md) | [简体中文](./README.zh-CN.md)

[![GitHub Release](https://img.shields.io/github/v/release/coldsmirk/vef-framework-go)](https://github.com/coldsmirk/vef-framework-go/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/coldsmirk/vef-framework-go/test.yml?branch=main)](https://github.com/coldsmirk/vef-framework-go/actions/workflows/test.yml)
[![Coverage](https://img.shields.io/codecov/c/github/coldsmirk/vef-framework-go)](https://codecov.io/gh/coldsmirk/vef-framework-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/coldsmirk/vef-framework-go.svg)](https://pkg.go.dev/github.com/coldsmirk/vef-framework-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/coldsmirk/vef-framework-go)](https://goreportcard.com/report/github.com/coldsmirk/vef-framework-go)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/coldsmirk/vef-framework-go)
[![License](https://img.shields.io/github/license/coldsmirk/vef-framework-go)](https://github.com/coldsmirk/vef-framework-go/blob/main/LICENSE)

一个基于 Uber FX 依赖注入和 Fiber 构建的现代化 Go Web 开发框架，采用约定优于配置的设计理念，为企业级应用快速开发提供开箱即用的完整功能。

> **开发状态**：VEF Framework Go 正处于积极开发阶段，尚未发布稳定的 1.0 版本。在不断完善最佳实践的过程中可能会出现破坏性更新。在生产环境中使用请务必谨慎。

## 核心特性

- **RPC + REST API 路由** - RPC 通过 `POST /api`，REST 通过标准 HTTP 方法访问 `/api/<resource>`
- **泛型 CRUD API** - 预置类型安全的增删改查操作，极少样板代码
- **类型安全的 ORM** - 基于 Bun 的流式查询构建器，自动审计字段维护
- **多策略认证** - 内置 JWT、OpenAPI 签名、密码认证，开箱即用
- **模块化设计** - Uber FX 依赖注入，可插拔模块化架构
- **内置功能齐全** - 缓存、事件总线、定时任务、对象存储、数据验证、国际化
- **RBAC 与数据权限** - 行级安全控制，可自定义数据范围

## 快速开始

### 安装

```bash
go get github.com/coldsmirk/vef-framework-go
```

**环境要求：** Go 1.25.0 或更高版本

**问题排查：** 如果在执行 `go mod tidy` 时遇到 `google.golang.org/genproto` 的模糊依赖错误，请运行：

```bash
go get google.golang.org/genproto@latest
go mod tidy
```

### 最小示例

创建 `main.go`：

```go
package main

import "github.com/coldsmirk/vef-framework-go"

func main() {
    vef.Run()
}
```

创建 `configs/application.toml`：

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

运行应用：

```bash
go run main.go
```

您的 API 服务现已运行在 `http://localhost:8080`。

## 项目结构

VEF 应用程序遵循模块化架构模式，将业务领域组织成独立的模块。

```
my-app/
├── cmd/
│   └── server/
│       └── main.go           # 应用入口 - 组合所有模块
├── configs/
│   └── application.toml       # 配置文件
└── internal/
    ├── auth/                  # 认证提供者
    │   ├── module.go
    │   ├── user_loader.go
    │   └── user_info_loader.go
    ├── sys/                   # 系统/管理功能
    │   ├── models/
    │   ├── payloads/
    │   ├── resources/
    │   ├── schemas/           # 从模型生成（通过 vef-cli）
    │   └── module.go
    ├── [domain]/              # 业务领域（如 order、inventory）
    │   ├── models/
    │   ├── payloads/
    │   ├── resources/
    │   ├── schemas/
    │   └── module.go
    ├── vef/                   # VEF 框架集成
    │   ├── module.go
    │   ├── build_info.go
    │   ├── *_subscriber.go
    │   └── *_loader.go
    └── web/                   # SPA 前端集成（可选）
        ├── dist/
        └── module.go
```

每个模块导出一个 `vef.Module()`，封装其依赖和资源。main.go 组合这些模块：

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
        ivef.Module,     // 框架集成
        web.Module,      // SPA 服务（可选）
        auth.Module,     // 认证提供者
        sys.Module,      // 系统资源
        // 在此添加您的业务领域模块
    )
}
```

**模块定义示例：**

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

## 架构设计

### RPC 与 REST 路由

VEF 支持两种路由策略，可同时使用：

- **RPC**：单一端点 `POST /api`，统一请求/响应格式
- **REST**：标准 HTTP 方法访问 `/api/<resource>`。外部应用可通过 OpenAPI 签名认证。

**RPC 请求格式：**

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

**RPC 响应格式：**

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "page": 1,
    "size": 20,
    "total": 100,
    "items": [...]
  }
}
```

**REST 示例：** `GET /api/sys/user/page?page=1&size=20&keyword=john`

参数与元数据：
- `params`：业务参数（查询筛选、创建/更新字段）。定义的结构体需嵌入 `api.P`。
- `meta`：请求级控制信息（分页、导入导出格式等）。定义的结构体需嵌入 `api.M`（如 `page.Pageable`）。
  - 在 REST 下，`params` 可来自 path/query/body，`meta` 可通过 `X-Meta-*` 请求头传入。

### 依赖注入

VEF 使用 Uber FX 进行依赖注入。通过辅助函数注册组件：

```go
vef.Run(
    vef.ProvideAPIResource(NewUserResource),
    vef.Provide(NewUserService),
)
```

## 定义数据模型

所有模型应嵌入 `orm.Model` 以获得自动审计字段管理：

```go
package models

import (
    "github.com/coldsmirk/vef-framework-go/null"
    "github.com/coldsmirk/vef-framework-go/orm"
)

type User struct {
    orm.BaseModel `bun:"table:sys_user,alias:su"`
    orm.Model

    Username string      `json:"username" validate:"required,alphanum,max=32" label:"用户名"`
    Email    null.String `json:"email" validate:"omitempty,email,max=64" label:"邮箱"`
    IsActive bool        `json:"isActive"`
}
```

**字段标签：** `bun`（ORM 配置）、`json`（序列化）、`validate`（验证规则）、`label`（错误消息中的字段名）。

**审计字段**（`orm.Model` 自动维护）：
- `id` - 主键（20 字符的 XID，base32 编码）
- `created_at`、`created_by` - 创建时间戳和用户 ID
- `created_by_name` - 创建者名称（仅扫描，不存储到数据库）
- `updated_at`、`updated_by` - 最后更新时间戳和用户 ID
- `updated_by_name` - 更新者名称（仅扫描，不存储到数据库）

说明：数据库列名使用下划线命名（如 `created_at`），JSON 字段使用驼峰命名（如 `createdAt`）。

**可空类型：** 使用 `null.String`、`null.Int`、`null.Bool` 等处理可空字段。

### 布尔列的字段类型

| 使用场景 | 首选类型 | 数据库列类型 |
|---------|----------|--------------|
| 数据库原生布尔、非空列 | `bool` | boolean |
| 可空布尔（三态） | `null.Bool` | boolean 或 smallint/tinyint |
| 兼容无布尔数据库，或强制数值存储 0/1 | `sql.Bool`（非空）/ `null.Bool`（可空） | smallint/tinyint |
| 仅 Go 计算字段（不入库） | `bool` 且 `bun:"-"` | N/A |

```go
type User struct {
    orm.Model
    IsActive        bool      `json:"isActive"`                                    // 原生布尔（推荐）
    IsLocked        sql.Bool  `json:"isLocked" bun:"type:smallint,notnull,default:0"` // 数值 0/1 兼容
    IsEmailVerified null.Bool `json:"isEmailVerified" bun:"type:smallint"`         // 三态：NULL/0/1
    HasPermissions  bool      `json:"hasPermissions" bun:"-"`                      // 计算字段，不入库
}
```

`null.Bool` 三态：`{Valid: false}` → NULL，`{Valid: true, Bool: false}` → 0，`{Valid: true, Bool: true}` → 1。

## 构建 CRUD API

### 第一步：定义参数结构

**查询参数：**

```go
package payloads

import "github.com/coldsmirk/vef-framework-go/api"

type UserSearch struct {
    api.P
    Keyword  string `json:"keyword" search:"contains,column=username|email"`
    IsActive *bool  `json:"isActive" search:"eq"`
}
```

**创建/更新参数：**

```go
type UserParams struct {
    api.P
    ID       string      `json:"id"` // 更新操作时必需
    Username string      `json:"username" validate:"required,alphanum,max=32" label:"用户名"`
    Email    null.String `json:"email" validate:"omitempty,email,max=64" label:"邮箱"`
    IsActive bool        `json:"isActive"`
}
```

**分离创建和更新参数**（当验证要求不同时）：

```go
type UserParams struct {
    api.P
    ID       string
    Username string      `json:"username" validate:"required,alphanum,max=32" label:"用户名"`
    Email    null.String `json:"email" validate:"omitempty,email,max=64" label:"邮箱"`
    IsActive bool        `json:"isActive"`
}

type UserCreateParams struct {
    UserParams      `json:",inline"`
    Password        string `json:"password" validate:"required,min=6,max=16" label:"密码"`
    PasswordConfirm string `json:"passwordConfirm" validate:"required,eqfield=Password" label:"确认密码"`
}

type UserUpdateParams struct {
    UserParams      `json:",inline"`
    Password        null.String `json:"password" validate:"omitempty,min=6,max=16" label:"密码"`
    PasswordConfirm null.String `json:"passwordConfirm" validate:"omitempty,eqfield=Password" label:"确认密码"`
}
```

### 第二步：创建 API 资源

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

**资源命名：** 使用 `{app}/{domain}/{entity}` 模式（如 `smp/sys/user`）。RPC 使用 `snake_case`，REST 使用 `kebab-case`。保留命名空间：`security/auth`、`sys/storage`、`sys/monitor`。

### 第三步：注册资源

```go
func main() {
    vef.Run(
        vef.ProvideAPIResource(resources.NewUserResource),
    )
}
```

### 预置 API 列表

| 接口 | 描述 | Action |
|-----|------|--------|
| FindOne | 查询单条记录 | find_one |
| FindAll | 查询全部记录 | find_all |
| FindPage | 分页查询 | find_page |
| Create | 创建记录 | create |
| Update | 更新记录 | update |
| Delete | 删除记录 | delete |
| CreateMany | 批量创建 | create_many |
| UpdateMany | 批量更新 | update_many |
| DeleteMany | 批量删除 | delete_many |
| FindTree | 树形查询 | find_tree |
| FindOptions | 选项列表(label/value) | find_options |
| FindTreeOptions | 树形选项 | find_tree_options |
| Import | 导入 Excel/CSV | import |
| Export | 导出 Excel/CSV | export |

**提示：** 上表中的 action 为 **RPC** 动作名。对于 **REST** 资源，action 以 HTTP 方法与子路径表示（如 `GET /`、`GET /page`、`POST /`、`PUT /:id`）。

### API Builder 方法

使用流式构建器方法配置 API 行为：

```go
Create: crud.NewCreate[User, UserParams]().
    Action("create_user").             // 自定义操作名
    Public().                          // 无需认证
    PermToken("sys.user.create").      // 权限令牌
    EnableAudit().                     // 启用审计日志
    Timeout(10 * time.Second).         // 请求超时
    RateLimit(10, 1*time.Minute).      // 每分钟 10 次请求
```

### Find Operation 配置方法

所有 Find Operation 类型（FindOne、FindAll、FindPage、FindTree、FindOptions、FindTreeOptions、Export）都支持统一的查询配置系统。

| 方法 | 说明 |
|------|------|
| `WithProcessor` | 查询结果的后处理函数 |
| `WithSelect` / `WithSelectAs` | 添加列到 SELECT 子句 |
| `WithDefaultSort` | 设置默认排序 |
| `WithCondition` | 使用 ConditionBuilder 添加 WHERE 条件 |
| `WithRelation` | 添加关联查询 |
| `WithAuditUserNames` | 获取审计用户名（created_by_name、updated_by_name） |
| `WithQueryApplier` | 自定义查询应用函数 |
| `DisableDataPerm` | 禁用数据权限过滤 |

**WithProcessor** - 查询后、返回客户端前转换结果：

```go
FindAll: crud.NewFindAll[User, UserSearch]().
    WithProcessor(func(users []User, search UserSearch, ctx fiber.Ctx) any {
        for i := range users {
            users[i].Password = "***"
        }
        return users
    }),
```

**WithDefaultSort：**

```go
FindPage: crud.NewFindPage[User, UserSearch]().
    WithDefaultSort(&sortx.OrderSpec{
        Column:    "created_at",
        Direction: sortx.OrderDesc,
    }),
```

**WithCondition：**

```go
FindAll: crud.NewFindAll[User, UserSearch]().
    WithCondition(func(cb orm.ConditionBuilder) {
        cb.Equals("is_deleted", false)
        cb.Equals("is_active", true)
    }),
```

**WithRelation：**

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

**WithAuditUserNames：**

```go
FindAll: crud.NewFindAll[User, UserSearch]().
    WithAuditUserNames(&User{}),           // 默认使用 "name" 列
    // 或：WithAuditUserNames(&User{}, "username")
```

#### QueryPart 系统（用于树形查询）

`parts` 参数指定选项应用于查询的哪个部分。这对于使用递归 CTE 的树形 API 尤为重要。

| QueryPart | 说明 |
|-----------|------|
| `QueryRoot` | 外层/根查询（排序、最终过滤） |
| `QueryBase` | CTE 中的基础查询（初始条件、起始节点） |
| `QueryRecursive` | CTE 中的递归查询（遍历配置） |
| `QueryAll` | 所有查询部分 |

**树形查询示例：**

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

#### API 特定配置

**FindPage：**

```go
FindPage: crud.NewFindPage[User, UserSearch]().
    WithDefaultPageSize(20),
```

**FindOptions：**

```go
FindOptions: crud.NewFindOptions[User, UserSearch]().
    WithDefaultColumnMapping(&crud.DataOptionColumnMapping{
        LabelColumn:       "name",
        ValueColumn:       "id",
        DescriptionColumn: "description",
    }),
```

**FindTree：**

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

模型需要父 ID 字段（通常为 `null.String`）和子节点字段（`bun:"-"`）：

```go
type Organization struct {
    orm.Model
    Name     string          `json:"name"`
    ParentID null.String     `json:"parentID" bun:"type:varchar(20)"`
    Children []Organization  `json:"children" bun:"-"`
}
```

**Export：**

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

### Pre/Post 钩子

在 CRUD 操作前后添加自定义业务逻辑：

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
        return sendWelcomeEmail(model.Email) // 在事务内执行
    }),
```

可用的钩子：

- **单条记录：** `WithPreCreate`/`WithPostCreate`、`WithPreUpdate`/`WithPostUpdate`（接收旧模型和新模型）、`WithPreDelete`/`WithPostDelete`
- **批量操作：** `WithPreCreateMany`/`WithPostCreateMany`、`WithPreUpdateMany`/`WithPostUpdateMany`、`WithPreDeleteMany`/`WithPostDeleteMany`
- **导入导出：** `WithPreImport`/`WithPostImport`、`WithPreExport`

所有 `Post*` 钩子在事务内运行。

### 自定义处理器

#### 混合生成和自定义 API

使用 `api.WithOperations()` 将 CRUD API 与自定义操作结合。**RPC** 资源将 `action`（snake_case）映射到 PascalCase 方法（如 `find_role_permissions` → `FindRolePermissions`）。**REST** 资源必须在 `OperationSpec.Handler` 中指定处理器。

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
    // 自定义业务逻辑
    return result.Ok(permissions).Response(ctx)
}

func (r *RoleResource) SaveRolePermissions(ctx fiber.Ctx, db orm.DB, params payloads.RolePermissionParams) error {
    return db.RunInTx(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
        // 在事务中保存权限
        return nil
    })
}
```

#### REST 资源示例

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

#### 可注入参数

处理器方法支持自动参数注入：

- `fiber.Ctx`、`orm.DB`、`log.Logger`、`mold.Transformer`、`*security.Principal`、`page.Pageable`
- 嵌入 `api.P`（params）或 `api.M`（meta）的自定义结构体
- Resource 结构体字段（直接字段、`api:"in"` 标签字段或嵌入的结构体）

如果您的服务实现了 `log.LoggerConfigurable[T]` 接口，框架在注入时会自动通过 `WithLogger` 提供请求范围的日志记录器。

## 数据库操作

### 查询构建器

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

### 条件构建器方法

- `Equals`、`NotEquals` - 等于/不等于
- `GreaterThan`、`GreaterThanOrEquals`、`LessThan`、`LessThanOrEquals` - 比较
- `Contains`、`StartsWith`、`EndsWith` - 字符串匹配（LIKE）
- `In`、`Between` - 范围/集合
- `IsNull`、`IsNotNull` - 空值检查
- `Or(conditions...)` - OR 多个条件

### Search 标签

使用 `search` 标签自动应用查询条件：

```go
type UserSearch struct {
    api.P
    Username string `search:"eq"`                                    // username = ?
    Email    string `search:"contains"`                              // email LIKE ?
    Age      int    `search:"gte"`                                   // age >= ?
    Status   string `search:"in"`                                    // status IN (?)
    Keyword  string `search:"contains,column=username|email|name"`   // 搜索多个列
}
```

**支持的操作符：**

| 标签 | SQL | 标签 | SQL |
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

不区分大小写变体：加 `i` 前缀（如 `iContains`、`iEndsWith`）。否定变体：加 `not` 前缀（如 `notStartsWith`、`iNotContains`）。

### 事务处理

```go
err := db.RunInTx(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
    _, err := tx.NewInsert().Model(&user).Exec(txCtx)
    if err != nil {
        return err // 自动回滚
    }
    _, err = tx.NewUpdate().Model(&profile).WherePK().Exec(txCtx)
    return err // 返回 nil 自动提交，返回错误自动回滚
})
```

## 认证与授权

### 认证方式

VEF 支持多种认证策略：

1. **JWT 认证**（默认）- Bearer token 或查询参数 `?__accessToken=xxx`
2. **OpenAPI 签名认证** - 用于外部应用，使用 HMAC 签名
3. **密码认证** - 用户名密码登录

### 实现用户加载器

实现 `security.UserLoader` 接口以集成您的用户系统：

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
    // 类似的实现
}

// 在 main.go 中注册
func main() {
    vef.Run(vef.Provide(NewMyUserLoader))
}
```

### 权限控制

在 API 上设置权限令牌：

```go
Create: crud.NewCreate[User, UserParams]().
    PermToken("sys.user.create"),
```

#### 使用内置 RBAC 实现（推荐）

实现 `security.RolePermissionsLoader` 接口即可启用 RBAC：

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

// 在 main.go 中注册
func main() {
    vef.Run(vef.Provide(NewMyRolePermissionsLoader))
}
```

框架会自动使用您的 `RolePermissionsLoader` 实现来初始化 RBAC 权限检查器和数据权限解析器。

#### 完全自定义权限控制

对于非 RBAC 场景，实现 `security.PermissionChecker` 接口并通过 `vef.Replace(vef.Annotate(..., vef.As(new(security.PermissionChecker))))` 替换。

### 数据权限

数据权限用于实现行级数据访问控制。内置数据范围：

- **AllDataScope** - 无限制访问所有数据（用于管理员）
- **SelfDataScope** - 只能访问自己创建的数据

```go
allScope := security.NewAllDataScope()
selfScope := security.NewSelfDataScope("")           // 默认使用 created_by 列
selfScope := security.NewSelfDataScope("creator_id") // 自定义列名
```

在 `RolePermissionsLoader` 中为每个权限分配数据范围：

```go
result["sys.user.view"] = security.NewAllDataScope()
result["sys.user.edit"] = security.NewSelfDataScope("")
```

**数据范围优先级**（当用户拥有多个角色时）：`PrioritySelf` (10) < `PriorityDepartment` (20) < `PriorityDepartmentAndSub` (30) < `PriorityOrganization` (40) < `PriorityOrganizationAndSub` (50) < `PriorityCustom` (60) < `PriorityAll` (10000)。优先级最高的数据范围生效。

自定义数据范围需实现 `security.DataScope` 接口（`Key()`、`Priority()`、`Supports()`、`Apply()`）。

## 配置说明

### 配置文件

将 `application.toml` 放在 `./configs/` 或 `./` 目录，或通过 `VEF_CONFIG_PATH` 环境变量指定路径。

```toml
[vef.app]
name = "my-app"          # 应用名称
port = 8080              # HTTP 端口
body_limit = "10MB"      # 请求体大小限制

[vef.data_source]
type = "postgres"        # 数据库类型：postgres、mysql、sqlite、oracle、sqlserver
host = "localhost"
port = 5432
user = "postgres"
password = "password"
database = "mydb"
schema = "public"        # PostgreSQL schema
# path = "./data.db"    # SQLite 数据库文件路径

[vef.security]
token_expires = "2h"     # JWT token 过期时间

[vef.storage]
provider = "minio"       # 存储提供者：memory、filesystem、minio（默认：memory）

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

### 环境变量

- `VEF_CONFIG_PATH` - 配置文件路径
- `VEF_LOG_LEVEL` - 日志级别（debug、info、warn、error）
- `VEF_NODE_ID` - XID 节点标识符，用于 ID 生成
- `VEF_I18N_LANGUAGE` - 语言设置（en、zh-CN）

## 更多功能

### 缓存

```go
import "github.com/coldsmirk/vef-framework-go/cache"

// 内存缓存
memCache := cache.NewMemory[models.User](
    cache.WithMemMaxSize(1000),
    cache.WithMemDefaultTTL(5 * time.Minute),
)

// Redis 缓存
redisCache := cache.NewRedis[models.User](
    redisClient, "users",
    cache.WithRdsDefaultTTL(10 * time.Minute),
)

// 使用方式
user, err := memCache.GetOrLoad(ctx, "user:123", func(ctx context.Context) (models.User, error) {
    return loadUserFromDB(ctx, "123")
})
```

### 事件总线

```go
import "github.com/coldsmirk/vef-framework-go/event"

// 发布事件
bus.Publish(event.NewBaseEvent("user.created",
    event.WithSource("user-service"),
    event.WithMeta("userID", user.ID),
))

// 订阅事件
vef.Invoke(func(bus event.Bus, logger log.Logger) {
    unsubscribe := bus.Subscribe("user.created", func(ctx context.Context, e event.Event) {
        logger.Infof("用户已创建: %s", e.Meta()["userID"])
    })
    _ = unsubscribe
})
```

### 定时任务

基于 [gocron](https://github.com/go-co-op/gocron)：

```go
import "github.com/coldsmirk/vef-framework-go/cron"

vef.Invoke(func(scheduler cron.Scheduler) {
    // Cron 表达式（5 字段：分 时 日 月 周）
    scheduler.NewJob(cron.NewCronJob(
        "0 0 * * *", false,
        cron.WithName("daily-cleanup"),
        cron.WithTask(func(ctx context.Context) {
            // 任务逻辑
        }),
    ))

    // 固定间隔
    scheduler.NewJob(cron.NewDurationJob(
        5*time.Minute,
        cron.WithName("health-check"),
        cron.WithTask(func() {
            // 每 5 分钟执行一次
        }),
    ))
})
```

任务选项：`WithTags(...)`、`WithConcurrent()`、`WithStartImmediately()`、`WithStartAt(t)`、`WithStopAt(t)`、`WithLimitedRuns(n)`。

### 文件存储

内置 `sys/storage` 资源提供：`upload`、`get_presigned_url`、`delete_temp`、`stat`、`list`。

**自定义文件上传：**

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

在 `application.toml` 中配置存储提供者，将 `vef.storage.provider` 设置为 `minio`、`filesystem` 或 `memory`（默认）。

### 数据验证

使用 [go-playground/validator](https://github.com/go-playground/validator) 标签：

```go
type UserParams struct {
    Username string `validate:"required,alphanum,min=3,max=32" label:"用户名"`
    Email    string `validate:"required,email" label:"邮箱"`
    Age      int    `validate:"min=18,max=120" label:"年龄"`
    Password string `validate:"required,min=8,containsany=!@#$%^&*" label:"密码"`
}
```

常用规则：`required`、`omitempty`、`min`、`max`、`len`、`email`、`url`、`uuid`、`alpha`、`alphanum`、`numeric`、`contains`、`startswith`、`endswith`。

### CLI 工具

**生成构建信息：**

```bash
go run github.com/coldsmirk/vef-framework-go/cmd/vef-cli@latest generate-build-info -o internal/vef/build_info.go -p vef
```

**生成模型 Schema**（类型安全的字段访问器）：

```bash
go run github.com/coldsmirk/vef-framework-go/cmd/vef-cli@latest generate-model-schema -i ./models -o ./schemas -p schemas
```

在查询中使用：

```go
import "my-app/internal/sys/schemas"

db.NewSelect().Model(&users).
    Where(func(cb orm.ConditionBuilder) {
        cb.Equals(schemas.User.Username(), "admin")
    }).
    OrderBy(schemas.User.CreatedAt(true) + " DESC").
    Scan(ctx)
```

### 生命周期钩子

使用 `vef.Lifecycle` 管理启动/关闭钩子，对资源清理至关重要：

```go
vef.Invoke(func(lc vef.Lifecycle, subscriber event.Subscriber) {
    auditSub := NewAuditEventSubscriber(subscriber)
    lc.Append(vef.StopHook(func() {
        auditSub.Unsubscribe()
    }))
})
```

### 上下文助手

`contextx` 包提供在依赖注入不可用时访问请求范围资源的函数：

- `contextx.DB(ctx)` - 请求范围的 `orm.DB`，已预配置审计字段
- `contextx.Principal(ctx)` - 当前 `*security.Principal`
- `contextx.Logger(ctx)` - 请求范围的 `log.Logger`，包含请求 ID
- `contextx.DataPermApplier(ctx)` - 数据权限应用器

在处理器方法中优先使用参数注入；在仅接收 `fiber.Ctx` 的工具函数中使用 `contextx`。

## 最佳实践

### 命名约定

- **模型：** 单数大驼峰（如 `User`、`Order`）
- **资源：** 小写斜杠分隔（如 `sys/user`、`shop/order`）
- **参数：** `XxxParams`（创建/更新）、`XxxSearch`（查询）
- **Action：** 小写下划线分隔（如 `find_page`、`create_user`）

### 错误处理

```go
import "github.com/coldsmirk/vef-framework-go/result"

return result.Ok(data).Response(ctx)
return result.Err("操作失败")
return result.Err("参数无效", result.WithCode(result.ErrCodeBadRequest))
return result.Errf("用户 %s 不存在", username)
```

### 日志记录

```go
func (r *UserResource) Handler(ctx fiber.Ctx, logger log.Logger) error {
    logger.Infof("处理来自 %s 的请求", ctx.IP())
    return nil
}
```

## 文档与资源

- [Fiber Web Framework](https://gofiber.io/) - 底层 HTTP 框架
- [Bun ORM](https://bun.uptrace.dev/) - 数据库 ORM
- [Go Playground Validator](https://github.com/go-playground/validator) - 数据验证
- [Uber FX](https://uber-go.github.io/fx/) - 依赖注入

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 许可。

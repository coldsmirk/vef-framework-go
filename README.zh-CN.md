<h1 align="center">VEF Framework Go</h1>

<p align="center">
  <img src="./mascot.png" alt="VEF Framework 吉祥物" width="180">
</p>

<p align="center">
  一个面向企业应用的 Go 框架，基于 Fiber、Uber FX 和 Bun 构建。
</p>

<p align="center">
  提供统一 API 资源模型、泛型 CRUD、认证鉴权、RBAC、校验、缓存、事件、存储、MCP 等开箱即用能力。
</p>

<p align="center">
  <a href="./README.md">English</a> |
  <a href="./README.zh-CN.md">简体中文</a> |
  <a href="#快速开始">Quick Start</a> |
  <a href="https://coldsmirk.github.io/vef-framework-go-docs">文档站点</a> |
  <a href="https://pkg.go.dev/github.com/coldsmirk/vef-framework-go">API 参考</a>
</p>

<p align="center">
  <a href="https://github.com/coldsmirk/vef-framework-go/releases"><img src="https://img.shields.io/github/v/release/coldsmirk/vef-framework-go?style=flat-square&label=release" alt="GitHub Release"></a>
  <a href="https://github.com/coldsmirk/vef-framework-go/actions/workflows/test.yml"><img src="https://img.shields.io/github/actions/workflow/status/coldsmirk/vef-framework-go/test.yml?branch=main&label=tests&style=flat-square&logo=githubactions" alt="Build Status"></a>
  <a href="https://codecov.io/gh/coldsmirk/vef-framework-go"><img src="https://img.shields.io/codecov/c/github/coldsmirk/vef-framework-go?style=flat-square&logo=codecov&label=codecov" alt="Coverage"></a>
  <a href="https://pkg.go.dev/github.com/coldsmirk/vef-framework-go"><img src="https://img.shields.io/badge/go-reference-00ACD7?style=flat-square&logo=go&logoColor=white" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/coldsmirk/vef-framework-go"><img src="https://img.shields.io/badge/go%20report-A%2B-75C46B?style=flat-square" alt="Go Report Card"></a>
  <a href="https://deepwiki.com/coldsmirk/vef-framework-go"><img src="https://img.shields.io/badge/Ask-DeepWiki-1f6feb?style=flat-square" alt="Ask DeepWiki"></a>
  <a href="https://github.com/coldsmirk/vef-framework-go/blob/main/LICENSE"><img src="https://img.shields.io/github/license/coldsmirk/vef-framework-go?style=flat-square&label=license" alt="License"></a>
</p>

VEF Framework Go 把依赖注入、HTTP 路由和数据访问整合成一套一致的应用框架，并内置 API 资源模型、认证鉴权、RBAC、校验、缓存、事件、存储、MCP 等常用能力。

> 本 README 刻意保持简洁。更详细的教程、参考手册和架构说明请查看[文档站点](https://coldsmirk.github.io/vef-framework-go-docs)。

> 当前项目仍处于 1.0 之前的快速迭代阶段，后续仍可能出现破坏性变更。

## 为什么选择 VEF

- 一套资源模型同时覆盖 RPC 和 REST API
- 用泛型 CRUD 减少重复的后台样板代码
- 基于 Uber FX 模块化组装，便于接入和扩展业务能力
- 认证鉴权、RBAC、限流、审计、缓存、事件、存储、MCP 等基础设施开箱即用，减少自行拼装成本

## 快速开始

环境要求：
- Go 1.26.0 或更高版本
- PostgreSQL、MySQL 或 SQLite 等受支持的数据库

安装：
```bash
go get github.com/coldsmirk/vef-framework-go
```

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

[vef.data_sources.primary]
type = "sqlite"
path = "./my-app.db"

# 附加数据源按需配置，通过 datasource.Registry.Get("<name>") 取用。
# 例如：
# [vef.data_sources.analytics]
# type = "postgres"
# host = "analytics.example.com"
# database = "warehouse"
```

这个配置示例已经可以直接运行；`vef.monitor`、`vef.mcp`、`vef.approval` 等配置段按需补充即可。

运行：

```bash
go run main.go
```

VEF 会从 `./configs`、`.`、`../configs` 或 `VEF_CONFIG_PATH` 指定的位置加载 `application.toml`。

## 核心概念

- `vef.Run(...)` 会启动框架，并按默认链路装配 config、datasource（数据源注册表与主连接）、middleware、API、security、event、CQRS、cron、redis、mold、storage、sequence、schema、monitor、MCP、app 等模块。
- API 通过 `api.NewRPCResource(...)` 或 `api.NewRESTResource(...)` 定义资源。
- 业务模块通常通过 `vef.ProvideAPIResource(...)`、`vef.ProvideMiddleware(...)`、`vef.ProvideMCPTools(...)` 等方式接入。
- 如果业务以标准增删改查为主，可以优先使用 `crud/` 中的泛型能力减少样板代码。

典型应用目录：

```text
my-app/
├── cmd/
├── configs/
└── internal/
    ├── auth/
    ├── sys/
    ├── <domain>/
    └── web/
```

## 文档入口

- 文档站点：<https://coldsmirk.github.io/vef-framework-go-docs>
- API 参考：<https://pkg.go.dev/github.com/coldsmirk/vef-framework-go>
- 仓库知识图谱：<https://deepwiki.com/coldsmirk/vef-framework-go>
- 测试规范：[TESTING.md](./TESTING.md)

如果你需要分步骤教程、架构细节或特性级参考，请优先查看[文档站点](https://coldsmirk.github.io/vef-framework-go-docs)，而不是继续膨胀这个 README。

## 开发

常用校验命令：

```bash
go test ./...
go test -race ./...
golangci-lint run
go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -test ./...
```

## 许可证

本项目基于 [Apache License 2.0](./LICENSE) 开源。

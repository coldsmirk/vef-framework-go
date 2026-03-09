package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/fx"

	smcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/coldsmirk/vef-framework-go/config"
	ilog "github.com/coldsmirk/vef-framework-go/internal/log"
	"github.com/coldsmirk/vef-framework-go/mcp"
)

var logger = ilog.Named("mcp")

type ServerParams struct {
	fx.In

	MCPConfig         *config.MCPConfig
	AppConfig         *config.AppConfig
	ToolProviders     []mcp.ToolProvider             `group:"vef:mcp:tools"`
	ResourceProviders []mcp.ResourceProvider         `group:"vef:mcp:resources"`
	TemplateProviders []mcp.ResourceTemplateProvider `group:"vef:mcp:templates"`
	PromptProviders   []mcp.PromptProvider           `group:"vef:mcp:prompts"`
	ServerInfo        *mcp.ServerInfo                `optional:"true"`
}

func NewServer(params ServerParams) *smcp.Server {
	if !params.MCPConfig.Enabled {
		logger.Info("MCP is disabled by configuration")

		return nil
	}

	server := smcp.NewServer(
		&smcp.Implementation{
			Name:    getServerName(params),
			Version: getServerVersion(params),
		},
		&smcp.ServerOptions{
			Instructions: getInstructions(params),
		},
	)

	middleware := createLoggingMiddleware()
	server.AddSendingMiddleware(middleware)
	server.AddReceivingMiddleware(middleware)

	registerTools(server, params.ToolProviders)
	registerResources(server, params.ResourceProviders)
	registerResourceTemplates(server, params.TemplateProviders)
	registerPrompts(server, params.PromptProviders)

	logger.Info("MCP server initialized")

	return server
}

func registerTools(server *smcp.Server, providers []mcp.ToolProvider) {
	for _, provider := range providers {
		for _, def := range provider.Tools() {
			server.AddTool(def.Tool, def.Handler)
			logger.Infof("Registered MCP tool: %s", def.Tool.Name)
		}
	}
}

func registerResources(server *smcp.Server, providers []mcp.ResourceProvider) {
	for _, provider := range providers {
		for _, def := range provider.Resources() {
			server.AddResource(def.Resource, def.Handler)
			logger.Infof("Registered MCP resource: %s", def.Resource.URI)
		}
	}
}

func registerResourceTemplates(server *smcp.Server, providers []mcp.ResourceTemplateProvider) {
	for _, provider := range providers {
		for _, def := range provider.ResourceTemplates() {
			server.AddResourceTemplate(def.Template, def.Handler)
			logger.Infof("Registered MCP resource template: %s", def.Template.URITemplate)
		}
	}
}

func registerPrompts(server *smcp.Server, providers []mcp.PromptProvider) {
	for _, provider := range providers {
		for _, def := range provider.Prompts() {
			server.AddPrompt(def.Prompt, def.Handler)
			logger.Infof("Registered MCP prompt: %s", def.Prompt.Name)
		}
	}
}

func createLoggingMiddleware() smcp.Middleware {
	return func(next smcp.MethodHandler) smcp.MethodHandler {
		return func(ctx context.Context, method string, req smcp.Request) (smcp.Result, error) {
			start := time.Now()
			result, err := next(ctx, method, req)
			elapsed := time.Since(start)

			sessionID := req.GetSession().ID()
			params := formatParams(req.GetParams())
			latency := formatLatency(elapsed)

			if err != nil {
				logger.Errorf("Request failed: %v | method: %s | params: %s | session: %s | latency: %s", err, method, params, sessionID, latency)
			} else {
				logger.Infof("Request completed | method: %s | params: %s | session: %s | latency: %s", method, params, sessionID, latency)
			}

			return result, err
		}
	}
}

func formatLatency(elapsed time.Duration) string {
	if ms := elapsed.Milliseconds(); ms > 0 {
		return fmt.Sprintf("%dms", ms)
	}

	return fmt.Sprintf("%dμs", elapsed.Microseconds())
}

func formatParams(params smcp.Params) string {
	if params == nil {
		return "{}"
	}

	jsonBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Sprintf("%v", params)
	}

	return string(jsonBytes)
}

func getServerName(params ServerParams) string {
	if params.ServerInfo != nil && params.ServerInfo.Name != "" {
		return params.ServerInfo.Name
	}

	if params.AppConfig != nil && params.AppConfig.Name != "" {
		return params.AppConfig.Name
	}

	return "vef-mcp-server"
}

func getServerVersion(params ServerParams) string {
	if params.ServerInfo != nil && params.ServerInfo.Version != "" {
		return params.ServerInfo.Version
	}

	return "v1.0.0"
}

func getInstructions(params ServerParams) string {
	if params.ServerInfo != nil {
		return params.ServerInfo.Instructions
	}

	return ""
}

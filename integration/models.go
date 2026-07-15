package integration

import (
	"encoding/json"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Contract defines a standard integration operation: the business-side input
// and output models every provider adapter must honor. Business code programs
// against contracts; provider differences stay inside adapter scripts.
type Contract struct {
	orm.BaseModel `bun:"table:itg_contract,alias:ic"`
	orm.FullAuditedModel

	Code        string  `json:"code" bun:"code"`
	Name        string  `json:"name" bun:"name"`
	Description *string `json:"description" bun:"description,nullzero"`
	// InputSchema is the self-contained JSON Schema (draft 2020-12) the
	// invocation input is validated against; empty skips input validation.
	InputSchema json.RawMessage `json:"inputSchema" bun:"input_schema,type:jsonb,nullzero"`
	// OutputSchema is the self-contained JSON Schema the adapter script's
	// return value is validated against; empty skips output validation.
	OutputSchema json.RawMessage `json:"outputSchema" bun:"output_schema,type:jsonb,nullzero"`
	IsEnabled    bool            `json:"isEnabled" bun:"is_enabled"`
}

// System is one external system instance: where it lives and how to
// authenticate against it. BaseURL enables the scoped http library,
// DataSource enables the scoped read-only sql library; a system may carry
// both. Connection-level defaults (timeout, retry) apply to every adapter of
// the system unless overridden per adapter.
type System struct {
	orm.BaseModel `bun:"table:itg_system,alias:isy"`
	orm.FullAuditedModel

	Code    string `json:"code" bun:"code"`
	Name    string `json:"name" bun:"name"`
	BaseURL string `json:"baseUrl" bun:"base_url"`
	// OutboundAuth selects how the framework authenticates against this
	// system's HTTP endpoints; nil sends requests unauthenticated.
	OutboundAuth *OutboundAuthConfig `json:"outboundAuth" bun:"outbound_auth,type:jsonb,nullzero"`
	// InboundAuth selects how calls arriving on this system's inbound
	// endpoints are proven to originate from the system itself; a system
	// without it refuses inbound delivery entirely (fail closed — the "none"
	// scheme opens it up deliberately).
	InboundAuth *InboundAuthConfig `json:"inboundAuth" bun:"inbound_auth,type:jsonb,nullzero"`
	// DataSource is the system's direct database connection (external views /
	// exchange tables). Its password is stored encrypted and masked in
	// management API responses.
	DataSource *DataSourceConfig `json:"dataSource" bun:"data_source,type:jsonb,nullzero"`
	// Params are non-sensitive, system-specific values (branch codes,
	// version flags) exposed to adapter scripts as system.params.
	Params map[string]string `json:"params" bun:"params,type:jsonb,nullzero"`
	// TimeoutMs bounds each outbound call of the system; zero applies the
	// framework default.
	TimeoutMs int          `json:"timeoutMs" bun:"timeout_ms"`
	Retry     *RetryPolicy `json:"retry" bun:"retry,type:jsonb,nullzero"`
	IsEnabled bool         `json:"isEnabled" bun:"is_enabled"`
}

// DataSourceConfig describes a system's direct database connection. It
// mirrors config.DataSourceConfig with JSON tags for jsonb storage and the
// management API; ToConfig converts it for the datasource registry.
type DataSourceConfig struct {
	Kind        config.DBKind  `json:"kind"`
	Host        string         `json:"host,omitempty"`
	Port        uint16         `json:"port,omitempty"`
	User        string         `json:"user,omitempty"`
	Password    string         `json:"password,omitempty"`
	Database    string         `json:"database,omitempty"`
	Schema      string         `json:"schema,omitempty"`
	Path        string         `json:"path,omitempty"`
	SSLMode     config.SSLMode `json:"sslMode,omitempty"`
	SSLRootCert string         `json:"sslRootCert,omitempty"`
}

// ToConfig converts the connection settings into the framework's data source
// configuration. Scripts stay read-only through the sql library's own guard,
// so the connection-level SQL guard is left off.
func (c *DataSourceConfig) ToConfig() config.DataSourceConfig {
	return config.DataSourceConfig{
		Kind:        c.Kind,
		Host:        c.Host,
		Port:        c.Port,
		User:        c.User,
		Password:    c.Password,
		Database:    c.Database,
		Schema:      c.Schema,
		Path:        c.Path,
		SSLMode:     c.SSLMode,
		SSLRootCert: c.SSLRootCert,
	}
}

// MaskedSecret is the placeholder management APIs return in place of a
// sensitive auth parameter value. An update submitting the placeholder keeps
// the stored value unchanged.
const MaskedSecret = "******"

// OutboundAuthConfig selects the OutboundAuthScheme authenticating a system's
// outbound calls and carries its parameters. Values of the parameters named
// by the scheme's SensitiveParams are stored encrypted and masked in
// management API responses.
type OutboundAuthConfig struct {
	Scheme string            `json:"scheme"`
	Params map[string]string `json:"params,omitempty"`
}

// InboundAuthConfig selects the InboundAuthScheme verifying calls on a
// system's inbound endpoints and carries its parameters. Values of the
// parameters named by the scheme's SensitiveParams are stored encrypted and
// masked in management API responses.
type InboundAuthConfig struct {
	Scheme string            `json:"scheme"`
	Params map[string]string `json:"params,omitempty"`
	// Script is the custom verification body for the "script" scheme: it runs
	// in a runtime with no IO capabilities, sees request and params, and
	// grants access by returning a truthy value.
	Script string `json:"script,omitempty"`
}

// RetryPolicy is the declarative retry configuration for a system's outbound
// calls. It rides on the httpx default retry policy: idempotent methods only,
// retried on transport errors and 429/502/503/504 responses.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts, the first call included.
	MaxAttempts int `json:"maxAttempts"`
	// InitialBackoffMs is the base delay before the first retry; zero applies
	// the httpx default.
	InitialBackoffMs int64 `json:"initialBackoffMs,omitempty"`
	// MaxBackoffMs caps the delay between attempts; zero applies the httpx
	// default.
	MaxBackoffMs int64 `json:"maxBackoffMs,omitempty"`
}

// Direction distinguishes the two integration flows an adapter can implement:
// outbound (business code invokes the external system) and inbound (the
// external system calls in through an inbound gateway).
type Direction string

const (
	// DirectionOutbound marks a script translating contract input into calls
	// on the external system and its responses into the contract output.
	DirectionOutbound Direction = "outbound"
	// DirectionInbound marks a script translating a request the external
	// system initiated into a contract dispatch and the dispatch result into
	// the reply the system expects.
	DirectionInbound Direction = "inbound"
)

// IsValid reports whether the direction is one of the two known flows.
func (d Direction) IsValid() bool {
	return d == DirectionOutbound || d == DirectionInbound
}

// Adapter binds one system to one contract: its script translates between
// the system's wire format and the contract's standard models. A system
// implements a contract with exactly one adapter per direction.
type Adapter struct {
	orm.BaseModel `bun:"table:itg_adapter,alias:iad"`
	orm.FullAuditedModel

	SystemID   string `json:"systemId" bun:"system_id"`
	ContractID string `json:"contractId" bun:"contract_id"`
	// Direction selects the flow the script implements; an empty value is
	// normalized to outbound at save time.
	Direction Direction `json:"direction" bun:"direction"`
	Script    string    `json:"script" bun:"script"`
	// TimeoutMs overrides the system call timeout for this adapter; zero
	// inherits it.
	TimeoutMs int  `json:"timeoutMs" bun:"timeout_ms"`
	IsEnabled bool `json:"isEnabled" bun:"is_enabled"`
}

// Route maps a route key (tenant, branch, hospital area) to the system
// serving a contract. ContractID scopes the rule to one contract, empty
// applies to every contract; a row with an empty RouteKey is the default
// route. Exact (key, contract) matches win over contract-wildcard matches.
type Route struct {
	orm.BaseModel `bun:"table:itg_route,alias:irt"`
	orm.FullAuditedModel

	RouteKey   string `json:"routeKey" bun:"route_key"`
	ContractID string `json:"contractId" bun:"contract_id"`
	SystemID   string `json:"systemId" bun:"system_id"`
	IsEnabled  bool   `json:"isEnabled" bun:"is_enabled"`
}

// InvocationLog is one recorded invocation: its classification, timing, and
// the masked, size-capped captures selected by vef.integration.log.
type InvocationLog struct {
	orm.BaseModel `bun:"table:itg_invocation_log,alias:il"`
	orm.CreationAuditedModel

	SystemCode   string `json:"systemCode" bun:"system_code"`
	ContractCode string `json:"contractCode" bun:"contract_code"`
	// Direction records which flow produced the entry: outbound invocations
	// or inbound deliveries.
	Direction Direction `json:"direction" bun:"direction"`
	// FailureKind is empty for a successful invocation.
	FailureKind FailureKind     `json:"failureKind" bun:"failure_kind"`
	DurationMs  int64           `json:"durationMs" bun:"duration_ms"`
	Input       json.RawMessage `json:"input" bun:"input,type:jsonb,nullzero"`
	Output      json.RawMessage `json:"output" bun:"output,type:jsonb,nullzero"`
	HTTPTrace   []HTTPExchange  `json:"httpTrace" bun:"http_trace,type:jsonb,nullzero"`
	Error       *string         `json:"error" bun:"error,nullzero"`
	RequestID   string          `json:"requestId" bun:"request_id"`
}

// HTTPExchange is one wire exchange captured while an adapter script ran,
// shared by invocation logs and the dry-run trace. Bodies and header values
// are masked and truncated per vef.integration.log before they land here.
type HTTPExchange struct {
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	RequestHeaders  map[string]string `json:"requestHeaders,omitempty"`
	RequestBody     string            `json:"requestBody,omitempty"`
	Status          int               `json:"status,omitempty"`
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`
	ResponseBody    string            `json:"responseBody,omitempty"`
	DurationMs      int64             `json:"durationMs"`
	Error           string            `json:"error,omitempty"`
}

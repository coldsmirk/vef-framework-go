package auth

import (
	"fmt"

	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/integration"
)

// Built-in scheme names.
const (
	OutboundSchemeNone   = "none"
	OutboundSchemeBasic  = "basic"
	OutboundSchemeBearer = "bearer"
	OutboundSchemeHeader = "header"
	OutboundSchemeQuery  = "query"
)

// builtinOutboundSchemes returns the framework-provided auth schemes.
func builtinOutboundSchemes() []integration.OutboundAuthScheme {
	return []integration.OutboundAuthScheme{
		new(noneOutboundScheme),
		new(basicOutboundScheme),
		new(bearerOutboundScheme),
		new(headerOutboundScheme),
		new(queryOutboundScheme),
	}
}

// requireParam returns the named parameter or ErrMissingParam when absent or
// empty.
func requireParam(params map[string]string, name string) (string, error) {
	value := params[name]
	if value == "" {
		return "", fmt.Errorf("%w: %s", ErrMissingParam, name)
	}

	return value, nil
}

// noneOutboundScheme sends requests unauthenticated.
type noneOutboundScheme struct{}

func (*noneOutboundScheme) Name() string {
	return OutboundSchemeNone
}

func (*noneOutboundScheme) Apply(map[string]string) ([]httpx.Option, error) {
	return nil, nil
}

func (*noneOutboundScheme) SensitiveParams() []string {
	return nil
}

// basicOutboundScheme authenticates with HTTP Basic credentials
// (params: username, password).
type basicOutboundScheme struct{}

func (*basicOutboundScheme) Name() string {
	return OutboundSchemeBasic
}

func (*basicOutboundScheme) Apply(params map[string]string) ([]httpx.Option, error) {
	username, err := requireParam(params, "username")
	if err != nil {
		return nil, err
	}

	password, err := requireParam(params, "password")
	if err != nil {
		return nil, err
	}

	return []httpx.Option{httpx.WithBasicAuth(username, password)}, nil
}

func (*basicOutboundScheme) SensitiveParams() []string {
	return []string{"password"}
}

// bearerOutboundScheme authenticates with a static bearer token (params: token).
type bearerOutboundScheme struct{}

func (*bearerOutboundScheme) Name() string {
	return OutboundSchemeBearer
}

func (*bearerOutboundScheme) Apply(params map[string]string) ([]httpx.Option, error) {
	token, err := requireParam(params, "token")
	if err != nil {
		return nil, err
	}

	return []httpx.Option{httpx.WithBearerToken(token)}, nil
}

func (*bearerOutboundScheme) SensitiveParams() []string {
	return []string{"token"}
}

// headerOutboundScheme sends a static credential header (params: name, value).
type headerOutboundScheme struct{}

func (*headerOutboundScheme) Name() string {
	return OutboundSchemeHeader
}

func (*headerOutboundScheme) Apply(params map[string]string) ([]httpx.Option, error) {
	name, err := requireParam(params, "name")
	if err != nil {
		return nil, err
	}

	value, err := requireParam(params, "value")
	if err != nil {
		return nil, err
	}

	return []httpx.Option{httpx.WithHeader(name, value)}, nil
}

func (*headerOutboundScheme) SensitiveParams() []string {
	return []string{"value"}
}

// queryOutboundScheme sends a static credential query parameter, such as an API key
// (params: name, value).
type queryOutboundScheme struct{}

func (*queryOutboundScheme) Name() string {
	return OutboundSchemeQuery
}

func (*queryOutboundScheme) Apply(params map[string]string) ([]httpx.Option, error) {
	name, err := requireParam(params, "name")
	if err != nil {
		return nil, err
	}

	value, err := requireParam(params, "value")
	if err != nil {
		return nil, err
	}

	return []httpx.Option{httpx.WithQuery(name, value)}, nil
}

func (*queryOutboundScheme) SensitiveParams() []string {
	return []string{"value"}
}

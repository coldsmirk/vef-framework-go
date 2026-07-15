package auth

import (
	"fmt"

	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/integration"
)

// Built-in scheme names.
const (
	SchemeNone   = "none"
	SchemeBasic  = "basic"
	SchemeBearer = "bearer"
	SchemeHeader = "header"
	SchemeQuery  = "query"
)

// builtinSchemes returns the framework-provided auth schemes.
func builtinSchemes() []integration.AuthScheme {
	return []integration.AuthScheme{
		new(noneScheme),
		new(basicScheme),
		new(bearerScheme),
		new(headerScheme),
		new(queryScheme),
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

// noneScheme sends requests unauthenticated.
type noneScheme struct{}

func (*noneScheme) Name() string {
	return SchemeNone
}

func (*noneScheme) Apply(map[string]string) ([]httpx.Option, error) {
	return nil, nil
}

func (*noneScheme) SensitiveParams() []string {
	return nil
}

// basicScheme authenticates with HTTP Basic credentials
// (params: username, password).
type basicScheme struct{}

func (*basicScheme) Name() string {
	return SchemeBasic
}

func (*basicScheme) Apply(params map[string]string) ([]httpx.Option, error) {
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

func (*basicScheme) SensitiveParams() []string {
	return []string{"password"}
}

// bearerScheme authenticates with a static bearer token (params: token).
type bearerScheme struct{}

func (*bearerScheme) Name() string {
	return SchemeBearer
}

func (*bearerScheme) Apply(params map[string]string) ([]httpx.Option, error) {
	token, err := requireParam(params, "token")
	if err != nil {
		return nil, err
	}

	return []httpx.Option{httpx.WithBearerToken(token)}, nil
}

func (*bearerScheme) SensitiveParams() []string {
	return []string{"token"}
}

// headerScheme sends a static credential header (params: name, value).
type headerScheme struct{}

func (*headerScheme) Name() string {
	return SchemeHeader
}

func (*headerScheme) Apply(params map[string]string) ([]httpx.Option, error) {
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

func (*headerScheme) SensitiveParams() []string {
	return []string{"value"}
}

// queryScheme sends a static credential query parameter, such as an API key
// (params: name, value).
type queryScheme struct{}

func (*queryScheme) Name() string {
	return SchemeQuery
}

func (*queryScheme) Apply(params map[string]string) ([]httpx.Option, error) {
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

func (*queryScheme) SensitiveParams() []string {
	return []string{"value"}
}

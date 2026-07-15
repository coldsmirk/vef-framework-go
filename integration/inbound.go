package integration

import (
	"context"
	"encoding/json"
	"fmt"
)

// InboundRequest is the protocol-neutral envelope an inbound gateway builds
// from one vendor-initiated call. SystemCode and ContractCode identify the
// target the gateway resolved (for HTTP, from the URL path). Headers carries
// the protocol's named metadata with lowercased keys — HTTP headers verbatim;
// a non-HTTP gateway maps its protocol's equivalent (an MLLP gateway could
// project MSH fields). Method, Path, and Query are HTTP-native and stay empty
// where the protocol has no counterpart.
type InboundRequest struct {
	SystemCode   string
	ContractCode string
	// Protocol names the gateway that received the call (e.g. "http").
	Protocol string
	Method   string
	Path     string
	Headers  map[string]string
	Query    map[string]string
	// Body is the raw request payload.
	Body []byte
	// ClientAddr is the network peer address, for IP-based verification.
	ClientAddr string
}

// InboundAuthScheme verifies that a vendor-initiated request truly originates
// from the system it targets, using the system's stored inbound auth
// configuration. Built-in schemes cover the common cases; applications
// register their own via vef.ProvideIntegrationInboundAuthScheme, and a
// scheme whose name matches a built-in replaces it.
type InboundAuthScheme interface {
	// Name identifies the scheme referenced by a system's inboundAuth.scheme.
	Name() string
	// Verify authenticates the request against the system's inbound auth
	// configuration. The config arrives with sensitive parameters decrypted;
	// their values must never be logged. Any returned error denies the
	// delivery — its message stays server-side and is never echoed to the
	// caller.
	Verify(ctx context.Context, req *InboundRequest, auth *InboundAuthConfig) error
	// SensitiveParams names the parameters whose values are stored encrypted
	// and masked in management API responses; the SensitiveAll wildcard marks
	// every parameter sensitive.
	SensitiveParams() []string
}

// SensitiveAll is the SensitiveParams wildcard marking every parameter of a
// scheme sensitive, for schemes whose parameter names are not known
// statically (the built-in "script" scheme uses it).
const SensitiveAll = "*"

// InboundHandler is the business-side receiver of one inbound contract: it
// consumes the standard input an inbound adapter script dispatched and
// returns the standard output, both validated against the contract's schemas.
// Register implementations with vef.ProvideIntegrationInboundHandler — one
// handler per contract code. Handlers must be idempotent: vendors deliver
// at-least-once.
type InboundHandler interface {
	// Contract returns the code of the contract the handler serves.
	Contract() string
	// Handle processes one schema-validated dispatch and returns the standard
	// output. A returned error is classified as FailureHandler; adapter
	// scripts may catch it to shape the vendor-facing reply.
	Handle(ctx context.Context, input any) (any, error)
}

// NewInboundHandler adapts a typed function to an InboundHandler: the
// schema-validated input is decoded into I through a JSON round-trip before
// the function runs.
func NewInboundHandler[I any, O any](contract string, handle func(ctx context.Context, input I) (O, error)) InboundHandler {
	return &typedInboundHandler[I, O]{contract: contract, handle: handle}
}

type typedInboundHandler[I any, O any] struct {
	contract string
	handle   func(ctx context.Context, input I) (O, error)
}

// Contract returns the code of the contract the handler serves.
func (h *typedInboundHandler[I, O]) Contract() string {
	return h.contract
}

// Handle decodes the input into the typed model and delegates to the wrapped
// function.
func (h *typedInboundHandler[I, O]) Handle(ctx context.Context, input any) (any, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("integration: encode inbound input: %w", err)
	}

	var typed I
	if err := json.Unmarshal(payload, &typed); err != nil {
		return nil, fmt.Errorf("integration: decode inbound input: %w", err)
	}

	return h.handle(ctx, typed)
}

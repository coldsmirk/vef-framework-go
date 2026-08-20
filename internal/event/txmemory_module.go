package event

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/memory"
	"github.com/coldsmirk/vef-framework-go/event/transport/txmemory"
	itxmemory "github.com/coldsmirk/vef-framework-go/internal/event/transport/txmemory"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

var txMemoryLogger = logx.Named("event:tx_memory")

// TxMemoryTransportModule wires the in-process transactional transport.
// Disabled by default; enable via
// vef.event.transports.tx_memory.enabled = true.
var TxMemoryTransportModule = fx.Module(
	"vef:event:tx_memory",
	fx.Provide(
		fx.Annotate(
			newTxMemoryTransport,
			fx.ResultTags(`group:"vef:event:transports"`),
			fx.As(new(transport.Transport)),
		),
	),
)

func newTxMemoryTransport(cfg *config.EventConfig) transport.Transport {
	if !cfg.Transports.TxMemory.Enabled {
		return nil
	}

	// Loud on purpose: the transport looks like the outbox to every
	// start-up check that asks for a transactional route, but buys none of
	// its durability. Anyone reading a production log should see the
	// trade they made.
	txMemoryLogger.Warnf(
		"Transport %q is enabled: events publish in-process and are NOT durable — "+
			"anything published between commit and delivery is lost if this process dies. "+
			"Intended for development against a shared database; use the outbox in production.",
		txmemory.Name,
	)

	return itxmemory.New(txMemoryConfig(cfg), txMemoryLogger)
}

// txMemoryConfig collapses the framework-level config into the delivery
// config of the private memory transport that performs the fan-out.
func txMemoryConfig(cfg *config.EventConfig) memory.Config {
	c := cfg.Transports.TxMemory

	return memory.Config{
		QueueSize:      c.QueueSize,
		FullPolicy:     memory.FullPolicy(c.FullPolicy),
		PublishTimeout: c.PublishTimeout,
	}
}

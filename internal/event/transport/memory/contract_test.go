package memory_test

import (
	"testing"

	"github.com/coldsmirk/vef-framework-go/event/transport"
	pubmemory "github.com/coldsmirk/vef-framework-go/event/transport/memory"
	"github.com/coldsmirk/vef-framework-go/internal/event/transport/memory"
	"github.com/coldsmirk/vef-framework-go/internal/event/transport/transporttest"
)

func TestMemoryTransportContract(t *testing.T) {
	factory := func(*testing.T) (transport.Transport, func()) {
		tp := memory.New(pubmemory.Config{QueueSize: 64, FullPolicy: pubmemory.FullPolicyError})

		return tp, func() {}
	}
	transporttest.Suite(t, "Memory", factory)
}

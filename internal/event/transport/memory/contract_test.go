package memory_test

import (
	"testing"

	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/event/transport/memory"
	imemory "github.com/coldsmirk/vef-framework-go/internal/event/transport/memory"
	"github.com/coldsmirk/vef-framework-go/internal/event/transport/transporttest"
)

func TestMemoryTransportContract(t *testing.T) {
	factory := func(*testing.T) (transport.Transport, func()) {
		tp := imemory.New(memory.Config{QueueSize: 64, FullPolicy: memory.FullPolicyError})

		return tp, func() {}
	}
	transporttest.Suite(t, "Memory", factory)
}

package store

import (
	"fmt"
	"os"

	"github.com/coldsmirk/vef-framework-go/id"
)

// newNodeID builds this process's executor identity: informative for
// operators reading the run journal (host and pid) and unique per boot (the
// XID suffix), so restarted processes never impersonate their predecessor's
// heartbeats.
func newNodeID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown"
	}

	return fmt.Sprintf("%s/%d/%s", hostname, os.Getpid(), id.Generate())
}

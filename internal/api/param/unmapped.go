package param

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

var logger = logx.Named("api.param")

// maxUnmappedReports caps how many distinct reports are remembered. The key
// embeds caller-supplied parameter names, so without a cap a client sending
// random names could grow the set without bound.
const maxUnmappedReports = 1024

var (
	reportedUnmapped      sync.Map
	reportedUnmappedCount atomic.Int64
)

// warnUnmappedParams reports request keys that no field of the target struct
// claimed. Decoding ignores them on purpose, so a client still sending a
// retired field keeps working — but that same silence is what lets a renamed or
// misspelled key fail invisibly: the request succeeds while the value it
// carried never lands anywhere.
//
// Each operation and key set is reported once. The drift is a property of a
// client build rather than of a request, so repeating the line per call would
// bury it under its own noise — the log would then be filtered away exactly
// when it finally has something new to say.
func warnUnmappedParams(identifier api.Identifier, unmapped []string) {
	if len(unmapped) == 0 {
		return
	}

	keys := strings.Join(unmapped, ", ")
	report := identifier.String() + "|" + keys

	if _, reported := reportedUnmapped.Load(report); reported {
		return
	}

	// Past the cap reporting stops rather than growing: a deployment producing
	// this many distinct reports has drift that one more line would not
	// clarify. Concurrent callers may overshoot it by a few entries, which the
	// bound tolerates — it exists to keep the set finite, not to ration it.
	if reportedUnmappedCount.Load() >= maxUnmappedReports {
		return
	}

	if _, loaded := reportedUnmapped.LoadOrStore(report, struct{}{}); loaded {
		return
	}

	reportedUnmappedCount.Add(1)

	logger.Warnf("Operation %s received request params it declares no field for: %s", identifier, keys)
}

package event

import "context"

// StreamInspector reports consumer-group state for every stream a
// cross-process transport manages. Its purpose is operational visibility:
// a decommissioned subscriber leaves its consumer group behind (growing lag,
// stalled last-delivered position), and this is the surface that makes such
// orphans observable — the monitor module exposes it as an API endpoint.
//
// The redis_stream transport provides an implementation when enabled; the
// dependency is optional, so consumers must tolerate a nil inspector.
type StreamInspector interface {
	// Streams lists every stream under the transport's prefix together
	// with its consumer groups.
	Streams(ctx context.Context) ([]StreamInfo, error)
}

// StreamInfo describes one transport stream and its consumer groups.
type StreamInfo struct {
	// Stream is the full transport-level stream key (prefix + event type).
	Stream string `json:"stream"`
	// Length is the current number of entries in the stream (post-trim).
	Length int64 `json:"length"`
	// Groups lists the consumer groups attached to the stream.
	Groups []StreamGroupInfo `json:"groups"`
}

// StreamGroupInfo describes one consumer group on a stream. A group whose
// Lag keeps growing while Consumers stay idle is an orphan candidate — a
// subscriber that was removed or renamed without decommissioning its group.
type StreamGroupInfo struct {
	// Name is the consumer group name (the subscription's WithGroup value
	// or its derived default).
	Name string `json:"name"`
	// Consumers is the number of consumer records registered in the group,
	// including historical consumers of restarted processes.
	Consumers int64 `json:"consumers"`
	// Pending is the number of delivered-but-unacknowledged entries.
	Pending int64 `json:"pending"`
	// Lag is the number of stream entries not yet delivered to this group,
	// as reported by Redis (approximate after trimming; zero on server
	// versions that do not report lag).
	Lag int64 `json:"lag"`
	// LastDeliveredID is the stream ID of the last entry delivered to the
	// group. Its timestamp part shows how long the group has been stalled.
	LastDeliveredID string `json:"lastDeliveredId"`
}

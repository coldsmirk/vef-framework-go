package store

import (
	"time"

	"github.com/coldsmirk/vef-framework-go/orm"
)

type fireRequestKind string

const (
	fireRequestManual   fireRequestKind = "manual"
	fireRequestRecovery fireRequestKind = "recovery"
)

// fireRequest is one durable, explicit execution request. Regular trigger
// occurrences remain represented by Schedule.NextFireAtUnixMs; manual and
// recovery fires live here so each request survives independently until claimed.
type fireRequest struct {
	orm.BaseModel `bun:"table:crn_fire_request,alias:cfr"`
	orm.Model

	ScheduleID        string          `bun:"schedule_id"`
	Kind              fireRequestKind `bun:"kind"`
	ScheduledAtUnixMs int64           `bun:"scheduled_at_unix_ms"`
	SourceRunID       string          `bun:"source_run_id,nullzero"`
}

func (r *fireRequest) scheduledAt() time.Time {
	return unixTime(r.ScheduledAtUnixMs)
}

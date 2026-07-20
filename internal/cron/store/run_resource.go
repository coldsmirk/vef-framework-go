package store

import (
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/sortx"
)

// RunSearch contains the search parameters for the run journal.
type RunSearch struct {
	crud.Sortable

	// ID addresses one journal row; find_one has no other way to name the
	// record the caller means.
	ID                    string `json:"id" search:"eq,column=id"`
	ScheduleName          string `json:"scheduleName" search:"eq,column=schedule_name"`
	JobName               string `json:"jobName" search:"eq,column=job_name"`
	Status                string `json:"status" search:"eq"`
	NodeID                string `json:"nodeId" search:"eq,column=node_id"`
	ScheduledAtFromUnixMs *int64 `json:"scheduledAtFromUnixMs" search:"gte,column=scheduled_at_unix_ms"`
	ScheduledAtToUnixMs   *int64 `json:"scheduledAtToUnixMs" search:"lte,column=scheduled_at_unix_ms"`
}

// RunResource exposes the run journal read-only: the paged view for
// browsing and the single-record view for the full error text.
type RunResource struct {
	api.Resource

	crud.FindPage[cron.Run, RunSearch]
	crud.FindOne[cron.Run, RunSearch]
}

// NewRunResource creates the run journal resource. With the store disabled
// the resource mounts no operations.
func NewRunResource(cfg *config.CronConfig) api.Resource {
	const name = "sys/cron/run"

	if !cfg.Store.Enabled {
		return api.NewRPCResource(name)
	}

	return &RunResource{
		Resource: api.NewRPCResource(name),
		FindPage: crud.NewFindPage[cron.Run, RunSearch]().
			WithDefaultSort(
				&sortx.OrderSpec{Column: "claimed_at_unix_ms", Direction: sortx.OrderDesc},
				&sortx.OrderSpec{Column: "id", Direction: sortx.OrderDesc},
			).
			RequiredPermission("cron.run.query"),
		FindOne: crud.NewFindOne[cron.Run, RunSearch]().
			RequiredPermission("cron.run.query"),
	}
}

package store

import (
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// RunSearch contains the search parameters for the run journal.
type RunSearch struct {
	crud.Sortable

	ScheduleName    string          `json:"scheduleName" search:"eq,column=schedule_name"`
	JobName         string          `json:"jobName" search:"eq,column=job_name"`
	Status          string          `json:"status" search:"eq"`
	NodeID          string          `json:"nodeId" search:"eq,column=node_id"`
	ScheduledAtFrom *timex.DateTime `json:"scheduledAtFrom" search:"gte,column=scheduled_at"`
	ScheduledAtTo   *timex.DateTime `json:"scheduledAtTo" search:"lte,column=scheduled_at"`
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
			RequiredPermission("cron.run.query"),
		FindOne: crud.NewFindOne[cron.Run, RunSearch]().
			RequiredPermission("cron.run.query"),
	}
}

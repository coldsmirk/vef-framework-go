package monitor

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/monitor"
	"github.com/coldsmirk/vef-framework-go/result"
)

// defaultRateLimit is the default rate limit configuration for monitor endpoints.
var defaultRateLimit = &api.RateLimitConfig{Max: 60}

// NewResource creates a new monitor resource with the provided service. The
// stream inspector is optional — nil when the redis_stream transport is off —
// and gates the event-streams endpoint; the integration stats inspector is
// optional likewise — nil when the integration module is off.
func NewResource(service monitor.Service, streams event.StreamInspector, integrationStats integration.StatsInspector) api.Resource {
	return &Resource{
		service:          service,
		streams:          streams,
		integrationStats: integrationStats,
		Resource: api.NewRPCResource(
			"sys/monitor",
			api.WithOperations(
				api.OperationSpec{Action: "get_overview", RateLimit: defaultRateLimit},
				api.OperationSpec{Action: "get_cpu", RateLimit: defaultRateLimit},
				api.OperationSpec{Action: "get_memory", RateLimit: defaultRateLimit},
				api.OperationSpec{Action: "get_disk", RateLimit: defaultRateLimit},
				api.OperationSpec{Action: "get_network", RateLimit: defaultRateLimit},
				api.OperationSpec{Action: "get_host", RateLimit: defaultRateLimit},
				api.OperationSpec{Action: "get_process", RateLimit: defaultRateLimit},
				api.OperationSpec{Action: "get_load", RateLimit: defaultRateLimit},
				api.OperationSpec{Action: "get_build_info", RateLimit: defaultRateLimit},
				api.OperationSpec{Action: "get_event_streams", RateLimit: defaultRateLimit},
				api.OperationSpec{Action: "get_integration_stats", RateLimit: defaultRateLimit},
			),
		),
	}
}

// Resource handles system monitoring-related API endpoints.
type Resource struct {
	api.Resource

	service          monitor.Service
	streams          event.StreamInspector
	integrationStats integration.StatsInspector
}

// GetOverview returns a comprehensive system overview.
func (r *Resource) GetOverview(ctx fiber.Ctx) error {
	overview, err := r.service.Overview(ctx.Context())
	if err != nil {
		return err
	}

	return result.Ok(overview).Response(ctx)
}

// GetCPU returns detailed CPU information.
func (r *Resource) GetCPU(ctx fiber.Ctx) error {
	cpuInfo, err := r.service.CPU(ctx.Context())
	if err != nil {
		return monitor.ErrNotReady
	}

	return result.Ok(cpuInfo).Response(ctx)
}

// GetMemory returns memory usage information.
func (r *Resource) GetMemory(ctx fiber.Ctx) error {
	memInfo, err := r.service.Memory(ctx.Context())
	if err != nil {
		logger.Errorf("Failed to collect memory info: %v", err)

		return monitor.ErrCollectionFailed
	}

	return result.Ok(memInfo).Response(ctx)
}

// GetDisk returns disk usage and partition information.
func (r *Resource) GetDisk(ctx fiber.Ctx) error {
	diskInfo, err := r.service.Disk(ctx.Context())
	if err != nil {
		logger.Errorf("Failed to collect disk info: %v", err)

		return monitor.ErrCollectionFailed
	}

	return result.Ok(diskInfo).Response(ctx)
}

// GetNetwork returns network interface and I/O statistics.
func (r *Resource) GetNetwork(ctx fiber.Ctx) error {
	netInfo, err := r.service.Network(ctx.Context())
	if err != nil {
		logger.Errorf("Failed to collect network info: %v", err)

		return monitor.ErrCollectionFailed
	}

	return result.Ok(netInfo).Response(ctx)
}

// GetHost returns static host information.
func (r *Resource) GetHost(ctx fiber.Ctx) error {
	hostInfo, err := r.service.Host(ctx.Context())
	if err != nil {
		logger.Errorf("Failed to collect host info: %v", err)

		return monitor.ErrCollectionFailed
	}

	return result.Ok(hostInfo).Response(ctx)
}

// GetProcess returns information about the current process.
func (r *Resource) GetProcess(ctx fiber.Ctx) error {
	procInfo, err := r.service.Process(ctx.Context())
	if err != nil {
		return monitor.ErrNotReady
	}

	return result.Ok(procInfo).Response(ctx)
}

// GetLoad returns system load averages.
func (r *Resource) GetLoad(ctx fiber.Ctx) error {
	loadInfo, err := r.service.Load(ctx.Context())
	if err != nil {
		logger.Errorf("Failed to collect load info: %v", err)

		return monitor.ErrCollectionFailed
	}

	return result.Ok(loadInfo).Response(ctx)
}

// GetBuildInfo returns application build information.
func (r *Resource) GetBuildInfo(ctx fiber.Ctx) error {
	return result.Ok(r.service.BuildInfo()).Response(ctx)
}

// GetIntegrationStats reports per-node integration invocation statistics so
// operators can watch external-system health (see monitor.IntegrationStatsInfo).
func (r *Resource) GetIntegrationStats(ctx fiber.Ctx) error {
	info := &monitor.IntegrationStatsInfo{Stats: []integration.InvocationStats{}}
	if r.integrationStats == nil {
		return result.Ok(info).Response(ctx)
	}

	info.Enabled = true
	if stats := r.integrationStats.Stats(); len(stats) > 0 {
		info.Stats = stats
	}

	return result.Ok(info).Response(ctx)
}

// GetEventStreams reports cross-process event stream and consumer-group
// state so operators can spot orphaned groups (see monitor.EventStreamsInfo).
func (r *Resource) GetEventStreams(ctx fiber.Ctx) error {
	info := &monitor.EventStreamsInfo{Streams: []event.StreamInfo{}}
	if r.streams == nil {
		return result.Ok(info).Response(ctx)
	}

	streams, err := r.streams.Streams(ctx.Context())
	if err != nil {
		logger.Errorf("Failed to inspect event streams: %v", err)

		return monitor.ErrCollectionFailed
	}

	info.Enabled = true
	if len(streams) > 0 {
		info.Streams = streams
	}

	return result.Ok(info).Response(ctx)
}

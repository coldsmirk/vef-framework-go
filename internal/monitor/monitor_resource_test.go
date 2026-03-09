package monitor_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/monitor"
	"github.com/coldsmirk/vef-framework-go/security"
)

type MonitorResourceTestSuite struct {
	apptest.Suite

	ctx     context.Context
	service monitor.Service
	token   string
}

func (suite *MonitorResourceTestSuite) SetupSuite() {
	suite.T().Log("Setting up MonitorResourceTestSuite - starting test app")

	suite.ctx = context.Background()

	monitorConfig := &config.MonitorConfig{
		SampleInterval: 100 * time.Millisecond,
		SampleDuration: 50 * time.Millisecond,
	}

	buildInfo := &monitor.BuildInfo{
		AppVersion: "v1.0.0-test",
		BuildTime:  "2024-01-01T00:00:00Z",
		GitCommit:  "test123abc",
	}

	suite.SetupApp(
		fx.Replace(
			&config.DataSourceConfig{
				Kind: "sqlite",
			},
			monitorConfig,
			&security.JWTConfig{
				Secret:   security.DefaultJWTSecret,
				Audience: "test_app",
			},
		),
		fx.Supply(buildInfo),
		fx.Populate(&suite.service),
	)

	suite.token = suite.GenerateToken(&security.Principal{
		ID:   "test-admin",
		Name: "admin",
	})

	time.Sleep(100 * time.Millisecond)
}

func (suite *MonitorResourceTestSuite) TearDownSuite() {
	suite.T().Log("Tearing down MonitorResourceTestSuite")
	suite.TearDownApp()
}

func (suite *MonitorResourceTestSuite) TestGetOverview() {
	suite.T().Log("Testing get_overview endpoint")

	suite.Run("Success", func() {
		resp := suite.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/monitor",
				Action:   "get_overview",
				Version:  "v1",
			},
		}, suite.token)

		suite.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Overview request should succeed")

		data := suite.ReadDataAsMap(body.Data)

		suite.Contains(data, "host", "Should have host info")
		suite.Contains(data, "cpu", "Should have CPU info")
		suite.Contains(data, "memory", "Should have memory info")
		suite.Contains(data, "disk", "Should have disk info")
		suite.Contains(data, "network", "Should have network info")
		suite.Contains(data, "process", "Should have process info")
		suite.Contains(data, "load", "Should have load info")
		suite.Contains(data, "build", "Should have build info")

		buildInfo := suite.ReadDataAsMap(data["build"])
		suite.Equal("v1.0.0-test", buildInfo["appVersion"], "AppVersion should match")
		suite.NotEmpty(buildInfo["vefVersion"], "VEFVersion should be populated")
		suite.Equal("2024-01-01T00:00:00Z", buildInfo["buildTime"], "BuildTime should match")
		suite.Equal("test123abc", buildInfo["gitCommit"], "GitCommit should match")
	})
}

func (suite *MonitorResourceTestSuite) TestGetCPU() {
	suite.T().Log("Testing get_cpu endpoint")

	suite.Run("Success", func() {
		resp := suite.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/monitor",
				Action:   "get_cpu",
				Version:  "v1",
			},
		}, suite.token)

		suite.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "CPU request should succeed")

		data := suite.ReadDataAsMap(body.Data)

		suite.Contains(data, "physicalCores", "Should have physical cores")
		suite.Contains(data, "logicalCores", "Should have logical cores")
		suite.Contains(data, "modelName", "Should have model name")
		suite.Contains(data, "usagePercent", "Should have usage percent")
		suite.Contains(data, "totalPercent", "Should have total percent")

		physicalCores, ok := data["physicalCores"].(float64)
		suite.True(ok, "Physical cores should be a number")
		suite.Greater(physicalCores, float64(0), "Should have at least 1 physical core")

		logicalCores, ok := data["logicalCores"].(float64)
		suite.True(ok, "Logical cores should be a number")
		suite.GreaterOrEqual(logicalCores, physicalCores, "Logical cores should be >= physical cores")
	})
}

func (suite *MonitorResourceTestSuite) TestGetMemory() {
	suite.T().Log("Testing get_memory endpoint")

	suite.Run("Success", func() {
		resp := suite.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/monitor",
				Action:   "get_memory",
				Version:  "v1",
			},
		}, suite.token)

		suite.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Memory request should succeed")

		data := suite.ReadDataAsMap(body.Data)

		suite.Contains(data, "virtual", "Should have virtual memory info")

		virtual := suite.ReadDataAsMap(data["virtual"])
		suite.Contains(virtual, "total", "Should have total memory")
		suite.Contains(virtual, "used", "Should have used memory")
		suite.Contains(virtual, "usedPercent", "Should have used percent")

		total, ok := virtual["total"].(float64)
		suite.True(ok, "Total should be a number")
		suite.Greater(total, float64(0), "Total memory should be > 0")
	})
}

func (suite *MonitorResourceTestSuite) TestGetDisk() {
	suite.T().Log("Testing get_disk endpoint")

	suite.Run("Success", func() {
		resp := suite.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/monitor",
				Action:   "get_disk",
				Version:  "v1",
			},
		}, suite.token)

		suite.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Disk request should succeed")

		data := suite.ReadDataAsMap(body.Data)

		suite.Contains(data, "partitions", "Should have partitions")

		partitions, ok := data["partitions"].([]any)
		suite.True(ok, "Partitions should be an array")
		suite.NotEmpty(partitions, "Should have at least one partition")

		firstPartition := suite.ReadDataAsMap(partitions[0])
		suite.Contains(firstPartition, "mountPoint", "Should have mount point")
		suite.Contains(firstPartition, "total", "Should have total size")
	})
}

func (suite *MonitorResourceTestSuite) TestGetNetwork() {
	suite.T().Log("Testing get_network endpoint")

	suite.Run("Success", func() {
		resp := suite.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/monitor",
				Action:   "get_network",
				Version:  "v1",
			},
		}, suite.token)

		suite.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Network request should succeed")

		data := suite.ReadDataAsMap(body.Data)

		suite.Contains(data, "interfaces", "Should have interfaces")

		interfaces, ok := data["interfaces"].([]any)
		suite.True(ok, "Interfaces should be an array")
		suite.NotEmpty(interfaces, "Should have at least one network interface")

		firstInterface := suite.ReadDataAsMap(interfaces[0])
		suite.Contains(firstInterface, "name", "Should have interface name")
		suite.NotEmpty(firstInterface["name"], "Interface name should not be empty")
	})
}

func (suite *MonitorResourceTestSuite) TestGetHost() {
	suite.T().Log("Testing get_host endpoint")

	suite.Run("Success", func() {
		resp := suite.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/monitor",
				Action:   "get_host",
				Version:  "v1",
			},
		}, suite.token)

		suite.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Host request should succeed")

		data := suite.ReadDataAsMap(body.Data)

		suite.Contains(data, "hostname", "Should have hostname")
		suite.Contains(data, "os", "Should have OS")
		suite.Contains(data, "platform", "Should have platform")

		suite.NotEmpty(data["hostname"], "Hostname should not be empty")
		suite.NotEmpty(data["os"], "OS should not be empty")
	})

	suite.Run("ConsistentResults", func() {
		resp1 := suite.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/monitor",
				Action:   "get_host",
				Version:  "v1",
			},
		}, suite.token)

		body1 := suite.ReadResult(resp1)
		data1 := suite.ReadDataAsMap(body1.Data)

		resp2 := suite.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/monitor",
				Action:   "get_host",
				Version:  "v1",
			},
		}, suite.token)

		body2 := suite.ReadResult(resp2)
		data2 := suite.ReadDataAsMap(body2.Data)

		suite.Equal(data1["hostname"], data2["hostname"], "Hostname should be consistent")
		suite.Equal(data1["os"], data2["os"], "OS should be consistent")
	})
}

func (suite *MonitorResourceTestSuite) TestGetProcess() {
	suite.T().Log("Testing get_process endpoint")

	suite.Run("Success", func() {
		resp := suite.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/monitor",
				Action:   "get_process",
				Version:  "v1",
			},
		}, suite.token)

		suite.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Process request should succeed")

		data := suite.ReadDataAsMap(body.Data)

		suite.Contains(data, "pid", "Should have PID")
		suite.Contains(data, "name", "Should have process name")
		suite.Contains(data, "cpuPercent", "Should have CPU percent")
		suite.Contains(data, "memoryPercent", "Should have memory percent")
		suite.Contains(data, "memoryRss", "Should have memory RSS")

		pid, ok := data["pid"].(float64)
		suite.True(ok, "PID should be a number")
		suite.Greater(pid, float64(0), "PID should be > 0")

		suite.NotEmpty(data["name"], "Process name should not be empty")
	})
}

func (suite *MonitorResourceTestSuite) TestGetLoad() {
	suite.T().Log("Testing get_load endpoint")

	suite.Run("Success", func() {
		resp := suite.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/monitor",
				Action:   "get_load",
				Version:  "v1",
			},
		}, suite.token)

		suite.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Load request should succeed")

		data := suite.ReadDataAsMap(body.Data)

		suite.Contains(data, "load1", "Should have 1-minute load")
		suite.Contains(data, "load5", "Should have 5-minute load")
		suite.Contains(data, "load15", "Should have 15-minute load")

		load1, ok := data["load1"].(float64)
		suite.True(ok, "Load1 should be a number")
		suite.GreaterOrEqual(load1, float64(0), "Load1 should be >= 0")
	})
}

func (suite *MonitorResourceTestSuite) TestGetBuildInfo() {
	suite.T().Log("Testing get_build_info endpoint")

	suite.Run("Success", func() {
		resp := suite.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/monitor",
				Action:   "get_build_info",
				Version:  "v1",
			},
		}, suite.token)

		suite.Equal(200, resp.StatusCode, "Should return 200 OK")

		body := suite.ReadResult(resp)
		suite.True(body.IsOk(), "Build info request should succeed")

		data := suite.ReadDataAsMap(body.Data)

		suite.Equal("v1.0.0-test", data["appVersion"], "AppVersion should match")
		suite.NotEmpty(data["vefVersion"], "VEFVersion should be populated")
		suite.Equal("2024-01-01T00:00:00Z", data["buildTime"], "BuildTime should match")
		suite.Equal("test123abc", data["gitCommit"], "GitCommit should match")
	})
}

// TestMonitorResourceTestSuite tests monitor resource test suite functionality.
func TestMonitorResource(t *testing.T) {
	suite.Run(t, new(MonitorResourceTestSuite))
}

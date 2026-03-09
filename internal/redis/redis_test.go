package redis

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

type RedisTestSuite struct {
	suite.Suite

	ctx            context.Context
	redisContainer *testx.RedisContainer
}

func (suite *RedisTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.redisContainer = testx.NewRedisContainer(suite.ctx, suite.T())
}

func (suite *RedisTestSuite) TestNewClient() {
	suite.T().Log("Testing Redis client creation")

	suite.Run("DefaultConfiguration", func() {
		cfg := &config.RedisConfig{
			Host:     "127.0.0.1",
			Port:     6379,
			Database: 0,
		}

		client := NewClient(cfg, &config.AppConfig{
			Name: "test-app",
		})
		suite.Require().NotNil(client, "Client should be created")

		options := client.Options()
		suite.Equal("test-app", options.ClientName, "Client name should match app name")
		suite.Equal("vef", options.IdentitySuffix, "Identity suffix should be VEF name")
		suite.Equal(3, options.Protocol, "Protocol should be RESP3")
		suite.Equal("127.0.0.1:6379", options.Addr, "Address should match config")
		suite.Equal(0, options.DB, "Database should be 0")
		suite.True(options.ContextTimeoutEnabled, "Context timeout should be enabled")

		suite.Greater(options.PoolSize, 0, "Pool size should be positive")
		suite.Greater(options.PoolTimeout, time.Duration(0), "Pool timeout should be positive")
		suite.Greater(options.MaxRetries, 0, "Max retries should be positive")
		suite.Greater(options.MinIdleConns, 0, "Min idle connections should be positive")
		suite.Greater(options.ConnMaxLifetime, time.Duration(0), "Connection max lifetime should be positive")
		suite.Greater(options.ConnMaxIdleTime, time.Duration(0), "Connection max idle time should be positive")

		suite.T().Logf("Redis client created - Pool size: %d, Pool timeout: %v",
			options.PoolSize, options.PoolTimeout)
	})

	suite.Run("CustomConfiguration", func() {
		cfg := &config.RedisConfig{
			Host:     "custom-host",
			Port:     6380,
			User:     "testuser",
			Password: "testpass",
			Database: 5,
			Network:  "tcp",
		}

		client := NewClient(cfg, &config.AppConfig{
			Name: "custom-app",
		})
		suite.Require().NotNil(client, "Client should be created")

		options := client.Options()
		suite.Equal("custom-app", options.ClientName, "Client name should match app name")
		suite.Equal("custom-host:6380", options.Addr, "Address should match custom config")
		suite.Equal("testuser", options.Username, "Username should match config")
		suite.Equal("testpass", options.Password, "Password should match config")
		suite.Equal(5, options.DB, "Database should be 5")
		suite.Equal("tcp", options.Network, "Network should be tcp")

		suite.T().Logf("Custom client created - Addr: %s, DB: %d", options.Addr, options.DB)
	})
}

func (suite *RedisTestSuite) TestRedisConnection() {
	suite.T().Log("Testing Redis connection and operations")

	cfg := suite.redisContainer.Redis
	client := NewClient(cfg, &config.AppConfig{
		Name: "test-connection",
	})

	suite.Require().NotNil(client, "Client should be created")
	defer client.Close()

	suite.T().Logf("Redis connection config: Host=%s, Port=%d, DB=%d",
		cfg.Host, cfg.Port, cfg.Database)

	suite.Run("PingConnection", func() {
		err := client.Ping(suite.ctx).Err()
		suite.NoError(err, "PING should succeed")
	})

	suite.Run("StringOperations", func() {
		err := client.Set(suite.ctx, "test_key", "test_value", 0).Err()
		suite.NoError(err, "SET should succeed")

		val, err := client.Get(suite.ctx, "test_key").Result()
		suite.NoError(err, "GET should succeed")
		suite.Equal("test_value", val, "Value should match")

		err = client.Del(suite.ctx, "test_key").Err()
		suite.NoError(err, "DEL should succeed")

		_, err = client.Get(suite.ctx, "test_key").Result()
		suite.Error(err, "GET should fail for deleted key")

		suite.T().Log("String operations completed successfully")
	})

	suite.Run("HashOperations", func() {
		err := client.HSet(suite.ctx, "test_hash", "field1", "value1").Err()
		suite.NoError(err, "HSET should succeed")

		hashVal, err := client.HGet(suite.ctx, "test_hash", "field1").Result()
		suite.NoError(err, "HGET should succeed")
		suite.Equal("value1", hashVal, "Hash value should match")

		err = client.Del(suite.ctx, "test_hash").Err()
		suite.NoError(err, "DEL hash should succeed")

		suite.T().Log("Hash operations completed successfully")
	})
}

func (suite *RedisTestSuite) TestHealthCheck() {
	suite.T().Log("Testing Redis health check")

	suite.Run("HealthCheckSuccess", func() {
		cfg := suite.redisContainer.Redis
		client := NewClient(cfg, &config.AppConfig{
			Name: "test-health",
		})

		suite.Require().NotNil(client, "Client should be created")
		defer client.Close()

		err := HealthCheck(suite.ctx, client)
		suite.NoError(err, "Health check should succeed for valid connection")

		suite.T().Log("Health check passed")
	})

	suite.Run("HealthCheckFailure", func() {
		cfg := &config.RedisConfig{
			Host:     "invalid-host",
			Port:     9999,
			Database: 0,
		}

		client := NewClient(cfg, &config.AppConfig{
			Name: "test-health-fail",
		})

		suite.Require().NotNil(client, "Client should be created")
		defer client.Close()

		err := HealthCheck(suite.ctx, client)
		suite.Error(err, "Health check should fail for invalid connection")

		suite.T().Log("Health check failed as expected")
	})
}

// TestBuildRedisAddr tests build redis addr functionality.
func TestBuildRedisAddr(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.RedisConfig
		expected string
	}{
		{
			name: "DefaultHostAndPort",
			config: &config.RedisConfig{
				Host: "",
				Port: 0,
			},
			expected: "127.0.0.1:6379",
		},
		{
			name: "CustomHostAndPort",
			config: &config.RedisConfig{
				Host: "redis.example.com",
				Port: 6380,
			},
			expected: "redis.example.com:6380",
		},
		{
			name: "CustomHostWithDefaultPort",
			config: &config.RedisConfig{
				Host: "localhost",
				Port: 0,
			},
			expected: "localhost:6379",
		},
		{
			name: "DefaultHostWithCustomPort",
			config: &config.RedisConfig{
				Host: "",
				Port: 6380,
			},
			expected: "127.0.0.1:6380",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := buildRedisAddr(tt.config)
			assert.Equal(t, tt.expected, addr, "Address should match expected format")
		})
	}
}

// TestGetPoolSize tests get pool size functionality.
func TestGetPoolSize(t *testing.T) {
	poolSize := getPoolSize()

	assert.GreaterOrEqual(t, poolSize, 4, "Pool size should be at least 4")
	assert.LessOrEqual(t, poolSize, 100, "Pool size should be at most 100")

	t.Logf("Calculated pool size: %d", poolSize)
}

// TestGetConnectionConfig tests get connection config functionality.
func TestGetConnectionConfig(t *testing.T) {
	tests := []struct {
		name     string
		poolSize int
	}{
		{"SmallPool", 4},
		{"MediumPool", 10},
		{"LargePool", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poolTimeout, idleTimeout, maxRetries := getConnectionConfig(tt.poolSize)

			assert.GreaterOrEqual(t, poolTimeout, 1*time.Second,
				"Pool timeout should be at least 1 second")
			assert.LessOrEqual(t, poolTimeout, 5*time.Second,
				"Pool timeout should be at most 5 seconds")
			assert.Equal(t, 5*time.Minute, idleTimeout,
				"Idle timeout should be 5 minutes")
			assert.Equal(t, 3, maxRetries,
				"Max retries should be 3")

			t.Logf("Pool size: %d - Pool timeout: %v, Idle timeout: %v, Max retries: %d",
				tt.poolSize, poolTimeout, idleTimeout, maxRetries)
		})
	}
}

// TestRedisTestSuite tests redis test suite functionality.
func TestRedis(t *testing.T) {
	suite.Run(t, new(RedisTestSuite))
}

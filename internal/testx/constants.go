package testx

import "time"

// Container images.
const (
	PostgresImage = "postgres:18-alpine"
	MySQLImage    = "mysql:lts"
	RedisImage    = "redis:8-alpine"
	MinIOImage    = "minio/minio:latest"
)

// Database credentials.
const (
	TestDatabaseName = "testdb"
	TestUsername     = "testuser"
	TestPassword     = "testpass"
)

// MinIO credentials.
const (
	TestMinIOAccessKey = "testadmin"
	TestMinIOSecretKey = "testadmin"
	TestMinIOBucket    = "testbucket"
)

// DefaultContainerTimeout is the maximum wait time for container readiness.
const DefaultContainerTimeout = 30 * time.Second

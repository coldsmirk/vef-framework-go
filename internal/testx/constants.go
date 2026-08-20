package testx

import "time"

// Container images.
const (
	PostgresImage = "postgres:18-alpine"
	// Pinned to the major line like the others rather than :lts, which tracks
	// whichever release MySQL currently designates and has already carried the
	// tag across a major version — a jump that would land on an unrelated
	// commit with nothing to point at.
	MySQLImage = "mysql:9"
	RedisImage = "redis:8-alpine"
	// Pinned to a release rather than :latest so a CI run reproduces: MinIO's
	// health endpoints and startup timing have changed between releases, and a
	// moving tag turns that into an unexplained flake on an unrelated commit.
	MinIOImage = "minio/minio:RELEASE.2025-09-07T16-13-09Z"
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

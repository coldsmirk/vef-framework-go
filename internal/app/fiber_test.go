package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestCreateFiberAppDefaultBodyLimit(t *testing.T) {
	app, err := createFiberApp(&config.AppConfig{})
	require.NoError(t, err, "Fiber app should be created with default config")

	assert.Equal(t, 32*1024*1024, app.Config().BodyLimit, "Default body limit should fit MinIO multipart chunks")
}

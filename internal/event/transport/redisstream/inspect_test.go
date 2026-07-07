package redisstream

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goredis "github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/event/transport/redisstream"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

func newInspectorClient(t *testing.T) *goredis.Client {
	t.Helper()

	container := testx.NewRedisContainer(context.Background(), t)
	client := goredis.NewClient(&goredis.Options{
		Addr: net.JoinHostPort(container.Redis.Host, strconv.Itoa(int(container.Redis.Port))),
	})
	t.Cleanup(func() { _ = client.Close() })

	return client
}

func TestInspectorStreams(t *testing.T) {
	ctx := context.Background()
	client := newInspectorClient(t)
	cfg := redisstream.Config{StreamPrefix: "inspect:test:"}
	inspector := NewInspector(client, cfg)

	t.Run("EmptyKeyspace", func(t *testing.T) {
		infos, err := inspector.Streams(ctx)
		require.NoError(t, err, "Inspecting an empty keyspace should succeed")
		assert.Empty(t, infos, "No streams should be reported before anything is published")
	})

	t.Run("ReportsGroupsAndSkipsForeignKeys", func(t *testing.T) {
		stream := cfg.StreamKey("some.event")

		for range 2 {
			require.NoError(t, client.XAdd(ctx, &goredis.XAddArgs{
				Stream: stream,
				Values: map[string]any{"frame": "{}"},
			}).Err(), "Seeding stream entries should succeed")
		}

		require.NoError(t, client.XGroupCreateMkStream(ctx, stream, "inspect:group", "0").Err(),
			"Creating the consumer group should succeed")

		// One read without ack: the group gains a consumer record and one
		// pending entry, both of which must surface in the report.
		_, err := client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    "inspect:group",
			Consumer: "inspect-consumer",
			Streams:  []string{stream, ">"},
			Count:    1,
			Block:    -1,
		}).Result()
		require.NoError(t, err, "Reading one entry into the group should succeed")

		// A stream outside the transport prefix must not be listed.
		require.NoError(t, client.XAdd(ctx, &goredis.XAddArgs{
			Stream: "foreign:stream",
			Values: map[string]any{"x": "y"},
		}).Err(), "Seeding the foreign stream should succeed")

		infos, err := inspector.Streams(ctx)
		require.NoError(t, err, "Inspection should succeed")
		require.Len(t, infos, 1, "Only streams under the transport prefix should be listed")

		info := infos[0]
		assert.Equal(t, stream, info.Stream, "Stream key should be reported verbatim")
		assert.EqualValues(t, 2, info.Length, "Stream length should match the seeded entries")
		require.Len(t, info.Groups, 1, "The consumer group should be listed")

		group := info.Groups[0]
		assert.Equal(t, "inspect:group", group.Name, "Group name should be reported")
		assert.EqualValues(t, 1, group.Consumers, "The reading consumer should be counted")
		assert.EqualValues(t, 1, group.Pending, "The unacknowledged entry should be pending")
		assert.NotEmpty(t, group.LastDeliveredID, "Last delivered ID should be populated after a read")
	})
}

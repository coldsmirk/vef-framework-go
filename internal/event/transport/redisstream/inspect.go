package redisstream

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/transport/redisstream"
)

// Inspector implements event.StreamInspector over the transport's Redis
// keyspace. It is independent of a running Transport — inspection only
// needs the client and the stream prefix — so the monitor endpoint can
// observe streams and groups regardless of what this process subscribes to.
type Inspector struct {
	client *goredis.Client
	cfg    redisstream.Config
}

// NewInspector constructs an Inspector sharing the transport's client and
// configuration.
func NewInspector(client *goredis.Client, cfg redisstream.Config) *Inspector {
	return &Inspector{client: client, cfg: cfg}
}

// Streams lists every stream under the configured prefix with its consumer
// groups.
func (i *Inspector) Streams(ctx context.Context) ([]event.StreamInfo, error) {
	keys, err := scanStreamKeys(ctx, i.client, i.cfg.EffectiveStreamPrefix())
	if err != nil {
		return nil, err
	}

	infos := make([]event.StreamInfo, 0, len(keys))

	for _, key := range keys {
		length, err := i.client.XLen(ctx, key).Result()
		if err != nil {
			return nil, fmt.Errorf("redis_stream: xlen %s: %w", key, err)
		}

		groups, err := i.client.XInfoGroups(ctx, key).Result()
		if err != nil {
			return nil, fmt.Errorf("redis_stream: xinfo groups %s: %w", key, err)
		}

		info := event.StreamInfo{
			Stream: key,
			Length: length,
			Groups: make([]event.StreamGroupInfo, 0, len(groups)),
		}

		for _, g := range groups {
			info.Groups = append(info.Groups, event.StreamGroupInfo{
				Name:            g.Name,
				Consumers:       g.Consumers,
				Pending:         g.Pending,
				Lag:             g.Lag,
				LastDeliveredID: g.LastDeliveredID,
			})
		}

		infos = append(infos, info)
	}

	return infos, nil
}

// scanStreamKeys collects every stream-typed key under the prefix via
// cursor-based SCAN, so inspection never blocks Redis the way KEYS would.
func scanStreamKeys(ctx context.Context, client *goredis.Client, prefix string) ([]string, error) {
	var (
		keys   []string
		cursor uint64
	)

	for {
		batch, next, err := client.ScanType(ctx, cursor, prefix+"*", 128, "stream").Result()
		if err != nil {
			return nil, fmt.Errorf("redis_stream: scan streams: %w", err)
		}

		keys = append(keys, batch...)

		if next == 0 {
			return keys, nil
		}

		cursor = next
	}
}

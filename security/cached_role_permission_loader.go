package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/event"
	ilog "github.com/coldsmirk/vef-framework-go/internal/log"
	"github.com/coldsmirk/vef-framework-go/log"
)

// eventTypeRolePermissionsChanged is the event type for role permissions changes.
// When this event is published, the entire role permissions cache will be cleared.
const eventTypeRolePermissionsChanged = "vef.security.role_permissions.changed"

// RolePermissionsChangedEvent is published when role permissions are modified.
type RolePermissionsChangedEvent struct {
	event.BaseEvent

	Roles []string `json:"roles"` // Affected role names (empty means all roles)
}

// PublishRolePermissionsChangedEvent publishes a role permissions changed event via the provided publisher.
// If no roles are specified, subscribers should interpret the event as affecting all roles.
func PublishRolePermissionsChangedEvent(publisher event.Publisher, roles ...string) {
	publisher.Publish(&RolePermissionsChangedEvent{
		BaseEvent: event.NewBaseEvent(eventTypeRolePermissionsChanged),

		Roles: roles,
	})
}

// CachedRolePermissionsLoader is a decorator that adds caching to a RolePermissionsLoader.
// It uses the cache system and event bus for automatic cache invalidation.
type CachedRolePermissionsLoader struct {
	loader    RolePermissionsLoader
	permCache cache.Cache[map[string]DataScope]
	logger    log.Logger
}

// NewCachedRolePermissionsLoader creates a new cached role permissions loader.
// It automatically subscribes to role permissions change events to invalidate cache.
func NewCachedRolePermissionsLoader(
	loader RolePermissionsLoader,
	eventBus event.Subscriber,
) RolePermissionsLoader {
	cached := &CachedRolePermissionsLoader{
		loader:    loader,
		permCache: cache.NewMemory[map[string]DataScope](),
		logger:    ilog.Named("security:cached_role_permissions_loader"),
	}

	// Subscribe to role permissions change events
	eventBus.Subscribe(eventTypeRolePermissionsChanged, cached.handlePermissionsChanged)

	return cached
}

func (c *CachedRolePermissionsLoader) handlePermissionsChanged(ctx context.Context, evt event.Event) {
	changeEvent, ok := evt.(*RolePermissionsChangedEvent)
	if !ok {
		c.logger.Errorf("Received invalid event type: %T", evt)

		return
	}

	// Empty roles means clear all cache
	if len(changeEvent.Roles) == 0 {
		if err := c.permCache.Clear(ctx); err != nil {
			c.logger.Errorf("Failed to clear all role permissions cache: %v", err)
		} else {
			c.logger.Info("Cleared all role permissions cache")
		}

		return
	}

	// Clear cache for specific roles
	for _, role := range changeEvent.Roles {
		if err := c.permCache.Delete(ctx, role); err != nil {
			c.logger.Errorf("Failed to delete cache for role %s: %v", role, err)
		} else {
			c.logger.Infof("Cleared cache for role: %s", role)
		}
	}
}

func (c *CachedRolePermissionsLoader) LoadPermissions(ctx context.Context, role string) (map[string]DataScope, error) {
	return c.permCache.GetOrLoad(ctx, role, func(ctx context.Context) (map[string]DataScope, error) {
		return c.loader.LoadPermissions(ctx, role)
	})
}

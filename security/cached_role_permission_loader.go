package security

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/event"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// eventTypeRolePermissionsChanged is the event type for role permissions changes.
// When this event is published, the entire role permissions cache will be cleared.
const eventTypeRolePermissionsChanged = "vef.security.role_permissions.changed"

// RolePermissionsChangedEvent is published when role permissions are modified.
type RolePermissionsChangedEvent struct {
	Roles []string `json:"roles"` // Affected role names (empty means all roles)
}

// EventType implements event.Event.
func (*RolePermissionsChangedEvent) EventType() string { return eventTypeRolePermissionsChanged }

// PublishRolePermissionsChangedEvent publishes a role permissions changed event.
// If no roles are specified, subscribers should interpret the event as affecting all roles.
func PublishRolePermissionsChangedEvent(ctx context.Context, bus event.Bus, roles ...string) error {
	return bus.Publish(ctx, &RolePermissionsChangedEvent{Roles: roles})
}

// CachedRolePermissionsLoader is a decorator that adds caching to a RolePermissionsLoader.
// It uses the cache system and event bus for automatic cache invalidation.
type CachedRolePermissionsLoader struct {
	loader    RolePermissionsLoader
	permCache cache.Cache[map[string]DataScope]
	logger    logx.Logger
}

// NewCachedRolePermissionsLoader creates a new cached role permissions loader.
// It automatically subscribes to role permissions change events to invalidate cache.
func NewCachedRolePermissionsLoader(
	loader RolePermissionsLoader,
	bus event.Bus,
) RolePermissionsLoader {
	cached := &CachedRolePermissionsLoader{
		loader:    loader,
		permCache: cache.NewMemory[map[string]DataScope](),
		logger:    ilogx.Named("security:cached_role_permissions_loader"),
	}

	if _, err := event.SubscribeTyped[*RolePermissionsChangedEvent](bus, cached.handlePermissionsChanged); err != nil {
		panic(fmt.Errorf("subscribe role_permissions.changed: %w", err))
	}

	return cached
}

func (c *CachedRolePermissionsLoader) handlePermissionsChanged(ctx context.Context, evt *RolePermissionsChangedEvent, _ event.Envelope) error {
	// Empty roles means clear all cache
	if len(evt.Roles) == 0 {
		if err := c.permCache.Clear(ctx); err != nil {
			c.logger.Errorf("Failed to clear all role permissions cache: %v", err)
			return err
		}
		c.logger.Info("Cleared all role permissions cache")
		return nil
	}

	// Clear cache for specific roles
	for _, role := range evt.Roles {
		if err := c.permCache.Delete(ctx, role); err != nil {
			c.logger.Errorf("Failed to delete cache for role %s: %v", role, err)
			return err
		}
		c.logger.Infof("Cleared cache for role: %s", role)
	}
	return nil
}

func (c *CachedRolePermissionsLoader) LoadPermissions(ctx context.Context, role string) (map[string]DataScope, error) {
	return c.permCache.GetOrLoad(ctx, role, func(ctx context.Context) (map[string]DataScope, error) {
		return c.loader.LoadPermissions(ctx, role)
	})
}

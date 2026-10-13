package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/security"
)

var errResolverDown = errors.New("resolver down")

// routingOnlyResolver resolves globals but exposes none of them to forms — the
// shape of every resolver written before FormGlobalsExposer existed.
type routingOnlyResolver struct {
	calls int
}

func (r *routingOnlyResolver) Resolve(context.Context, *security.Principal, string) (map[string]any, error) {
	r.calls++

	return map[string]any{"region": "north"}, nil
}

// exposingResolver exposes the keys it is given and records what it was asked.
type exposingResolver struct {
	keys        []string
	globals     map[string]any
	err         error
	calls       int
	seenFlow    string
	seenUserID  string
	keysForFlow string
}

func (r *exposingResolver) Resolve(_ context.Context, principal *security.Principal, flowCode string) (map[string]any, error) {
	r.calls++
	r.seenFlow = flowCode
	r.seenUserID = principal.ID

	return r.globals, r.err
}

func (r *exposingResolver) FormGlobalKeys(flowCode string) []string {
	r.keysForFlow = flowCode

	return r.keys
}

func TestFormGlobalsServiceVisible(t *testing.T) {
	t.Run("ExposesNothingWithoutTheExtension", func(t *testing.T) {
		svc := service.NewFormGlobalsService(new(routingOnlyResolver))

		assert.Nil(t, svc.Visible("leave", map[string]any{"region": "north"}), "A resolver that never opted in must expose no global to forms")
	})

	t.Run("KeepsOnlyTheExposedKeys", func(t *testing.T) {
		resolver := &exposingResolver{keys: []string{"region"}}
		svc := service.NewFormGlobalsService(resolver)

		visible := svc.Visible("leave", map[string]any{"region": "north", "quotaLimit": 8000})

		assert.Equal(t, map[string]any{"region": "north"}, visible, "Only the exposed key should reach the form")
		assert.Equal(t, "leave", resolver.keysForFlow, "The exposed keys should be asked for the instance's own flow")
	})

	t.Run("IgnoresExposedKeysTheSnapshotLacks", func(t *testing.T) {
		svc := service.NewFormGlobalsService(&exposingResolver{keys: []string{"region", "title"}})

		assert.Equal(t, map[string]any{"region": "north"}, svc.Visible("leave", map[string]any{"region": "north"}), "A missing key should be skipped, not emitted as nil")
	})

	t.Run("ReturnsNilWhenNothingIsVisible", func(t *testing.T) {
		svc := service.NewFormGlobalsService(&exposingResolver{keys: []string{"title"}})

		assert.Nil(t, svc.Visible("leave", map[string]any{"region": "north"}), "No visible key should yield nil so the DTO omits the field")
	})

	t.Run("ReturnsNilForAnEmptySnapshot", func(t *testing.T) {
		svc := service.NewFormGlobalsService(&exposingResolver{keys: []string{"region"}})

		assert.Nil(t, svc.Visible("leave", nil), "An instance without globals should expose none")
	})
}

func TestFormGlobalsServiceResolve(t *testing.T) {
	principal := &security.Principal{ID: "user-1"}

	t.Run("SkipsTheResolverWhenNothingIsExposed", func(t *testing.T) {
		resolver := new(routingOnlyResolver)
		svc := service.NewFormGlobalsService(resolver)

		globals, err := svc.Resolve(context.Background(), principal, "leave")

		require.NoError(t, err, "Nothing exposed should not be an error")
		assert.Nil(t, globals, "Nothing exposed should yield no globals")
		assert.Zero(t, resolver.calls, "A host that never opted in must not pay a resolver call per start form")
	})

	t.Run("ResolvesForTheCallerAndFilters", func(t *testing.T) {
		resolver := &exposingResolver{keys: []string{"region"}, globals: map[string]any{"region": "north", "quotaLimit": 8000}}
		svc := service.NewFormGlobalsService(resolver)

		globals, err := svc.Resolve(context.Background(), principal, "leave")

		require.NoError(t, err, "Resolution should succeed")
		assert.Equal(t, map[string]any{"region": "north"}, globals, "Only the exposed key should be returned")
		assert.Equal(t, "leave", resolver.seenFlow, "The resolver should be told which flow is being started")
		assert.Equal(t, "user-1", resolver.seenUserID, "The resolver should be handed the caller")
	})

	t.Run("WrapsTheResolverError", func(t *testing.T) {
		svc := service.NewFormGlobalsService(&exposingResolver{keys: []string{"region"}, err: errResolverDown})

		_, err := svc.Resolve(context.Background(), principal, "leave")

		require.ErrorIs(t, err, errResolverDown, "The resolver's own error should stay inspectable")
	})
}

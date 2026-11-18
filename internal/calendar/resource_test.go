package calendar_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	vef "github.com/coldsmirk/vef-framework-go"
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/calendar"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/timex"
)

type calendarPermission struct{}

func (calendarPermission) HasPermission(_ context.Context, p *security.Principal, permission string) (bool, error) {
	return p.ID == "manager" && permission == "sys.calendar.manage", nil
}

type hostLoader struct{}

func (hostLoader) LoadDays(_ context.Context, start, end timex.Date) ([]calendar.Day, error) {
	var days []calendar.Day
	for d := start; !d.After(end); d = d.AddDays(1) {
		days = append(days, calendar.Day{Date: d, Kind: calendar.Holiday})
	}

	return days, nil
}

func TestCalendarRPC(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
	}{{"HostedSource", false}, {"OverrideStore", true}} {
		t.Run(tc.name, func(t *testing.T) {
			var app apptest.Suite
			app.SetT(t)

			opts := []fx.Option{
				fx.Replace(&config.CalendarConfig{Store: config.CalendarStoreConfig{Enabled: tc.enabled, AutoMigrate: true}}),
				fx.Decorate(func() security.PermissionChecker { return calendarPermission{} }),
			}
			if !tc.enabled {
				opts = append(opts, vef.ProvideCalendarDayLoader(func() calendar.DayLoader { return hostLoader{} }))
			}

			app.SetupApp(opts...)
			t.Cleanup(app.TearDownApp)
			user := app.GenerateToken(security.NewUser("reader", "Reader"))
			manager := app.GenerateToken(security.NewUser("manager", "Manager"))
			call := func(action, token string, params api.Params) result.Result {
				return app.ReadResult(app.MakeRPCRequestWithToken(api.Request{Resource: "sys/calendar", Action: action, Version: "v1", Params: params}, token))
			}
			dates := api.Params{"startsAt": "2024-01-01", "endsAt": "2024-12-31"}
			assert.False(t, call("find_days", "", dates).IsOk(), "day reads require login")
			r := call("find_days", user, dates)
			require.True(t, r.IsOk(), "ordinary user should read 366 days: %s", r.Message)
			days := app.ReadDataAsSlice(r.Data)
			require.Len(t, days, 366, "inclusive leap year should fit")

			if !tc.enabled {
				assert.Equal(t, "holiday", app.ReadDataAsMap(days[0])["kind"], "host loader should replace baseline")
			}

			dates["endsAt"] = "2025-01-01"
			assert.Equal(t, result.ErrCodeBadRequest, call("find_days", user, dates).Code, "367 days exceed the limit")

			rule := map[string]any{"timezone": "UTC", "windows": []any{map[string]any{"times": []any{map[string]any{"startsAt": "00:00:00", "endsAt": "01:00:00"}}}}}
			params := api.Params{"rule": rule, "from": 0, "limit": 1}
			r = call("preview_windows", user, params)
			require.True(t, r.IsOk(), "ordinary user should preview a rule: %s", r.Message)
			data := app.ReadDataAsMap(r.Data)
			assert.Len(t, app.ReadDataAsSlice(data["windows"]), 1, "preview should honor limit")
			status := app.ReadDataAsMap(data["status"])
			assert.Equal(t, true, status["open"], "explicit Unix epoch should be preserved")
			assert.Contains(t, status, "next", "preview limit should not truncate status")

			params["limit"] = 51
			assert.Equal(t, result.ErrCodeBadRequest, call("preview_windows", user, params).Code, "limit above 50 should fail")
			delete(params, "limit")
			r = call("preview_windows", user, params)
			require.True(t, r.IsOk(), "omitting the limit should succeed")
			assert.Len(t, app.ReadDataAsSlice(app.ReadDataAsMap(r.Data)["windows"]), 10, "default preview size should be ten")

			save := api.Params{"overrides": []map[string]string{{"date": "2024-01-01", "kind": "holiday", "name": "New Year"}}}

			remove := api.Params{"dates": []string{"2024-01-01"}}
			if !tc.enabled {
				assert.Equal(t, result.ErrCodeNotFound, call("save_overrides", manager, save).Code, "disabled store should omit save")
				assert.Equal(t, result.ErrCodeNotFound, call("delete_overrides", manager, remove).Code, "disabled store should omit delete")

				return
			}

			assert.Equal(t, result.ErrCodeAccessDenied, call("save_overrides", user, save).Code, "ordinary user cannot save")
			assert.Equal(t, result.ErrCodeAccessDenied, call("delete_overrides", user, remove).Code, "ordinary user cannot delete")
			assert.True(t, call("save_overrides", manager, save).IsOk(), "management permission allows batch save")
			assert.True(t, call("delete_overrides", manager, remove).IsOk(), "management permission allows delete")

			for _, bad := range []map[string]string{
				{"kind": "workday"}, {"date": "2025-01-01", "kind": "unknown"},
			} {
				assert.Equal(t, result.ErrCodeBadRequest, call("save_overrides", manager, api.Params{"overrides": []map[string]string{bad}}).Code, "each override should validate")
			}

			save["overrides"] = make([]map[string]string, 401)
			assert.Equal(t, result.ErrCodeBadRequest, call("save_overrides", manager, save).Code, "batch must not exceed 400 overrides")
		})
	}
}

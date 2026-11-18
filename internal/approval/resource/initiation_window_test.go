package resource_test

import (
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/calendar"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func (s *InstanceResourceTestSuite) TestInitiationWindow() {
	flow := s.approval
	call := func(resource, action string, p api.Params) result.Result {
		return s.ReadResult(s.MakeRPCRequestWithToken(api.Request{Resource: resource, Action: action, Version: "v1", Params: p}, s.token))
	}
	update := func(rule any) result.Result {
		return call(
			"approval/flow",
			"update",
			api.Params{
				"flowId":                 flow.FlowID,
				"name":                   "Approval Flow",
				"bindingMode":            "standalone",
				"isAllInitiationAllowed": true,
				"instanceTitleTemplate":  "Approval Flow {{.instanceNo}}",
				"initiationWindow":       rule,
			},
		)
	}
	s.T().Cleanup(func() { s.Require().True(update(nil).IsOk(), "flow should return to its unrestricted state") })

	empty := calendar.WindowRule{}
	invalid := call(
		"approval/flow",
		"create",
		api.Params{
			"tenantId":               "default",
			"categoryId":             s.categoryID,
			"code":                   "invalid-window",
			"name":                   "Invalid window",
			"bindingMode":            "standalone",
			"isAllInitiationAllowed": true,
			"instanceTitleTemplate":  "Window",
			"initiationWindow":       empty,
		},
	)
	s.Equal(calendar.ErrCodeInvalidWindowRule, invalid.Code, "create must validate a supplied rule")
	s.Equal(calendar.ErrCodeInvalidWindowRule, update(empty).Code, "update must validate a supplied rule")

	today := timex.NowDate()
	soon, distant := today.AddDays(2), today.AddDays(400)
	closed := calendar.WindowRule{Windows: []calendar.Window{{Dates: &calendar.DateRange{StartsAt: soon, EndsAt: soon}}}}
	beyondHorizon := calendar.WindowRule{Windows: []calendar.Window{{Dates: &calendar.DateRange{StartsAt: distant, EndsAt: distant}}}}
	opened := calendar.WindowRule{Windows: []calendar.Window{{}}}

	for _, tc := range []struct {
		name string
		rule *calendar.WindowRule
		open bool
		next string
	}{
		{"Unrestricted", nil, true, ""},
		{"Open", &opened, true, ""},
		{"Closed", &closed, false, soon.String() + " 00:00"},
		{"BeyondHorizon", &beyondHorizon, false, ""},
	} {
		s.Run(tc.name, func() {
			saved := update(tc.rule)
			s.Require().True(saved.IsOk(), "rule should save: %s", saved.Message)

			res := call("approval/my", "find_available_flows", api.Params{"keyword": "Approval Flow", "page": 1, "pageSize": 50})
			s.Require().True(res.IsOk(), "flow list should load")

			var item map[string]any
			for _, v := range s.ReadDataAsSlice(s.ReadDataAsMap(res.Data)["items"]) {
				candidate := s.ReadDataAsMap(v)
				if candidate["flowId"] == flow.FlowID {
					item = candidate

					break
				}
			}

			s.Require().NotNil(item, "a closed flow must remain listed")

			if tc.rule == nil {
				s.NotContains(item, "initiationWindow", "unrestricted list item omits status")
			} else {
				status := s.ReadDataAsMap(item["initiationWindow"])
				s.Equal(tc.open, status["open"], "list should expose actual availability")

				if !tc.open && tc.next == "" {
					s.NotContains(status, "next", "openings beyond the horizon should be omitted")
				}
			}

			form := call("approval/my", "get_start_form", api.Params{"flowCode": flow.FlowCode})

			start := s.rpcCall("start", api.Params{"flowCode": flow.FlowCode})
			if !tc.open {
				s.Equal(approval.ErrCodeInitiationWindowClosed, form.Code, "closed form must be denied")
				s.Equal(approval.ErrCodeInitiationWindowClosed, start.Code, "closed start must be denied")

				message, err := i18n.Te(shared.ErrMessageInitiationWindowClosed, map[string]any{"next": tc.next})
				s.Require().NoError(err, "the closed-window message should exist for the active language")
				s.Equal(message, form.Message, "the form should explain when initiation can reopen")
				s.Equal(message, start.Message, "starting should return the same localized opening information")

				return
			}

			s.Require().True(form.IsOk(), "open or unrestricted form should load")
			s.Require().True(start.IsOk(), "open or unrestricted start should succeed")

			if tc.rule == nil {
				s.NotContains(s.ReadDataAsMap(form.Data), "initiationWindow", "unrestricted form omits status")
			} else {
				status := s.ReadDataAsMap(s.ReadDataAsMap(form.Data)["initiationWindow"])
				s.Equal(true, status["open"], "open form should expose status")
				s.Contains(status, "current", "current interval should expose its closing instant")
			}
		})
	}

	s.Run("ResubmitWhileClosed", func() {
		s.Require().True(update(opened).IsOk(), "the flow should be open for the initial submission")
		instance := s.startInstance(flow.FlowCode, nil)
		s.Require().True(s.rpcCall("withdraw", api.Params{"instanceId": instance["id"], "reason": "Revise"}).IsOk(), "the applicant should be able to withdraw the instance")
		s.Require().True(update(closed).IsOk(), "the updated window should close the flow")
		s.Require().True(s.rpcCall("resubmit", api.Params{"instanceId": instance["id"]}).IsOk(), "the applicant should still be able to resubmit")
	})
}

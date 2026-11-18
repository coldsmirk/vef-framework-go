package resource_test

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

type bookingHandler struct{ blocked atomic.Bool }

func (*bookingHandler) Describe() approval.BusinessKindDescriptor {
	return approval.BusinessKindDescriptor{Kind: "booking", Label: "Booking"}
}

func (*bookingHandler) Submit(_ context.Context, _ orm.DB, sc *approval.SubmissionContext) (*approval.Submission, error) {
	return &approval.Submission{BusinessRef: sc.InstanceID}, nil
}

func (*bookingHandler) View(context.Context, orm.DB, *approval.ViewContext) (any, error) {
	return nil, nil
}

func (*bookingHandler) OnTaskAction(context.Context, orm.DB, *approval.TaskActionContext) (*approval.TaskActionOutcome, error) {
	return nil, nil
}

func (h *bookingHandler) PlanVoid(_ context.Context, _ orm.DB, vc *approval.VoidContext) (*approval.VoidPlan, error) {
	p := &approval.VoidPlan{Reversals: []string{"Release booking"}, FollowUps: []string{"Notify guest"}}
	if h.blocked.Load() {
		p.Blockers = []string{"Already checked in"}
	}

	if release, _ := vc.Options["release"].(bool); release {
		p.Reversals = append(p.Reversals, "Release deposit")
	}

	return p, nil
}
func (*bookingHandler) Void(context.Context, orm.DB, *approval.VoidContext) error { return nil }

func (s *InstanceResourceTestSuite) voidRPC(resource, action string, params api.Params, token string) result.Result {
	res := s.MakeRPCRequestWithToken(api.Request{Resource: "approval/" + resource, Action: action, Version: "v1", Params: params}, token)
	s.Require().Equal(http.StatusOK, res.StatusCode, "The RPC should return an application result")

	return s.ReadResult(res)
}

func (s *InstanceResourceTestSuite) TestVoidPolicy() {
	req := s.createAndPublishFlow("policy-request", "Policy request picker", simpleFlowDef(), nil, approval.BusinessKindVoidRequest)

	create := func(code string, kind, policy, flowCode any) result.Result {
		return s.voidRPC(
			"flow",
			"create",
			api.Params{
				"categoryId":             s.categoryID,
				"tenantId":               testTenant,
				"code":                   code,
				"name":                   code,
				"isAllInitiationAllowed": true,
				"instanceTitleTemplate":  "Test",
				"businessKind":           kind,
				"voidPolicy":             policy,
				"voidFlowCode":           flowCode,
			},
			s.token,
		)
	}
	for _, tc := range []struct {
		name               string
		kind, policy, code any
	}{
		{"unknown", nil, "other", nil}, {"disabled-code", nil, "disabled", req.FlowCode}, {"direct-code", nil, "direct", req.FlowCode}, {"missing-code", nil, "approval", nil}, {"blank-code", nil, "approval", " "}, {"absent-flow", nil, "approval", "missing"}, {"wrong-kind", nil, "approval", s.simple.FlowCode}, {"unsupported", "invoice", "direct", nil}, {"recursive", approval.BusinessKindVoidRequest, "approval", req.FlowCode},
	} {
		s.Run(tc.name, func() {
			r := create("invalid-"+tc.name, tc.kind, tc.policy, tc.code)
			s.Equal(approval.ErrCodeVoidPolicyInvalid, r.Code, "The unsupported policy must fail at save: %v", r)
		})
	}

	r := create("policy-good", "booking", "approval", req.FlowCode)
	s.Require().True(r.IsOk(), "The target supports reversal and the request kind resolves: %v", r)
	r = s.voidRPC("flow", "list_business_kinds", nil, s.token)
	s.Require().True(r.IsOk(), "The designer may query the catalog")

	for _, item := range s.ReadDataAsSlice(r.Data) {
		d := s.ReadDataAsMap(item)
		s.Equal(d["kind"] == "booking", d["isVoidable"], "Voidability is derived from the handler")
	}

	r = s.voidRPC(
		"flow",
		"find_flows",
		api.Params{"businessKind": approval.BusinessKindVoidRequest, "tenantId": testTenant, "includeShared": true, "page": 1, "pageSize": 1, "keyword": "Policy request picker"},
		s.token,
	)
	s.Require().True(r.IsOk(), "The designer can combine kind and tenant filters with paging")
	items := s.ReadDataAsSlice(s.ReadDataAsMap(r.Data)["items"])
	s.Require().Len(items, 1, "The selected request flow matches")
	s.Equal(req.FlowID, s.ReadDataAsMap(items[0])["id"], "Only the requested kind and code should match")
	r = s.voidRPC(
		"flow",
		"update",
		api.Params{
			"flowId":                 req.FlowID,
			"name":                   "Converted request",
			"voidPolicy":             "approval",
			"voidFlowCode":           req.FlowCode,
			"isAllInitiationAllowed": true,
			"instanceTitleTemplate":  "New form",
		},
		s.token,
	)
	s.Equal(approval.ErrCodeVoidPolicyInvalid, r.Code, "An edited request cannot authorize itself using its previous kind")
}

func (s *InstanceResourceTestSuite) TestVoidLifecycle() {
	request := s.createAndPublishFlow("reverse-booking", "Request reversal", approvalFlowDef(), nil, approval.BusinessKindVoidRequest)
	alt := s.createAndPublishFlow("other-request", "Other reversal", approvalFlowDef(), nil, approval.BusinessKindVoidRequest)
	target := s.createAndPublishFlow("void-booking", "Booking", simpleFlowDef(), nil, "booking")
	update := func(policy approval.VoidPolicy) {
		p := api.Params{"flowId": target.FlowID, "name": "Booking", "businessKind": "booking", "instanceTitleTemplate": "Booking", "isAllInitiationAllowed": true, "voidPolicy": policy}
		if policy == approval.VoidApproval {
			p["voidFlowCode"] = request.FlowCode
		}

		r := s.voidRPC("flow", "update", p, s.token)
		s.Require().True(r.IsOk(), "The live void policy should update: %v", r)
	}
	detail := func(resource, id string) map[string]any {
		r := s.voidRPC(resource, "get_instance_detail", api.Params{"instanceId": id}, s.token)
		s.Require().True(r.IsOk(), "Should read %s detail: %v", resource, r)

		return s.ReadDataAsMap(r.Data)
	}
	requester := func(id, code, reason string) result.Result {
		return s.rpcCall(
			"start",
			api.Params{"flowCode": code, "formData": api.Params{"targetInstanceId": id, "reason": reason, "attachments": []string{"proof"}, "options": api.Params{"release": true}}},
		)
	}
	preview := func(resource, id, token string) result.Result {
		return s.voidRPC(resource, "plan_void", api.Params{"instanceId": id, "options": api.Params{"release": true}}, token)
	}
	direct := func(id string) result.Result {
		return s.adminRPCCall("void_instance", api.Params{"instanceId": id, "reason": "Reverse booking", "attachments": []string{"file"}, "options": api.Params{"release": true}})
	}
	taskID := func(id string) string {
		tasks := s.findPendingTasks(id)
		s.Require().Len(tasks, 1, "The request should have one actionable seat")

		return tasks[0]["id"].(string)
	}
	s.Run("Direct", func() {
		id := s.startInstance(target.FlowCode, nil)["id"].(string)
		before := s.loadInstance(id)
		s.Equal(approval.ErrCodeVoidNotAllowed, direct(id).Code, "Disabled is the default")
		update(approval.VoidDirect)
		s.Contains(detail("admin", id)["availableActions"], "void", "Current policy enables direct reversal")

		s.voidBooking.blocked.Store(true)
		defer s.voidBooking.blocked.Store(false)

		pr := preview("admin", id, s.token)
		s.Require().True(pr.IsOk(), "Preview displays business blockers: %v", pr)
		s.Contains(s.ReadDataAsMap(s.ReadDataAsMap(pr.Data)["plan"])["blockers"], "Already checked in", "Preview should explain the blocker")

		r := direct(id)
		s.Equal(approval.ErrCodeVoidBlocked, r.Code, "Write rejects a blocked plan")
		s.Contains(r.Message, "Already checked in", "The refusal includes the blocker")
		s.Equal(approval.InstanceApproved, s.loadInstance(id).Status, "Preview and refused void leave the approval intact")
		s.voidBooking.blocked.Store(false)

		foreign := newTenantUser("admin-away", "Other admin")
		foreign.Details = api.Params{"tenant_id": "elsewhere"}
		s.Equal(approval.ErrCodeInstanceNotFound, preview("admin", id, s.GenerateToken(foreign)).Code, "Foreign admin preview conceals the target")
		r = s.voidRPC("admin", "void_instance", api.Params{"instanceId": id, "reason": "outside"}, s.GenerateToken(foreign))
		s.Equal(approval.ErrCodeInstanceNotFound, r.Code, "Foreign admin cannot reverse")
		r = direct(id)
		s.Require().True(r.IsOk(), "Direct void should succeed: %v", r)
		s.Equal(before.FinishedAt, s.loadInstance(id).FinishedAt, "The original completion time is preserved")

		for _, role := range []string{"admin", "my"} {
			d := detail(role, id)
			rec := s.ReadDataAsMap(d["voidRecord"])
			s.Equal("voided", rec["status"], "Both views expose the executed reversal")
			s.Equal([]any{"Release booking", "Release deposit"}, rec["reversals"], "The record pins the executed plan")
			s.NotEmpty(rec["voidedAt"], "The record has its own void timestamp")
			s.NotContains(d, "voidRequest", "Direct void has no request")
			tl := s.ReadDataAsSlice(d["timeline"])
			s.Equal("void", s.ReadDataAsMap(tl[len(tl)-1])["kind"], "Void appears after the completed flow")
		}

		listed := s.adminRPCCall("find_instances", api.Params{"tenantId": testTenant, "flowId": target.FlowID, "status": "voided"})
		s.Require().True(listed.IsOk(), "Voided is a valid instance status filter")
		items := s.ReadDataAsSlice(s.ReadDataAsMap(listed.Data)["items"])
		s.Require().Len(items, 1, "The reversed target matches the status filter")
		s.Equal(id, s.ReadDataAsMap(items[0])["instanceId"], "The voided list links to the target")
		s.Equal(approval.ErrCodeInstanceNotVoidable, direct(id).Code, "A void cannot be repeated")
		s.Equal(approval.ErrCodeInstanceNotVoidable, preview("admin", id, s.token).Code, "A voided target cannot be previewed")
	})
	s.Run("Request", func() {
		update(approval.VoidApproval)

		id := s.startInstance(target.FlowCode, nil)["id"].(string)
		info := detail("my", id)
		s.Contains(info["availableActions"], "request_void", "Applicant can initiate the configured flow")
		s.Equal(request.FlowCode, s.ReadDataAsMap(info["instance"])["voidFlowCode"], "The request drawer gets the live flow code")

		foreign := newTenantUser("foreign", "Foreign")
		foreign.Details = api.Params{"tenant_id": "other"}
		s.Equal(approval.ErrCodeInstanceNotFound, preview("my", id, s.GenerateToken(foreign)).Code, "A requester stays within their own write tenant")

		restrict := func(allowAll bool) {
			p := api.Params{
				"flowId":                 request.FlowID,
				"name":                   "Request reversal",
				"businessKind":           approval.BusinessKindVoidRequest,
				"instanceTitleTemplate":  "Request reversal",
				"isAllInitiationAllowed": allowAll,
			}
			if !allowAll {
				p["initiators"] = []any{api.Params{"kind": "user", "ids": []string{"manager-1"}}}
			}

			r := s.voidRPC("flow", "update", p, s.token)
			s.Require().True(r.IsOk(), "Should change the request initiation rules: %v", r)
		}
		restrict(false)
		s.Equal(approval.ErrCodeNotAllowedInitiate, preview("my", id, s.token).Code, "The request flow controls who may preview")
		s.NotContains(detail("my", id)["availableActions"], "request_void", "Participant without initiation permission cannot request void")
		restrict(true)
		s.Equal(approval.ErrCodeVoidNotAllowed, requester(id, alt.FlowCode, "Wrong flow").Code, "Cannot substitute a more permissive flow")

		s.voidBooking.blocked.Store(true)
		defer s.voidBooking.blocked.Store(false)

		pr := preview("my", id, s.token)
		s.Require().True(pr.IsOk(), "Requester sees the blocked plan")
		s.Contains(s.ReadDataAsMap(s.ReadDataAsMap(pr.Data)["plan"])["blockers"], "Already checked in", "Preview reports business blockers")
		s.Equal(approval.ErrCodeVoidBlocked, requester(id, request.FlowCode, "Blocked").Code, "An actual request honors blockers")
		s.voidBooking.blocked.Store(false)

		r := requester(id, request.FlowCode, "First reason")
		s.Require().True(r.IsOk(), "The configured request should start: %v", r)
		reqID := s.ReadDataAsMap(r.Data)["id"].(string)
		s.Equal(approval.ErrCodeVoidRequestPending, requester(id, request.FlowCode, "Second").Code, "A second request is refused")
		s.Equal(approval.ErrCodeVoidRequestPending, preview("my", id, s.token).Code, "A running request blocks preview")

		for _, role := range []string{"my", "admin"} {
			d := detail(role, id)
			summary := s.ReadDataAsMap(d["voidRequest"])
			s.Equal(reqID, summary["requestInstanceId"], "Summary links to the request")
			s.Equal("running", summary["requestStatus"], "Summary uses request instance status")
			s.NotEmpty(summary["requestInstanceNo"], "Summary names the request")
			s.NotEmpty(summary["createdAt"], "Summary gives the submission time")
			s.Equal("test-admin", s.ReadDataAsMap(summary["applicant"])["id"], "Summary identifies the request applicant")
			s.NotContains(d["availableActions"], "request_void", "Pending request cannot be duplicated")
		}

		bv := s.ReadDataAsMap(detail("admin", reqID)["businessView"])
		s.Equal(id, s.ReadDataAsMap(bv["target"])["instanceId"], "The request view identifies the target")
		s.Equal("First reason", s.ReadDataAsMap(bv["record"])["reason"], "The request view includes the saved reason")
		s.Contains(s.ReadDataAsMap(bv["plan"])["reversals"], "Release deposit", "The request view plans with its submitted options")
		startNode := s.findNodeIDByKind(s.loadInstance(reqID).FlowVersionID, approval.NodeStart)
		r = s.rpcCall("process_task", api.Params{"taskId": taskID(reqID), "action": "rollback", "opinion": "Clarify", "targetNodeId": startNode})
		s.Require().True(r.IsOk(), "Should return the request for revision: %v", r)
		s.Equal("returned", s.ReadDataAsMap(detail("my", id)["voidRequest"])["requestStatus"], "Return keeps the request pending")

		tomorrow := timex.NowDate().AddDays(2).String()
		setWindow := func(window any) {
			v := s.voidRPC(
				"flow",
				"update",
				api.Params{
					"flowId":                 request.FlowID,
					"name":                   "Void request",
					"businessKind":           approval.BusinessKindVoidRequest,
					"instanceTitleTemplate":  "Void request",
					"isAllInitiationAllowed": true,
					"initiationWindow":       window,
				},
				s.token,
			)
			s.Require().True(v.IsOk(), "Should update the window: %v", v)
		}
		setWindow(api.Params{"windows": []any{api.Params{"dates": api.Params{"startsAt": tomorrow, "endsAt": tomorrow}}}})

		changed := s.voidRPC(
			"flow",
			"update",
			api.Params{
				"flowId":                 target.FlowID,
				"name":                   "Booking",
				"businessKind":           "booking",
				"instanceTitleTemplate":  "Booking",
				"isAllInitiationAllowed": true,
				"voidPolicy":             "approval",
				"voidFlowCode":           alt.FlowCode,
			},
			s.token,
		)
		s.Require().True(changed.IsOk(), "Administrator can change the required approval flow")
		s.Equal(approval.ErrCodeVoidNotAllowed, preview("my", id, s.token).Code, "The returned applicant remains subject to the configured flow code")
		update(approval.VoidApproval)
		s.True(preview("my", id, s.token).IsOk(), "Returned applicant can preview outside the window")
		s.Equal(approval.ErrCodeVoidRequestPending, preview("my", id, s.manager1Token).Code, "Other callers cannot bypass an existing request")
		r = s.rpcCall("resubmit", api.Params{"instanceId": reqID, "formData": api.Params{"targetInstanceId": id, "reason": "Corrected reason", "options": api.Params{"release": true}}})
		s.Require().True(r.IsOk(), "Returned applicant can revise outside the initiation window: %v", r)
		setWindow(nil)

		task := taskID(reqID)

		s.voidBooking.blocked.Store(true)
		r = s.processTask(task, "approve", "OK")
		s.Equal(approval.ErrCodeVoidBlocked, r.Code, "Approval rechecks the plan")
		s.Equal(approval.InstanceRunning, s.loadInstance(reqID).Status, "Blocked approval rolls the request back")
		s.Equal(approval.TaskPending, s.loadTask(task).Status, "The same reviewer may retry")
		s.Equal(approval.InstanceApproved, s.loadInstance(id).Status, "The target is unchanged")
		s.voidBooking.blocked.Store(false)
		r = s.processTask(task, "approve", "OK")
		s.Require().True(r.IsOk(), "Cleared blocker permits approval: %v", r)
		rec := s.ReadDataAsMap(detail("my", id)["voidRecord"])
		s.Equal(reqID, rec["requestInstanceId"], "Approval promotes the same request record")
		s.Equal("Corrected reason", rec["reason"], "The approved revision is retained")
		s.Equal(approval.InstanceApproved, s.loadInstance(reqID).Status, "Request and target commit together")
	})

	for _, end := range []string{"reject", "withdraw", "terminate"} {
		s.Run(end, func() {
			update(approval.VoidApproval)

			id := s.startInstance(target.FlowCode, nil)["id"].(string)
			r := requester(id, request.FlowCode, "Request")
			s.Require().True(r.IsOk(), "The request should start: %v", r)

			reqID := s.ReadDataAsMap(r.Data)["id"].(string)
			switch end {
			case "reject":
				task := taskID(reqID)
				for _, policy := range []approval.VoidPolicy{approval.VoidDisabled, approval.VoidDirect} {
					update(policy)

					r = s.processTask(task, "approve", "Approved")
					s.Equal(approval.ErrCodeVoidNotAllowed, r.Code, "Approval rechecks the changed policy")
					s.Equal(approval.TaskPending, s.loadTask(task).Status, "Policy refusal rolls back the decision")
					s.Equal("running", s.ReadDataAsMap(detail("admin", id)["voidRequest"])["requestStatus"], "The pending request stays visible under the new policy")

					if policy == approval.VoidDirect {
						s.Equal(approval.ErrCodeVoidRequestPending, direct(id).Code, "Changing to direct cannot bypass a pending request")
					}
				}

				r = s.processTask(task, "reject", "Policy changed")

			case "withdraw":
				r = s.rpcCall("withdraw", api.Params{"instanceId": reqID, "reason": "Cancel"})
			case "terminate":
				r = s.adminRPCCall("terminate_instance", api.Params{"instanceId": reqID, "reason": "Cancel"})
			}

			s.Require().True(r.IsOk(), "Closing a request should succeed: %v", r)
			bv := s.ReadDataAsMap(detail("admin", reqID)["businessView"])
			s.Equal("dismissed", s.ReadDataAsMap(bv["record"])["status"], "The audit row survives dismissal")
			update(approval.VoidApproval)
			s.NotContains(detail("admin", id), "voidRequest", "Dismissal clears the lock")

			if end == "withdraw" {
				r = s.rpcCall("resubmit", api.Params{"instanceId": reqID, "formData": api.Params{"targetInstanceId": id, "reason": "Reconsidered"}})
				s.Require().True(r.IsOk(), "A withdrawn applicant may resubmit the same request: %v", r)
				reopened := s.ReadDataAsMap(detail("admin", reqID)["businessView"])
				s.Equal(s.ReadDataAsMap(bv["record"])["id"], s.ReadDataAsMap(reopened["record"])["id"], "Resubmitting reopens the original record")
				s.Equal("requested", s.ReadDataAsMap(reopened["record"])["status"], "The resubmission reserves the target again")
				r = s.rpcCall("withdraw", api.Params{"instanceId": reqID, "reason": "Cancel"})
				s.Require().True(r.IsOk(), "Withdrawing again releases the reservation")
			}

			r = requester(id, request.FlowCode, "Replacement")
			s.Require().True(r.IsOk(), "A dismissed request permits a fresh request: %v", r)
		})
	}
}

func (s *PermissionEnforcementResourceTestSuite) TestVoidPermission() {
	for _, action := range []string{"plan_void", "void_instance"} {
		s.Run(action, func() {
			resp := s.MakeRPCRequestWithToken(
				api.Request{Resource: "approval/admin", Action: action, Version: "v1", Params: api.Params{"instanceId": "target", "reason": "example"}},
				s.token,
			)
			s.Require().Equal(http.StatusForbidden, resp.StatusCode, "Both administrative operations require the void permission")
			s.Equal(result.ErrCodeAccessDenied, s.ReadResult(resp).Code, "Permission gate runs before instance lookup")
		})
	}
}

package resource_test

import (
	"net/http"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/result"
)

func (s *InstanceResourceTestSuite) TestAnnotationWrites() {
	inst := s.startInstance(s.formField.FlowCode, map[string]any{"approver": "approver-2"})
	tasks := s.findPendingTasks(inst["id"].(string))
	s.Require().Len(tasks, 1, "The selected approver should have one pending task")
	params := map[string]any{
		"taskId":   tasks[0]["id"],
		"anchor":   map[string]any{"field": "approver"},
		"audience": map[string]any{"scope": "users", "userIds": []string{"manager-1"}},
		"content":  "Please confirm the approver",
		"operator": map[string]any{"id": "forged"},
	}
	res := s.rpcCall("save_annotation", params, s.approver2Token)
	s.Require().True(res.IsOk(), "The current task holder can save a draft: %v", res)
	a := s.ReadDataAsMap(res.Data)
	s.Equal("approver-2", a["author"].(map[string]any)["id"], "The principal supplies the author")
	s.Equal(params["anchor"], a["anchor"], "The field anchor survives the RPC round trip")
	s.Equal([]any{"manager-1"}, a["audience"].(map[string]any)["userIds"], "The user scope survives the RPC round trip")
	s.NotEmpty(a["nodeName"], "The saved view identifies the writing node")
	res = s.rpcCall("save_annotation", map[string]any{"id": a["id"], "content": "Revised"}, s.approver2Token)
	s.Require().True(res.IsOk(), "The author can update their draft: %v", res)
	s.Equal("Revised", s.ReadDataAsMap(res.Data)["content"], "The update returns the new content")
	res = s.rpcCall("delete_annotation", map[string]any{"id": a["id"]}, s.approver2Token)
	s.True(res.IsOk(), "The author can delete their draft: %v", res)
}

func (s *PermissionEnforcementResourceTestSuite) TestAnnotationPermission() {
	for _, action := range []string{"save_annotation", "delete_annotation"} {
		s.Run(action, func() {
			resp := s.MakeRPCRequestWithToken(api.Request{Resource: "approval/instance", Action: action, Version: "v1", Params: map[string]any{"id": "private", "content": "draft"}}, s.token)
			s.Require().Equal(http.StatusForbidden, resp.StatusCode, "The permission middleware must reject the write")
			s.Equal(result.ErrCodeAccessDenied, s.ReadResult(resp).Code, "The write requires approval.annotation.write")
		})
	}
}

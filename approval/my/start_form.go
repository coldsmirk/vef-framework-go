package my

import "encoding/json"

// StartForm is the pre-submission view of a flow for an applicant: the
// identity fields needed to render the initiation header plus the published
// version's host form-designer document, returned verbatim like the detail
// views. Loading it is gated exactly like starting the instance — active
// flow, initiation permission, published version — so a rendered form always
// implies a startable flow.
type StartForm struct {
	FlowID      string          `json:"flowId"`
	FlowCode    string          `json:"flowCode"`
	FlowName    string          `json:"flowName"`
	FlowIcon    *string         `json:"flowIcon,omitempty"`
	Description *string         `json:"description,omitempty"`
	VersionID   string          `json:"versionId"`
	Version     int             `json:"version"`
	FormSchema  json.RawMessage `json:"formSchema,omitempty"`
}

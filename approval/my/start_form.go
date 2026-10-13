package my

import (
	"encoding/json"

	"github.com/coldsmirk/vef-framework-go/approval"
)

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
	// Applicant is the snapshot the instance will record if the caller starts
	// it now, resolved as Start resolves it — the form's `$applicant`, so a
	// rule about the initiator evaluates at initiation exactly as it does
	// later on the instance's detail views.
	Applicant approval.UserInfo `json:"applicant"`
	// Globals are the form-visible host globals (approval.FormGlobalsExposer)
	// resolved for the caller — the form's `$globals`. Absent when the flow
	// exposes none.
	Globals map[string]any `json:"globals,omitempty"`
}

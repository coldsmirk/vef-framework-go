package my

// AvailableFlow describes a flow the current user is allowed to initiate.
type AvailableFlow struct {
	FlowID       string            `json:"flowId"`
	FlowCode     string            `json:"flowCode"`
	FlowName     string            `json:"flowName"`
	FlowIcon     *string           `json:"flowIcon,omitempty"`
	Description  *string           `json:"description,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	CategoryID   string            `json:"categoryId"`
	CategoryName string            `json:"categoryName"`
}

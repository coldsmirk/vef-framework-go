package my

// PendingCounts holds the counts of pending actions for the current user.
type PendingCounts struct {
	PendingTaskCount int `json:"pendingTaskCount"`
	UnreadCCCount    int `json:"unreadCcCount"`
}

package admin

import (
	"encoding/json"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// BusinessProjection is the operator-facing convergence state for one bound
// business record.
type BusinessProjection struct {
	ProjectionID           string                            `json:"projectionId"`
	TenantID               string                            `json:"tenantId"`
	FlowID                 string                            `json:"flowId"`
	FlowVersionID          string                            `json:"flowVersionId"`
	OwnerInstanceID        string                            `json:"ownerInstanceId"`
	AppliedOwnerInstanceID *string                           `json:"appliedOwnerInstanceId,omitempty"`
	BusinessTable          string                            `json:"businessTable"`
	RecordKey              json.RawMessage                   `json:"recordKey"`
	Consistency            config.ApprovalBindingConsistency `json:"consistency"`
	DesiredStatus          approval.InstanceStatus           `json:"desiredStatus"`
	DesiredStartedAt       timex.DateTime                    `json:"desiredStartedAt"`
	DesiredFinishedAt      *timex.DateTime                   `json:"desiredFinishedAt,omitempty"`
	DesiredRevision        int64                             `json:"desiredRevision"`
	AppliedRevision        int64                             `json:"appliedRevision"`
	Status                 approval.BindingProjectionStatus  `json:"status"`
	AttemptCount           int                               `json:"attemptCount"`
	NextAttemptAt          *timex.DateTime                   `json:"nextAttemptAt,omitempty"`
	LeaseUntil             *timex.DateTime                   `json:"leaseUntil,omitempty"`
	LastError              *string                           `json:"lastError,omitempty"`
	AppliedAt              *timex.DateTime                   `json:"appliedAt,omitempty"`
	UpdatedAt              timex.DateTime                    `json:"updatedAt"`
}

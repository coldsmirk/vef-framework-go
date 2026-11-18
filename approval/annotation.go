package approval

import (
	"context"
	"encoding/json"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

type AnnotationStatus string

const (
	AnnotationDraft     AnnotationStatus = "draft"
	AnnotationPublished AnnotationStatus = "published"
	AnnotationDiscarded AnnotationStatus = "discarded"
)

type AudienceScope string

const (
	AudienceParticipants AudienceScope = "participants"
	AudienceApplicant    AudienceScope = "applicant"
	AudienceNodes        AudienceScope = "nodes"
	AudienceUsers        AudienceScope = "users"
)

type AnnotationResolution string

const (
	ResolutionAddressed AnnotationResolution = "addressed"
	ResolutionDeclined  AnnotationResolution = "declined"
)

type AnnotationAudience struct {
	Scope   AudienceScope `json:"scope"`
	NodeIDs []string      `json:"nodeIds,omitempty"`
	UserIDs []string      `json:"userIds,omitempty"`
}
type FieldAnchor struct {
	Field  string `json:"field"`
	Row    *int   `json:"row,omitempty"`
	Column string `json:"column,omitempty"`
}
type Annotation struct {
	orm.BaseModel `bun:"table:apv_annotation,alias:aan"`
	orm.FullAuditedModel

	TenantID    string                `json:"tenantId"`
	InstanceID  string                `json:"instanceId"`
	NodeID      string                `json:"nodeId"`
	VisitID     *string               `json:"visitId,omitempty"`
	TaskID      *string               `json:"taskId,omitempty"`
	Author      UserInfo              `json:"author" bun:"embed:author_"`
	Status      AnnotationStatus      `json:"status"`
	Anchor      json.RawMessage       `json:"anchor,omitempty" bun:"type:jsonb,nullzero"`
	Content     string                `json:"content"`
	Audience    AnnotationAudience    `json:"audience" bun:"type:jsonb"`
	ParentID    *string               `json:"parentId,omitempty"`
	Resolution  *AnnotationResolution `json:"resolution,omitempty"`
	PublishedAt *timex.DateTime       `json:"publishedAt,omitempty"`
}

type AnnotationView struct {
	ID          string                `json:"id"`
	InstanceID  string                `json:"instanceId"`
	NodeID      string                `json:"nodeId"`
	NodeName    string                `json:"nodeName"`
	VisitID     *string               `json:"visitId,omitempty"`
	TaskID      *string               `json:"taskId,omitempty"`
	Author      UserInfo              `json:"author"`
	Status      AnnotationStatus      `json:"status"`
	Anchor      json.RawMessage       `json:"anchor,omitempty"`
	Content     string                `json:"content"`
	Audience    AnnotationAudience    `json:"audience"`
	ParentID    *string               `json:"parentId,omitempty"`
	Resolution  *AnnotationResolution `json:"resolution,omitempty"`
	PublishedAt *timex.DateTime       `json:"publishedAt,omitempty"`
	CreatedAt   timex.DateTime        `json:"createdAt"`
}

func (a *Annotation) View(nodeName string) AnnotationView {
	return AnnotationView{ID: a.ID, InstanceID: a.InstanceID, NodeID: a.NodeID, NodeName: nodeName, VisitID: a.VisitID, TaskID: a.TaskID, Author: a.Author, Status: a.Status, Anchor: a.Anchor, Content: a.Content, Audience: a.Audience, ParentID: a.ParentID, Resolution: a.Resolution, PublishedAt: a.PublishedAt, CreatedAt: a.CreatedAt}
}

type AnnotationAnchorValidator interface {
	// ValidateAnchor reports whether the author may annotate this location in the business record.
	ValidateAnchor(ctx context.Context, db orm.DB, ac *AnchorContext) error
}
type AnchorContext struct {
	Instance *Instance
	Node     *FlowNode
	Task     *Task
	Author   UserInfo
	Anchor   json.RawMessage
}

type SaveAnnotationInput struct {
	ID         string
	TaskID     string
	ParentID   string
	Anchor     json.RawMessage
	Content    string
	Audience   *AnnotationAudience
	Resolution *AnnotationResolution
	Operator   UserInfo
	Caller     CallerContext
}
type DeleteAnnotationInput struct {
	ID       string
	Operator UserInfo
	Caller   CallerContext
}

package service

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// ValidationService provides validation operations.
type ValidationService struct {
	assigneeService approval.AssigneeService
}

// NewValidationService creates a new ValidationService.
func NewValidationService(assigneeSvc approval.AssigneeService) *ValidationService {
	return &ValidationService{assigneeService: assigneeSvc}
}

// ValidateOpinion checks if an opinion is required but missing.
func (*ValidationService) ValidateOpinion(node *approval.FlowNode, opinion string) error {
	if node.IsOpinionRequired && strings.TrimSpace(opinion) == "" {
		return shared.ErrOpinionRequired
	}

	return nil
}

// ValidateFormData validates submitted form data against the published form schema.
func (*ValidationService) ValidateFormData(schema *approval.FormDefinition, formData map[string]any) error {
	if schema == nil || len(schema.Fields) == 0 {
		return nil
	}

	if formData == nil {
		formData = map[string]any{}
	}

	fieldByKey := make(map[string]approval.FormFieldDefinition, len(schema.Fields))
	for _, field := range schema.Fields {
		fieldByKey[field.Key] = field
	}

	for key := range formData {
		if _, ok := fieldByKey[key]; !ok {
			return newFormValidationError(fmt.Sprintf("字段 %s 未在表单定义中", key))
		}
	}

	for _, field := range schema.Fields {
		value, exists := formData[field.Key]
		if !exists || isEmptyFormValue(value) {
			if field.IsRequired {
				return newFormValidationError(fmt.Sprintf("字段 %s 为必填项", fieldLabel(field)))
			}

			continue
		}

		if err := validateFormField(field, value); err != nil {
			return err
		}
	}

	return nil
}

// ValidateRollbackTarget validates the rollback target node based on the node's RollbackType.
func (*ValidationService) ValidateRollbackTarget(ctx context.Context, db orm.DB, instance *approval.Instance, currentNode *approval.FlowNode, targetNodeID string) error {
	if targetNodeID == currentNode.ID {
		return shared.ErrInvalidRollbackTarget
	}

	switch currentNode.RollbackType {
	case approval.RollbackNone:
		return shared.ErrRollbackNotAllowed

	case approval.RollbackPrevious:
		count, err := db.NewSelect().
			Model((*approval.FlowEdge)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("source_node_id", targetNodeID).
					Equals("target_node_id", currentNode.ID).
					Equals("flow_version_id", instance.FlowVersionID)
			}).
			Count(ctx)
		if err != nil {
			return fmt.Errorf("find previous node: %w", err)
		}

		if count == 0 {
			return shared.ErrInvalidRollbackTarget
		}

	case approval.RollbackStart:
		var startNode approval.FlowNode

		if err := db.NewSelect().
			Model(&startNode).
			Select("id").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("flow_version_id", instance.FlowVersionID).
					Equals("kind", string(approval.NodeStart))
			}).
			Scan(ctx); err != nil {
			return fmt.Errorf("find start node: %w", err)
		}

		if startNode.ID != targetNodeID {
			return shared.ErrInvalidRollbackTarget
		}

	case approval.RollbackAny:
		count, err := db.NewSelect().
			Model((*approval.FlowNode)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", targetNodeID).
					Equals("flow_version_id", instance.FlowVersionID)
			}).
			Count(ctx)
		if err != nil {
			return fmt.Errorf("find rollback target node: %w", err)
		}

		if count == 0 {
			return shared.ErrInvalidRollbackTarget
		}

	case approval.RollbackSpecified:
		var targetNode approval.FlowNode

		if err := db.NewSelect().
			Model(&targetNode).
			Select("key").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", targetNodeID).
					Equals("flow_version_id", instance.FlowVersionID)
			}).
			Scan(ctx); err != nil {
			return shared.ErrInvalidRollbackTarget
		}

		if !slices.Contains(currentNode.RollbackTargetKeys, targetNode.Key) {
			return shared.ErrInvalidRollbackTarget
		}
	}

	return nil
}

func validateFormField(field approval.FormFieldDefinition, value any) error {
	switch field.Kind {
	case approval.FieldInput, approval.FieldTextarea, approval.FieldDate:
		text, ok := value.(string)
		if !ok {
			return newFormValidationError(fmt.Sprintf("字段 %s 类型无效，应为字符串", fieldLabel(field)))
		}

		return validateStringRule(field, text)

	case approval.FieldUpload:
		return validateUploadField(field, value)

	case approval.FieldNumber:
		number, ok := toFloat64(value)
		if !ok {
			return newFormValidationError(fmt.Sprintf("字段 %s 类型无效，应为数字", fieldLabel(field)))
		}

		return validateNumberRule(field, number)

	case approval.FieldSelect:
		return validateSelectField(field, value)

	default:
		return nil
	}
}

func validateStringRule(field approval.FormFieldDefinition, value string) error {
	if field.Validation == nil {
		return nil
	}

	if field.Validation.MinLength != nil && len(value) < *field.Validation.MinLength {
		return newFormValidationError(fmt.Sprintf("字段 %s 长度不能小于 %d", fieldLabel(field), *field.Validation.MinLength))
	}

	if field.Validation.MaxLength != nil && len(value) > *field.Validation.MaxLength {
		return newFormValidationError(fmt.Sprintf("字段 %s 长度不能大于 %d", fieldLabel(field), *field.Validation.MaxLength))
	}

	if field.Validation.Pattern != "" {
		matched, err := regexp.MatchString(field.Validation.Pattern, value)
		if err != nil {
			return newFormValidationError(fmt.Sprintf("字段 %s 校验规则无效", fieldLabel(field)))
		}

		if !matched {
			return newFormValidationError(validationMessage(field, fmt.Sprintf("字段 %s 格式不正确", fieldLabel(field))))
		}
	}

	return nil
}

func validateNumberRule(field approval.FormFieldDefinition, value float64) error {
	if field.Validation == nil {
		return nil
	}

	if field.Validation.Min != nil && value < *field.Validation.Min {
		return newFormValidationError(fmt.Sprintf("字段 %s 不能小于 %v", fieldLabel(field), *field.Validation.Min))
	}

	if field.Validation.Max != nil && value > *field.Validation.Max {
		return newFormValidationError(fmt.Sprintf("字段 %s 不能大于 %v", fieldLabel(field), *field.Validation.Max))
	}

	return nil
}

func validateUploadField(field approval.FormFieldDefinition, value any) error {
	switch files := value.(type) {
	case string:
		if strings.TrimSpace(files) == "" {
			return newFormValidationError(fmt.Sprintf("字段 %s 不能为空", fieldLabel(field)))
		}

		return nil

	case []string:
		if len(files) == 0 {
			return newFormValidationError(fmt.Sprintf("字段 %s 不能为空", fieldLabel(field)))
		}

		return nil

	case []any:
		if len(files) == 0 {
			return newFormValidationError(fmt.Sprintf("字段 %s 不能为空", fieldLabel(field)))
		}

		for _, item := range files {
			text, ok := item.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return newFormValidationError(fmt.Sprintf("字段 %s 包含无效文件项", fieldLabel(field)))
			}
		}

		return nil

	default:
		return newFormValidationError(fmt.Sprintf("字段 %s 类型无效，应为文件字符串或文件数组", fieldLabel(field)))
	}
}

func validateSelectField(field approval.FormFieldDefinition, value any) error {
	allowedValues := make(map[string]struct{}, len(field.Options))
	for _, option := range field.Options {
		allowedValues[fmt.Sprint(option.Value)] = struct{}{}
	}

	validateValue := func(item any) error {
		if len(allowedValues) == 0 {
			return nil
		}

		if _, ok := allowedValues[fmt.Sprint(item)]; !ok {
			return newFormValidationError(fmt.Sprintf("字段 %s 取值无效", fieldLabel(field)))
		}

		return nil
	}

	switch items := value.(type) {
	case []any:
		for _, item := range items {
			if err := validateValue(item); err != nil {
				return err
			}
		}

		return nil

	case []string:
		for _, item := range items {
			if err := validateValue(item); err != nil {
				return err
			}
		}

		return nil

	default:
		return validateValue(value)
	}
}

func newFormValidationError(message string) error {
	return result.Err(message, result.WithCode(shared.ErrCodeFormValidationFailed))
}

func fieldLabel(field approval.FormFieldDefinition) string {
	if strings.TrimSpace(field.Label) != "" {
		return field.Label
	}

	return field.Key
}

func validationMessage(field approval.FormFieldDefinition, fallback string) string {
	if field.Validation != nil && strings.TrimSpace(field.Validation.Message) != "" {
		return field.Validation.Message
	}

	return fallback
}

func isEmptyFormValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []string:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	default:
		return false
	}
}

func toFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

// MergeFormData filters editable form data and merges it into the instance.
func MergeFormData(instance *approval.Instance, formData map[string]any, permissions map[string]approval.Permission) {
	if len(formData) == 0 {
		return
	}

	editableData := FilterEditableFormData(formData, permissions)
	if len(editableData) == 0 {
		return
	}

	if instance.FormData == nil {
		instance.FormData = make(map[string]any, len(editableData))
	}

	maps.Copy(instance.FormData, editableData)
}

// FilterEditableFormData filters form data to only include fields that are editable or required
// based on the node's field permissions configuration.
func FilterEditableFormData(formData map[string]any, permissions map[string]approval.Permission) map[string]any {
	if len(permissions) == 0 {
		return nil
	}

	filtered := make(map[string]any, len(formData))

	for k, v := range formData {
		perm, hasPerm := permissions[k]
		if !hasPerm {
			continue
		}

		if perm == approval.PermissionEditable || perm == approval.PermissionRequired {
			filtered[k] = v
		}
	}

	return filtered
}

// CheckInitiationPermission checks if the applicant is allowed to initiate the flow.
func (s *ValidationService) CheckInitiationPermission(ctx context.Context, db orm.DB, flowID, applicantID string, applicantDeptID *string) (bool, error) {
	var initiators []approval.FlowInitiator

	if err := db.NewSelect().
		Model(&initiators).
		Select("kind", "ids").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", flowID)
		}).
		Scan(ctx); err != nil {
		return false, fmt.Errorf("query flow initiators: %w", err)
	}

	if len(initiators) == 0 {
		return false, nil
	}

	for _, initiator := range initiators {
		switch initiator.Kind {
		case approval.InitiatorUser:
			if slices.Contains(initiator.IDs, applicantID) {
				return true, nil
			}

		case approval.InitiatorDept:
			if applicantDeptID == nil {
				continue
			}

			if slices.Contains(initiator.IDs, *applicantDeptID) {
				return true, nil
			}

		case approval.InitiatorRole:
			if s.assigneeService == nil {
				continue
			}

			for _, roleID := range initiator.IDs {
				users, err := s.assigneeService.GetRoleUsers(ctx, roleID)
				if err != nil {
					return false, fmt.Errorf("get users by role %s: %w", roleID, err)
				}

				if slices.ContainsFunc(users, func(u approval.UserInfo) bool { return u.ID == applicantID }) {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

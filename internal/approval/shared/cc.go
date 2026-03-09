package shared

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cast"

	collections "github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

var (
	errUnsupportedCCKind          = errors.New("unsupported cc kind")
	errUnsupportedCCFormFieldType = errors.New("unsupported cc form field type")
)

// CCUserResolver resolves CC user IDs from a single FlowNodeCC configuration.
type CCUserResolver func(cfg approval.FlowNodeCC, formData approval.FormData) ([]string, error)

// CCConfigSelector decides whether a FlowNodeCC config should be included.
type CCConfigSelector func(cfg approval.FlowNodeCC) bool

// ResolveCCUserIDs resolves CC recipients from static IDs or form-field values.
// CCRole and CCDept kinds require external user resolution and are not supported here.
func ResolveCCUserIDs(cfg approval.FlowNodeCC, formData approval.FormData) ([]string, error) {
	switch cfg.Kind {
	case approval.CCUser:
		return NormalizeUniqueIDs(cfg.IDs), nil
	case approval.CCRole, approval.CCDept:
		// Role and department CC kinds require external user resolution (e.g., via AssigneeService).
		// Skip silently here; callers that need role/dept resolution should inject an alternative resolver.
		return nil, nil
	case approval.CCFormField:
		// handled below
	default:
		return nil, fmt.Errorf("%w %q", errUnsupportedCCKind, cfg.Kind)
	}

	if cfg.FormField == nil || strings.TrimSpace(*cfg.FormField) == "" {
		return nil, nil
	}

	field := strings.TrimSpace(*cfg.FormField)

	value := formData.Get(field)
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string:
		userID := strings.TrimSpace(v)
		if userID == "" {
			return nil, nil
		}

		return []string{userID}, nil

	case []string:
		return NormalizeUniqueIDs(v), nil
	case []any:
		userIDs := make([]string, 0, len(v))
		for _, item := range v {
			if userID := strings.TrimSpace(cast.ToString(item)); userID != "" {
				userIDs = append(userIDs, userID)
			}
		}

		return NormalizeUniqueIDs(userIDs), nil

	default:
		return nil, fmt.Errorf("%w: %T", errUnsupportedCCFormFieldType, value)
	}
}

// CollectUniqueCCUserIDs resolves and deduplicates CC user IDs while preserving
// first-seen order.
func CollectUniqueCCUserIDs(
	configs []approval.FlowNodeCC,
	formData approval.FormData,
	resolver CCUserResolver,
	selector CCConfigSelector,
) ([]string, error) {
	ccUserIDs := NewOrderedUnique[string](len(configs))

	for _, cfg := range configs {
		if selector != nil && !selector(cfg) {
			continue
		}

		resolvedIDs, err := resolver(cfg, formData)
		if err != nil {
			return nil, err
		}

		ccUserIDs.AddAll(resolvedIDs...)
	}

	return ccUserIDs.ToSlice(), nil
}

// InsertCCRecords inserts CC records for the given users and returns only the newly
// inserted user IDs (existing records are ignored).
//
// Callers must hold an instance-level FOR UPDATE lock to prevent concurrent
// inserts from racing on the existence check.
func InsertCCRecords(
	ctx context.Context,
	db orm.DB,
	instanceID string,
	nodeID *string,
	userIDs []string,
	userNames map[string]string,
	isManual bool,
) ([]string, error) {
	normalizedUserIDs := NormalizeUniqueIDs(userIDs)
	if len(normalizedUserIDs) == 0 {
		return nil, nil
	}

	var existingUserIDs []string
	if err := db.NewSelect().
		Model((*approval.CCRecord)(nil)).
		Select("cc_user_id").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				In("cc_user_id", normalizedUserIDs).
				ApplyIf(nodeID == nil, func(cb orm.ConditionBuilder) {
					cb.IsNull("node_id")
				}).
				ApplyIf(nodeID != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("node_id", *nodeID)
				})
		}).
		Scan(ctx, &existingUserIDs); err != nil {
		return nil, fmt.Errorf("query existing cc records: %w", err)
	}

	existingSet := collections.NewHashSetFrom(existingUserIDs...)

	insertedUserIDs := make([]string, 0, len(normalizedUserIDs))
	for _, userID := range normalizedUserIDs {
		if existingSet.Contains(userID) {
			continue
		}

		insertedUserIDs = append(insertedUserIDs, userID)
	}

	if len(insertedUserIDs) == 0 {
		return nil, nil
	}

	records := make([]approval.CCRecord, len(insertedUserIDs))
	for i, userID := range insertedUserIDs {
		var ccUserName string
		if userNames != nil {
			ccUserName = userNames[userID]
		}

		records[i] = approval.CCRecord{
			InstanceID: instanceID,
			NodeID:     nodeID,
			CCUserID:   userID,
			CCUserName: ccUserName,
			IsManual:   isManual,
		}
	}

	if _, err := db.NewInsert().Model(&records).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert cc records: %w", err)
	}

	return insertedUserIDs, nil
}

// InsertAutoCCRecords inserts non-manual CC records and returns newly inserted IDs.
func InsertAutoCCRecords(ctx context.Context, db orm.DB, instanceID, nodeID string, userIDs []string, userNames map[string]string) ([]string, error) {
	return InsertCCRecords(ctx, db, instanceID, &nodeID, userIDs, userNames, false)
}

// InsertManualCCRecords inserts manual CC records and returns newly inserted IDs.
func InsertManualCCRecords(ctx context.Context, db orm.DB, instanceID, nodeID string, userIDs []string, userNames map[string]string) ([]string, error) {
	return InsertCCRecords(ctx, db, instanceID, &nodeID, userIDs, userNames, true)
}

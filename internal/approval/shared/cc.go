package shared

import (
	"context"
	"fmt"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// InsertCCRecords inserts CC records for the given users and returns only the
// newly inserted user IDs (existing records are ignored). Each record
// snapshots the recipient's display info — name and department — as resolved
// at send time.
//
// Callers must hold an instance-level FOR UPDATE lock to prevent concurrent
// inserts from racing on the existence check.
//
// nodeID and visitID are set together: a node-anchored record always belongs
// to one traversal, so dedup is visit-scoped — a rollback redo notifies (and
// waits) again. Instance-level records (both nil) dedup across the lifetime.
func InsertCCRecords(
	ctx context.Context,
	db orm.DB,
	instanceID string,
	nodeID *string,
	visitID *string,
	userIDs []string,
	userInfos map[string]approval.UserInfo,
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
					cb.Equals("node_id", *nodeID).
						Equals("visit_id", *visitID)
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
		info := userInfos[userID]
		records[i] = approval.CCRecord{
			InstanceID:           instanceID,
			NodeID:               nodeID,
			VisitID:              visitID,
			CCUserID:             userID,
			CCUserName:           info.Name,
			CCUserDepartmentID:   info.DepartmentID,
			CCUserDepartmentName: info.DepartmentName,
			IsManual:             isManual,
		}
	}

	if _, err := db.NewInsert().Model(&records).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert cc records: %w", err)
	}

	return insertedUserIDs, nil
}

// InsertAutoCCRecords inserts non-manual CC records and returns newly inserted IDs.
func InsertAutoCCRecords(ctx context.Context, db orm.DB, instanceID, nodeID, visitID string, userIDs []string, userInfos map[string]approval.UserInfo) ([]string, error) {
	return InsertCCRecords(ctx, db, instanceID, &nodeID, &visitID, userIDs, userInfos, false)
}

// InsertManualCCRecords inserts manual CC records and returns newly inserted IDs.
func InsertManualCCRecords(ctx context.Context, db orm.DB, instanceID, nodeID, visitID string, userIDs []string, userInfos map[string]approval.UserInfo) ([]string, error) {
	return InsertCCRecords(ctx, db, instanceID, &nodeID, &visitID, userIDs, userInfos, true)
}

// HasUnreadCCRecords reports whether the CC node still has any record awaiting a
// read confirmation. It is the single source of truth for read-confirm CC node
// completion: both node entry (engine.CCProcessor deciding wait vs. continue)
// and the mark-read path (NodeService.AdvanceCCNodeIfAllRead deciding whether to
// advance) consult it, so the two can never disagree about whether the node is
// done. A node that resolved to zero recipients has no records and is therefore
// already complete — it must not wait, or nothing could ever advance it.
func HasUnreadCCRecords(ctx context.Context, db orm.DB, instanceID, nodeID, visitID string) (bool, error) {
	unread, err := db.NewSelect().
		Model((*approval.CCRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("node_id", nodeID).
				Equals("visit_id", visitID).
				IsNull("read_at")
		}).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check unread cc records: %w", err)
	}

	return unread, nil
}

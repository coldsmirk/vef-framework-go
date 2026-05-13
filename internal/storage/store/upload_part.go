package store

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// NewUploadPartStore returns the default UploadPartStore implementation
// backed by the orm.DB abstraction. The concrete SQL dialect is
// determined by the underlying orm provider; this package depends only
// on orm.DB.
func NewUploadPartStore(db orm.DB) UploadPartStore {
	return &uploadPartStore{db: db}
}

type uploadPartStore struct {
	db orm.DB
}

func (*uploadPartStore) Upsert(ctx context.Context, tx orm.DB, part *UploadPart) error {
	_, err := tx.NewInsert().Model(part).
		OnConflict(func(cb orm.ConflictBuilder) {
			cb.Columns("claim_id", "part_number").DoUpdate().
				Set("etag").
				Set("size")
		}).
		Exec(ctx)

	return err
}

func (s *uploadPartStore) ListByClaim(ctx context.Context, claimID string) ([]UploadPart, error) {
	var parts []UploadPart

	err := s.db.NewSelect().Model(&parts).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("claim_id", claimID)
	}).OrderBy("part_number").Scan(ctx)
	if err != nil {
		return nil, err
	}

	return parts, nil
}

func (*uploadPartStore) DeleteByClaim(ctx context.Context, tx orm.DB, claimID string) error {
	_, err := tx.NewDelete().Model((*UploadPart)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("claim_id", claimID)
	}).Exec(ctx)

	return err
}

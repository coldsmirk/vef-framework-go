package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// NewClaimStore returns the default ClaimStore implementation backed by
// the orm.DB abstraction. The concrete SQL dialect is determined by the
// underlying orm provider; this package depends only on orm.DB.
//
// The returned value also satisfies the public storage.ClaimConsumer
// interface; the fx graph exposes both surfaces.
func NewClaimStore(db orm.DB) ClaimStore {
	return &claimStore{db: db}
}

type claimStore struct {
	db orm.DB
}

func (s *claimStore) Create(ctx context.Context, claim *UploadClaim) error {
	if claim == nil {
		return errors.New("storage: nil claim")
	}

	_, err := s.db.NewInsert().Model(claim).Exec(ctx)

	return err
}

func (s *claimStore) UpdateUploadID(ctx context.Context, id, uploadID string) error {
	res, err := s.db.NewUpdate().Model((*UploadClaim)(nil)).
		Set("upload_id", uploadID).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", id)
		}).
		Exec(ctx)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if n == 0 {
		return fmt.Errorf("%w: %s", storage.ErrClaimNotFound, id)
	}

	return nil
}

func (*claimStore) MarkUploaded(ctx context.Context, tx orm.DB, id string) error {
	res, err := tx.NewUpdate().Model((*UploadClaim)(nil)).
		Set("status", string(ClaimStatusUploaded)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", id)
			cb.Equals("status", string(ClaimStatusPending))
		}).
		Exec(ctx)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if n == 0 {
		return fmt.Errorf("%w: %s", storage.ErrClaimNotFound, id)
	}

	return nil
}

func (s *claimStore) Get(ctx context.Context, id string) (*UploadClaim, error) {
	var claim UploadClaim

	err := s.db.NewSelect().Model(&claim).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("id", id)
	}).Scan(ctx)
	if err != nil {
		if result.IsRecordNotFound(err) {
			return nil, storage.ErrClaimNotFound
		}

		return nil, err
	}

	return &claim, nil
}

func (s *claimStore) CountPendingByOwner(ctx context.Context, owner string) (int, error) {
	count, err := s.db.NewSelect().Model((*UploadClaim)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("created_by", owner)
		cb.Equals("status", string(ClaimStatusPending))
	}).Count(ctx)

	return int(count), err
}

func (s *claimStore) GetByKey(ctx context.Context, key string) (*UploadClaim, error) {
	var claim UploadClaim

	err := s.db.NewSelect().Model(&claim).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("object_key", key)
	}).Scan(ctx)
	if err != nil {
		if result.IsRecordNotFound(err) {
			return nil, storage.ErrClaimNotFound
		}

		return nil, err
	}

	return &claim, nil
}

func (*claimStore) Consume(ctx context.Context, tx orm.DB, key string) error {
	res, err := tx.NewDelete().Model((*UploadClaim)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("object_key", key)
		cb.Equals("status", string(ClaimStatusUploaded))
	}).Exec(ctx)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if n == 0 {
		return fmt.Errorf("%w: %s", storage.ErrClaimNotFound, key)
	}

	return nil
}

func (*claimStore) ConsumeMany(ctx context.Context, tx orm.DB, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	uniq := dedupeStrings(keys)

	res, err := tx.NewDelete().Model((*UploadClaim)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.In("object_key", uniq)
		cb.Equals("status", string(ClaimStatusUploaded))
	}).Exec(ctx)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if n != int64(len(uniq)) {
		return fmt.Errorf("%w: matched %d of %d keys", storage.ErrClaimNotFound, n, len(uniq))
	}

	return nil
}

func (s *claimStore) ScanExpired(ctx context.Context, now timex.DateTime, limit int) ([]UploadClaim, error) {
	var claims []UploadClaim

	err := s.db.NewSelect().Model(&claims).Where(func(cb orm.ConditionBuilder) {
		cb.LessThan("expires_at", now)
	}).OrderBy("expires_at").Limit(limit).Scan(ctx)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

func (s *claimStore) DeleteByID(ctx context.Context, id string) error {
	_, err := s.db.NewDelete().Model((*UploadClaim)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("id", id)
	}).Exec(ctx)

	return err
}

func (*claimStore) DeleteByIDInTx(ctx context.Context, tx orm.DB, id string) error {
	_, err := tx.NewDelete().Model((*UploadClaim)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("id", id)
	}).Exec(ctx)

	return err
}

func (*claimStore) DeleteByIDs(ctx context.Context, tx orm.DB, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	_, err := tx.NewDelete().Model((*UploadClaim)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.In("id", ids)
	}).Exec(ctx)

	return err
}

func dedupeStrings(in []string) []string {
	if len(in) <= 1 {
		return in
	}

	seen := collections.NewHashSet[string]()
	out := make([]string, 0, len(in))

	for _, v := range in {
		if seen.Contains(v) {
			continue
		}

		seen.Add(v)
		out = append(out, v)
	}

	return out
}

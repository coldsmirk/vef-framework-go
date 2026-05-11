package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// NewClaimStore returns the default bun-backed ClaimStore implementation.
func NewClaimStore(db orm.DB) storage.ClaimStore {
	return &bunClaimStore{db: db}
}

type bunClaimStore struct {
	db orm.DB
}

func (s *bunClaimStore) Create(ctx context.Context, claim *storage.UploadClaim) error {
	if claim == nil {
		return errors.New("storage: nil claim")
	}

	_, err := s.db.NewInsert().Model(claim).Exec(ctx)

	return err
}

func (s *bunClaimStore) Get(ctx context.Context, id string) (*storage.UploadClaim, error) {
	var claim storage.UploadClaim

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

func (s *bunClaimStore) GetByKey(ctx context.Context, key string) (*storage.UploadClaim, error) {
	var claim storage.UploadClaim

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

func (s *bunClaimStore) Consume(ctx context.Context, tx orm.DB, key string) error {
	res, err := tx.NewDelete().Model((*storage.UploadClaim)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("object_key", key)
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

func (s *bunClaimStore) ConsumeMany(ctx context.Context, tx orm.DB, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	uniq := dedupeStrings(keys)

	res, err := tx.NewDelete().Model((*storage.UploadClaim)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.In("object_key", uniq)
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

func (s *bunClaimStore) ScanExpired(ctx context.Context, now timex.DateTime, limit int) ([]storage.UploadClaim, error) {
	var claims []storage.UploadClaim

	err := s.db.NewSelect().Model(&claims).Where(func(cb orm.ConditionBuilder) {
		cb.LessThan("expires_at", now)
	}).OrderBy("expires_at").Limit(limit).Scan(ctx)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

func (s *bunClaimStore) DeleteByID(ctx context.Context, id string) error {
	_, err := s.db.NewDelete().Model((*storage.UploadClaim)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("id", id)
	}).Exec(ctx)

	return err
}

func dedupeStrings(in []string) []string {
	if len(in) <= 1 {
		return in
	}

	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))

	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}

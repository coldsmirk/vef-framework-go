package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

var errNilClaim = errors.New("storage: nil claim")

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
		return errNilClaim
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
		Set("status", ClaimStatusUploaded).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", id)
			cb.Equals("status", ClaimStatusPending)
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

func (*claimStore) MarkUploadedIfPendingExpired(
	ctx context.Context,
	tx orm.DB,
	claim UploadClaim,
	cutoff timex.DateTime,
) (bool, error) {
	res, err := tx.NewUpdate().Model((*UploadClaim)(nil)).
		Set("status", ClaimStatusUploaded).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", claim.ID)
			cb.Equals("object_key", claim.Key)
			cb.Equals("upload_id", claim.UploadID)
			cb.Equals("status", ClaimStatusPending)
			cb.LessThan("expires_at", cutoff)
		}).
		Exec(ctx)
	if err != nil {
		return false, err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return n > 0, nil
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
		cb.Equals("status", ClaimStatusPending)
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
		cb.Equals("status", ClaimStatusUploaded)
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

// ConsumeMany deletes upload_claim rows whose object_key is in keys
// AND whose created_by matches principal.ID. Folding the ownership
// check into the DELETE WHERE makes the operation secure-by-default
// at zero extra-query cost: a row that exists but belongs to another
// principal is invisible to this caller, identical to a row that does
// not exist. The single sentinel (ErrClaimNotFound) intentionally does
// not distinguish the two — that would leak existence across tenants.
func (*claimStore) ConsumeMany(ctx context.Context, tx orm.DB, principal *security.Principal, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	// Reject anonymous or malformed principals up front. ErrAccessDenied
	// (not ErrClaimNotFound) is the right class here: a downstream
	// debugger seeing "claim not found" would chase missing-data leads,
	// while the real problem is "no authenticated subject in context"
	// — typically a background job / batch path calling Files without
	// supplying a system principal.
	if principal == nil || principal.ID == "" {
		return fmt.Errorf("%w: anonymous principal cannot consume claims", storage.ErrAccessDenied)
	}

	uniq := dedupeStrings(keys)

	res, err := tx.NewDelete().Model((*UploadClaim)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.In("object_key", uniq)
		cb.Equals("status", ClaimStatusUploaded)
		cb.Equals("created_by", principal.ID)
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

	// Only pending claims are eligible for sweeping. An 'uploaded' claim
	// whose business consumption is delayed past ExpiresAt would otherwise
	// be reaped, deleting the backend object while the business layer
	// still considers it live.
	err := s.db.NewSelect().Model(&claims).Where(func(cb orm.ConditionBuilder) {
		cb.LessThan("expires_at", now)
		cb.Equals("status", ClaimStatusPending)
	}).OrderBy("expires_at").Limit(limit).Scan(ctx)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

func (*claimStore) LockExpiredInTx(ctx context.Context, tx orm.DB, now timex.DateTime, limit int) ([]UploadClaim, error) {
	var claims []UploadClaim

	// FOR UPDATE SKIP LOCKED is essential for the multi-instance sweeper
	// story (same reasoning as DeleteQueue.Lease): without it, two
	// sweepers running on the same tick both pick the same rows and the
	// downstream Enqueue + DeleteByIDs serialize on the locked row set.
	// SKIP LOCKED makes each sweeper take a disjoint slice. SQLite, which
	// has no row-level locking, transparently drops the FOR UPDATE clause
	// (single-writer DB → no race to begin with).
	err := tx.NewSelect().Model(&claims).Where(func(cb orm.ConditionBuilder) {
		cb.LessThan("expires_at", now)
		cb.Equals("status", ClaimStatusPending)
	}).OrderBy("expires_at").Limit(limit).ForUpdateSkipLocked().Scan(ctx)
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

func (*claimStore) DeleteIfPendingExpired(
	ctx context.Context,
	tx orm.DB,
	claim UploadClaim,
	cutoff timex.DateTime,
) (bool, error) {
	res, err := tx.NewDelete().Model((*UploadClaim)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("id", claim.ID)
		cb.Equals("object_key", claim.Key)
		cb.Equals("upload_id", claim.UploadID)
		cb.Equals("status", ClaimStatusPending)
		cb.LessThan("expires_at", cutoff)
	}).Exec(ctx)
	if err != nil {
		return false, err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return n > 0, nil
}

func dedupeStrings(in []string) []string {
	if len(in) <= 1 {
		return in
	}

	return collections.NewHashSetFrom(in...).ToSlice()
}

package store

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// NewFileStore returns the default FileStore implementation backed by
// the orm.DB abstraction.
func NewFileStore(db orm.DB) FileStore {
	return &fileStore{db: db}
}

type fileStore struct {
	db orm.DB
}

func (*fileStore) Record(ctx context.Context, tx orm.DB, claim UploadClaim) error {
	// CreatedBy is copied from the claim rather than left to the audit
	// handler: the sweeper recovery path runs without a principal, and
	// the handler would stamp the system operator over the real uploader.
	record := &storage.FileRecord{
		ID:               claim.ID,
		CreatedAt:        timex.Now(),
		CreatedBy:        claim.CreatedBy,
		Key:              claim.Key,
		OriginalFilename: claim.OriginalFilename,
		ContentType:      claim.ContentType,
		Size:             claim.Size,
		Public:           claim.Public,
		Status:           storage.FileStatusUploaded,
		StartedAt:        claim.CreatedAt,
	}

	_, err := tx.NewInsert().Model(record).Exec(ctx)

	return err
}

func (*fileStore) MarkClaimed(ctx context.Context, tx orm.DB, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	uniq := dedupeStrings(keys)

	res, err := tx.NewUpdate().Model((*storage.FileRecord)(nil)).
		Set("status", storage.FileStatusClaimed).
		Set("claimed_at", timex.Now()).
		Where(func(cb orm.ConditionBuilder) {
			cb.In("object_key", uniq)
			cb.Equals("status", storage.FileStatusUploaded)
		}).
		Exec(ctx)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	// A shortfall is not an error — see the interface contract — but it
	// is worth surfacing: every adopted key uploaded after the registry
	// shipped should have a row waiting for it.
	if n != int64(len(uniq)) {
		logger.Warnf("Registry marked %d of %d adopted file(s) as claimed; the rest have no record", n, len(uniq))
	}

	return nil
}

func (*fileStore) MarkDeleted(ctx context.Context, tx orm.DB, key string, reason storage.DeleteReason) error {
	_, err := tx.NewUpdate().Model((*storage.FileRecord)(nil)).
		Set("status", storage.FileStatusDeleted).
		Set("deleted_at", timex.Now()).
		Set("delete_reason", reason).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("object_key", key)
			cb.NotEquals("status", storage.FileStatusDeleted)
		}).
		Exec(ctx)

	return err
}

func (s *fileStore) Lookup(ctx context.Context, keys []string) (map[string]storage.FileRecord, error) {
	found := make(map[string]storage.FileRecord, len(keys))
	if len(keys) == 0 {
		return found, nil
	}

	var records []storage.FileRecord

	err := s.db.NewSelect().Model(&records).Where(func(cb orm.ConditionBuilder) {
		cb.In("object_key", dedupeStrings(keys))
	}).Scan(ctx)
	if err != nil {
		return nil, err
	}

	for i := range records {
		found[records[i].Key] = records[i]
	}

	return found, nil
}

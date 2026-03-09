package orm

import "github.com/uptrace/bun"

// BunTx wraps a bun.Tx to implement the Tx interface for manual transaction control.
type BunTx struct {
	BunDB
}

func (t *BunTx) Commit() error {
	return t.db.(bun.Tx).Commit()
}

func (t *BunTx) Rollback() error {
	return t.db.(bun.Tx).Rollback()
}

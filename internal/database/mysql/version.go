package mysql

import (
	"context"

	"github.com/uptrace/bun"
)

func queryVersion(db *bun.DB) (string, error) {
	var version string

	return version, db.NewSelect().
		ColumnExpr("version()").
		Scan(context.Background(), &version)
}

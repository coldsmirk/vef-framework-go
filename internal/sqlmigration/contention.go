package sqlmigration

import (
	"errors"
	"strings"
)

// SQLite result codes for write-lock contention. A driver may report an
// extended code (SQLITE_BUSY_SNAPSHOT and friends carry the primary code in
// the low byte), so classification masks the primary code back out.
const (
	sqliteBusyCode        = 5
	sqliteLockedCode      = 6
	sqlitePrimaryCodeMask = 0xff
)

// resultCoder is the shape a SQLite driver error exposes its result code
// through — modernc.org/sqlite's *Error, the driver sqliteshim selects for
// CGO_ENABLED=0 builds, implements exactly this. Matching on the shape rather
// than on the concrete type keeps migration infrastructure independent of
// which driver variant sqliteshim compiled in.
type resultCoder interface {
	Code() int
}

// IsBusyContention reports whether err is SQLite write-lock contention:
// another connection held the database's single writer, so the statement was
// refused rather than failing on its own merits. Callers treat it as a benign
// "try again" — the migration lock retries acquisition, the cron store retries
// a claim on the next tick and re-attempts an outcome write.
//
// The driver's own result code is authoritative and is trusted alone whenever
// it is available: an unrelated failure must never reclassify as contention
// because a statement argument echoed into its message happened to read
// "is locked". Only a driver that surfaces no result code (the cgo
// github.com/mattn/go-sqlite3 build) falls back to matching the message.
func IsBusyContention(err error) bool {
	if err == nil {
		return false
	}

	var coder resultCoder
	if errors.As(err, &coder) {
		code := coder.Code() & sqlitePrimaryCodeMask

		return code == sqliteBusyCode || code == sqliteLockedCode
	}

	message := err.Error()

	return strings.Contains(message, "is locked") ||
		strings.Contains(message, "SQLITE_BUSY") ||
		strings.Contains(message, "SQLITE_LOCKED")
}

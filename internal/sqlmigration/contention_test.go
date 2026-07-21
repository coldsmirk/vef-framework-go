package sqlmigration

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// CodedError mimics the driver error shape modernc.org/sqlite returns: a
// message plus the SQLite result code behind it.
type CodedError struct {
	message string
	code    int
}

func (e *CodedError) Error() string { return e.message }

func (e *CodedError) Code() int { return e.code }

func TestIsBusyContention(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"Nil", nil, false},
		{
			"TypedBusy",
			&CodedError{message: "database is locked (5) (SQLITE_BUSY)", code: sqliteBusyCode},
			true,
		},
		{
			"TypedLocked",
			&CodedError{message: "database table is locked (6)", code: sqliteLockedCode},
			true,
		},
		{
			"TypedExtendedBusy",
			// SQLITE_BUSY_SNAPSHOT keeps SQLITE_BUSY in its low byte.
			&CodedError{message: "database is locked (517)", code: sqliteBusyCode | (2 << 8)},
			true,
		},
		{
			"TypedUnrelatedCodeWithContentionWording",
			// The false positive the result code exists to prevent: a failure
			// on the statement's own merits whose message echoes an argument
			// reading like contention.
			&CodedError{message: `constraint failed: error = "the account is locked" (19)`, code: 19},
			false,
		},
		{
			"WrappedTypedBusy",
			fmt.Errorf("advance schedule %q: %w", "orders.sync",
				&CodedError{message: "database is locked (5)", code: sqliteBusyCode}),
			true,
		},
		{"UntypedIsLocked", errors.New("database is locked"), true},
		{"UntypedBusyName", errors.New("failure (SQLITE_BUSY)"), true},
		{"UntypedLockedName", errors.New("failure (SQLITE_LOCKED)"), true},
		{"UntypedWrappedIsLocked", fmt.Errorf("commit: %w", errors.New("database is locked")), true},
		{"UntypedUnrelated", errors.New("no such table: crn_schedule"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsBusyContention(tc.err),
				"Contention classification must match the driver's own verdict")
		})
	}
}

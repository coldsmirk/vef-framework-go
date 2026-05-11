// Package models contains test fixtures consumed by generator integration tests.
// This directory lives under testdata/ so the Go toolchain skips it during
// regular builds; packages.Load still resolves it when invoked with file= mode.
package models

import (
	"github.com/coldsmirk/vef-framework-go/orm"
)

// User exercises every relevant tag combination supported by the generator.
type User struct {
	orm.BaseModel `bun:"table:users,alias:u"`

	ID       string `json:"id"       bun:"id,pk"`
	Name     string `json:"name"     bun:"name,notnull" label:"User Name"`
	Email    string `json:"email"    bun:"email"`
	NoTag    string
	Internal string `bun:"-"`
	Computed string `bun:",scanonly"`

	// Relationship fields must be skipped entirely.
	Profile *Profile  `json:"profile" bun:"rel:has-one,join:id=user_id"`
	Posts   []Profile `json:"posts"   bun:"rel:has-many,join:id=user_id"`

	// Address is embedded with a column name prefix.
	Address Address `bun:"embed:addr_"`

	// Lower-case unexported fields are skipped.
	internalNote string //nolint:unused

	// Embed of an embedded struct without prefix accumulates inner columns directly.
	AuditInfo

	// Reserved Go keyword as field name should be escaped in goName.
	Type string `bun:"type"`

	// Reserved schema method name should be prefixed with Col in MethodName.
	Table string `bun:"table_value"`
}

// Profile has its own BaseModel; nested rel types should not affect User parsing.
type Profile struct {
	orm.BaseModel `bun:"table:profiles"`

	UserID string `bun:"user_id,pk"`
	Bio    string `bun:"bio"`
}

// Address is a non-base-model struct used only as an embed.
type Address struct {
	City   string `bun:"city" label:"City"`
	Street string `bun:"street"`
}

// AuditInfo is anonymously embedded and contributes columns directly.
type AuditInfo struct {
	CreatedBy     string `bun:"created_by,notnull"`
	CreatedByName string `bun:",scanonly"`
}

// NotAModel does not embed orm.BaseModel and must not appear in the output.
type NotAModel struct {
	Name string
}

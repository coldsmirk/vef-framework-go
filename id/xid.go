package id

import "github.com/rs/xid"

// DefaultXIDGenerator is the default XID generator instance.
var DefaultXIDGenerator = NewXIDGenerator()

type xidGenerator struct{}

// Generate creates a new XID as a 20-character base32-encoded string.
func (*xidGenerator) Generate() string {
	return xid.New().String()
}

// NewXIDGenerator creates a new XID generator instance.
func NewXIDGenerator() IDGenerator {
	return &xidGenerator{}
}

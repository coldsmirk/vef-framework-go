package mold

import "github.com/coldsmirk/go-collections"

const (
	diveTag            = "dive"
	restrictedTagChars = ".[],|=+()`~!@#$%^&*\\\"/?<>{}"
	tagSeparator       = ","
	ignoreTag          = "-"
	tagKeySeparator    = "="
	utf8HexComma       = "0x2C"
	keysTag            = "keys"
	endKeysTag         = "endkeys"
)

var restrictedTags = collections.NewHashSetFrom(diveTag, ignoreTag)

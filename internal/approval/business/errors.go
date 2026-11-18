package business

import "errors"

var (
	ErrInvalidDescriptor = errors.New("invalid approval business kind descriptor")
	ErrDuplicateKind     = errors.New("duplicate approval business kind")
	ErrEmptyReference    = errors.New("approval business handler returned an empty reference")
	ErrChangedReference  = errors.New("approval business handler changed the instance's reference on resubmission")
)

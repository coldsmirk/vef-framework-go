package i18n

import "errors"

var (
	// ErrUnsupportedLanguage is returned when an unsupported language code is provided.
	ErrUnsupportedLanguage = errors.New("unsupported language code")
	// ErrMessageIDEmpty is returned when a translation message ID is empty.
	ErrMessageIDEmpty = errors.New("messageID cannot be empty")
)

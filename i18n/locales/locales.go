package locales

import "embed"

// EmbedLocales contains all localization files embedded into the binary.
// This allows the application to be deployed as a single binary without
// requiring external language files.
//
// The embed directive includes all JSON files in the current directory,
// which should contain the message translations for each supported language.
//
//go:embed *.json
var EmbedLocales embed.FS

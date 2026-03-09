package sequence

import (
	"strings"

	"github.com/coldsmirk/vef-framework-go/timex"
)

// FormatDate formats a datetime according to a user-friendly format string.
// Supported placeholders: yyyy, yy, MM, dd, HH, mm, ss.
// Returns an empty string if the format is empty.
func FormatDate(dt timex.DateTime, format string) string {
	if format == "" {
		return ""
	}

	return dt.Format(toGoLayout(format))
}

// dateLayoutReplacer converts user-friendly date placeholders to Go time layout tokens.
// Longer patterns are listed first to avoid partial matches (e.g. yyyy before yy).
var dateLayoutReplacer = strings.NewReplacer(
	"yyyy", "2006",
	"yy", "06",
	"MM", "01",
	"dd", "02",
	"HH", "15",
	"mm", "04",
	"ss", "05",
)

// toGoLayout converts a user-friendly date format to a Go time layout.
// Supported: yyyy→2006, yy→06, MM→01, dd→02, HH→15, mm→04, ss→05.
// Characters not matching any placeholder are preserved as-is.
func toGoLayout(format string) string {
	return dateLayoutReplacer.Replace(format)
}

package sequence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/timex"
)

func TestToGoLayout(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"yyyyMMdd", "yyyyMMdd", "20060102"},
		{"yyyyMM", "yyyyMM", "200601"},
		{"yyMMdd", "yyMMdd", "060102"},
		{"YearMonthDayWithDash", "yyyy-MM-dd", "2006-01-02"},
		{"FullDateTime", "yyyyMMddHHmmss", "20060102150405"},
		{"EmptyFormat", "", ""},
		{"NoPlaceholders", "---", "---"},
		{"MixedLiteralsAndPlaceholders", "yyyy/MM/dd", "2006/01/02"},
		{"YearMonthDayHour", "yyyyMMddHH", "2006010215"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, toGoLayout(tt.input), "Go layout should match expected for format %q", tt.input)
		})
	}
}

func TestFormatDate(t *testing.T) {
	dt := timex.DateTime(time.Date(2024, 3, 15, 9, 30, 45, 0, time.Local))

	tests := []struct {
		name     string
		format   string
		expected string
	}{
		{"yyyyMMdd", "yyyyMMdd", "20240315"},
		{"YearMonthDayWithDash", "yyyy-MM-dd", "2024-03-15"},
		{"yyMMdd", "yyMMdd", "240315"},
		{"yyyyMM", "yyyyMM", "202403"},
		{"FullDateTime", "yyyyMMddHHmmss", "20240315093045"},
		{"EmptyFormat", "", ""},
		{"OnlyLiterals", "---", "---"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, FormatDate(dt, tt.format), "Formatted date should match expected for format %q", tt.format)
		})
	}
}

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractHTMLURLs tests HTML URL extraction from various HTML content.
func TestExtractHTMLURLs(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected []string
	}{
		{
			name:     "EmptyString",
			html:     "",
			expected: nil,
		},
		{
			name:     "NoUrls",
			html:     `<div>No URLs here</div>`,
			expected: nil,
		},
		{
			name:     "DoubleQuotes",
			html:     `<img src="temp/pic.jpg">`,
			expected: []string{"temp/pic.jpg"},
		},
		{
			name:     "SingleQuotes",
			html:     `<img src='temp/pic.jpg'>`,
			expected: []string{"temp/pic.jpg"},
		},
		{
			name:     "MismatchedQuotesShouldNotMatch",
			html:     `<img src="temp/pic.jpg'> <a href='temp/doc.pdf">`,
			expected: nil,
		},
		{
			name:     "MixedValidQuotes",
			html:     `<img src="temp/pic1.jpg"> <a href='temp/doc.pdf'>`,
			expected: []string{"temp/pic1.jpg", "temp/doc.pdf"},
		},
		{
			name:     "MultipleUrlsWithDeduplication",
			html:     `<img src="temp/pic.jpg"> <a href="temp/pic.jpg">`,
			expected: []string{"temp/pic.jpg"},
		},
		{
			name:     "DifferentTagTypes",
			html:     `<img src="temp/img.jpg"> <video src='temp/video.mp4'> <audio src="temp/audio.mp3"> <source src='temp/src.mp4'> <embed src="temp/embed.swf"> <object data='temp/data.pdf'>`,
			expected: []string{"temp/img.jpg", "temp/video.mp4", "temp/audio.mp3", "temp/src.mp4", "temp/embed.swf", "temp/data.pdf"},
		},
		{
			name:     "AbsoluteUrlsIgnored",
			html:     `<img src="http://example.com/pic.jpg"> <a href="https://example.com/doc.pdf"> <img src="temp/local.jpg">`,
			expected: []string{"temp/local.jpg"},
		},
		{
			name:     "CaseInsensitiveTags",
			html:     `<IMG SRC="temp/pic.jpg"> <A HREF='temp/doc.pdf'>`,
			expected: []string{"temp/pic.jpg", "temp/doc.pdf"},
		},
		{
			name:     "UrlWithSpaces",
			html:     `<img src="temp/pic with spaces.jpg">`,
			expected: []string{"temp/pic with spaces.jpg"},
		},
		{
			name:     "UrlWithSpecialCharacters",
			html:     `<img src="temp/pic-file_name@2x.jpg">`,
			expected: []string{"temp/pic-file_name@2x.jpg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls := extractHtmlURLs(tt.html)
			if len(tt.expected) == 0 {
				assert.Empty(t, urls, "Should be empty")
			} else {
				assert.ElementsMatch(t, tt.expected, urls, "ElementsMatch assertion should pass")
			}
		})
	}
}

// TestReplaceHTMLURLs tests HTML URL replacement in various HTML content.
func TestReplaceHTMLURLs(t *testing.T) {
	tests := []struct {
		name         string
		html         string
		replacements map[string]string
		expected     string
	}{
		{
			name:         "EmptyString",
			html:         "",
			replacements: map[string]string{"temp/pic.jpg": "uploads/pic.jpg"},
			expected:     "",
		},
		{
			name:         "EmptyReplacements",
			html:         `<img src="temp/pic.jpg">`,
			replacements: map[string]string{},
			expected:     `<img src="temp/pic.jpg">`,
		},
		{
			name:         "DoubleQuotesReplacement",
			html:         `<img src="temp/pic.jpg">`,
			replacements: map[string]string{"temp/pic.jpg": "uploads/pic.jpg"},
			expected:     `<img src="uploads/pic.jpg">`,
		},
		{
			name:         "SingleQuotesPreserved",
			html:         `<img src='temp/pic.jpg'>`,
			replacements: map[string]string{"temp/pic.jpg": "uploads/pic.jpg"},
			expected:     `<img src='uploads/pic.jpg'>`,
		},
		{
			name: "MultipleReplacements",
			html: `<img src="temp/pic.jpg"> <a href="temp/doc.pdf">`,
			replacements: map[string]string{
				"temp/pic.jpg": "uploads/pic.jpg",
				"temp/doc.pdf": "uploads/doc.pdf",
			},
			expected: `<img src="uploads/pic.jpg"> <a href="uploads/doc.pdf">`,
		},
		{
			name: "MixedQuotesPreserved",
			html: `<img src="temp/pic.jpg"> <a href='temp/doc.pdf'>`,
			replacements: map[string]string{
				"temp/pic.jpg": "uploads/pic.jpg",
				"temp/doc.pdf": "uploads/doc.pdf",
			},
			expected: `<img src="uploads/pic.jpg"> <a href='uploads/doc.pdf'>`,
		},
		{
			name:         "NoMatchingUrls",
			html:         `<img src="other/pic.jpg">`,
			replacements: map[string]string{"temp/pic.jpg": "uploads/pic.jpg"},
			expected:     `<img src="other/pic.jpg">`,
		},
		{
			name: "PartialReplacement",
			html: `<img src="temp/pic1.jpg"> <a href="temp/doc.pdf"> <img src="keep/pic2.jpg">`,
			replacements: map[string]string{
				"temp/pic1.jpg": "uploads/pic1.jpg",
				"temp/doc.pdf":  "uploads/doc.pdf",
			},
			expected: `<img src="uploads/pic1.jpg"> <a href="uploads/doc.pdf"> <img src="keep/pic2.jpg">`,
		},
		{
			name:         "CaseInsensitiveAttributes",
			html:         `<IMG SRC="temp/pic.jpg">`,
			replacements: map[string]string{"temp/pic.jpg": "uploads/pic.jpg"},
			expected:     `<IMG SRC="uploads/pic.jpg">`,
		},
		{
			name:         "DifferentAttributes",
			html:         `<img src="temp/img.jpg"> <object data='temp/data.pdf'>`,
			replacements: map[string]string{"temp/img.jpg": "uploads/img.jpg", "temp/data.pdf": "uploads/data.pdf"},
			expected:     `<img src="uploads/img.jpg"> <object data='uploads/data.pdf'>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceHtmlURLs(tt.html, tt.replacements)
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestExtractMarkdownURLs tests Markdown URL extraction from various Markdown content.
func TestExtractMarkdownURLs(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		expected []string
	}{
		{
			name:     "EmptyString",
			markdown: "",
			expected: nil,
		},
		{
			name:     "NoUrls",
			markdown: "Plain text without URLs",
			expected: nil,
		},
		{
			name:     "ImageSyntax",
			markdown: `![Alt text](temp/pic.jpg)`,
			expected: []string{"temp/pic.jpg"},
		},
		{
			name:     "LinkSyntax",
			markdown: `[Link text](temp/doc.pdf)`,
			expected: []string{"temp/doc.pdf"},
		},
		{
			name:     "MixedImageAndLink",
			markdown: `![Image](temp/pic.jpg) [Link](temp/doc.pdf)`,
			expected: []string{"temp/pic.jpg", "temp/doc.pdf"},
		},
		{
			name:     "WithDoubleQuoteTitle",
			markdown: `![Image](temp/pic.jpg "Title")`,
			expected: []string{"temp/pic.jpg"},
		},
		{
			name:     "WithSingleQuoteTitle",
			markdown: `[Link](temp/doc.pdf 'Title')`,
			expected: []string{"temp/doc.pdf"},
		},
		{
			name:     "AbsoluteUrlsIgnored",
			markdown: `![Remote](https://example.com/pic.jpg) ![Local](temp/pic.jpg)`,
			expected: []string{"temp/pic.jpg"},
		},
		{
			name:     "EmptyAltText",
			markdown: `![](temp/pic.jpg) [](temp/doc.pdf)`,
			expected: []string{"temp/pic.jpg", "temp/doc.pdf"},
		},
		{
			name:     "MultipleUrlsWithDeduplication",
			markdown: `![Image](temp/pic.jpg) [Link](temp/pic.jpg)`,
			expected: []string{"temp/pic.jpg"},
		},
		{
			name:     "UrlWithSpaces",
			markdown: `![Image](temp/pic with spaces.jpg)`,
			expected: []string{"temp/pic with spaces.jpg"},
		},
		{
			name:     "UrlWithSpecialCharacters",
			markdown: `[Link](temp/doc-file_name@v2.pdf)`,
			expected: []string{"temp/doc-file_name@v2.pdf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls := extractMarkdownURLs(tt.markdown)
			if len(tt.expected) == 0 {
				assert.Empty(t, urls, "Should be empty")
			} else {
				assert.ElementsMatch(t, tt.expected, urls, "ElementsMatch assertion should pass")
			}
		})
	}
}

// TestReplaceMarkdownURLs tests Markdown URL replacement in various Markdown content.
func TestReplaceMarkdownURLs(t *testing.T) {
	tests := []struct {
		name         string
		markdown     string
		replacements map[string]string
		expected     string
	}{
		{
			name:         "EmptyString",
			markdown:     "",
			replacements: map[string]string{"temp/pic.jpg": "uploads/pic.jpg"},
			expected:     "",
		},
		{
			name:         "EmptyReplacements",
			markdown:     `![Image](temp/pic.jpg)`,
			replacements: map[string]string{},
			expected:     `![Image](temp/pic.jpg)`,
		},
		{
			name:         "ImageReplacement",
			markdown:     `![Alt](temp/pic.jpg)`,
			replacements: map[string]string{"temp/pic.jpg": "uploads/pic.jpg"},
			expected:     `![Alt](uploads/pic.jpg)`,
		},
		{
			name:         "LinkReplacement",
			markdown:     `[Text](temp/doc.pdf)`,
			replacements: map[string]string{"temp/doc.pdf": "uploads/doc.pdf"},
			expected:     `[Text](uploads/doc.pdf)`,
		},
		{
			name:     "MultipleReplacements",
			markdown: `![Image](temp/pic.jpg) [Link](temp/doc.pdf)`,
			replacements: map[string]string{
				"temp/pic.jpg": "uploads/pic.jpg",
				"temp/doc.pdf": "uploads/doc.pdf",
			},
			expected: `![Image](uploads/pic.jpg) [Link](uploads/doc.pdf)`,
		},
		{
			name:         "PreserveDoubleQuoteTitle",
			markdown:     `![Alt](temp/pic.jpg "Title")`,
			replacements: map[string]string{"temp/pic.jpg": "uploads/pic.jpg"},
			expected:     `![Alt](uploads/pic.jpg "Title")`,
		},
		{
			name:         "PreserveSingleQuoteTitle",
			markdown:     `[Text](temp/doc.pdf 'Title')`,
			replacements: map[string]string{"temp/doc.pdf": "uploads/doc.pdf"},
			expected:     `[Text](uploads/doc.pdf 'Title')`,
		},
		{
			name:         "NoMatchingUrls",
			markdown:     `![Image](other/pic.jpg)`,
			replacements: map[string]string{"temp/pic.jpg": "uploads/pic.jpg"},
			expected:     `![Image](other/pic.jpg)`,
		},
		{
			name:     "PartialReplacement",
			markdown: `![Image1](temp/pic1.jpg) [Link](temp/doc.pdf) ![Image2](keep/pic2.jpg)`,
			replacements: map[string]string{
				"temp/pic1.jpg": "uploads/pic1.jpg",
				"temp/doc.pdf":  "uploads/doc.pdf",
			},
			expected: `![Image1](uploads/pic1.jpg) [Link](uploads/doc.pdf) ![Image2](keep/pic2.jpg)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceMarkdownURLs(tt.markdown, tt.replacements)
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestIsRelativeURL tests relative URL detection.
func TestIsRelativeURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "EmptyString",
			url:      "",
			expected: false,
		},
		{
			name:     "WhitespaceOnly",
			url:      "   ",
			expected: false,
		},
		{
			name:     "RelativePath",
			url:      "temp/pic.jpg",
			expected: true,
		},
		{
			name:     "RelativePathWithLeadingSlash",
			url:      "/uploads/pic.jpg",
			expected: true,
		},
		{
			name:     "HttpURL",
			url:      "http://example.com/pic.jpg",
			expected: false,
		},
		{
			name:     "HttpsURL",
			url:      "https://example.com/pic.jpg",
			expected: false,
		},
		{
			name:     "UrlWithSpaces",
			url:      "temp/pic with spaces.jpg",
			expected: true,
		},
		{
			name:     "UrlWithSpecialCharacters",
			url:      "temp/doc-file_name@v2.pdf",
			expected: true,
		},
		{
			name:     "UrlWithLeadingSpaces",
			url:      "  temp/pic.jpg",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRelativeURL(tt.url)
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

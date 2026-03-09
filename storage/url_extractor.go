package storage

import (
	"regexp"
	"strings"

	"github.com/dlclark/regexp2"

	collections "github.com/coldsmirk/go-collections"
)

// Uses regexp2 instead of standard library because backreferences (\1, \2) are needed
// to ensure opening and closing quotes match (e.g., reject src="url').
var (
	htmlImgSrc     = regexp2.MustCompile(`(?i)<img[^>]+src\s*=\s*(["'])([^"']+)\1`, regexp2.None)
	htmlAHref      = regexp2.MustCompile(`(?i)<a[^>]+href\s*=\s*(["'])([^"']+)\1`, regexp2.None)
	htmlVideoSrc   = regexp2.MustCompile(`(?i)<video[^>]+src\s*=\s*(["'])([^"']+)\1`, regexp2.None)
	htmlAudioSrc   = regexp2.MustCompile(`(?i)<audio[^>]+src\s*=\s*(["'])([^"']+)\1`, regexp2.None)
	htmlSourceSrc  = regexp2.MustCompile(`(?i)<source[^>]+src\s*=\s*(["'])([^"']+)\1`, regexp2.None)
	htmlEmbedSrc   = regexp2.MustCompile(`(?i)<embed[^>]+src\s*=\s*(["'])([^"']+)\1`, regexp2.None)
	htmlObjectData = regexp2.MustCompile(`(?i)<object[^>]+data\s*=\s*(["'])([^"']+)\1`, regexp2.None)

	htmlURLPatterns = []*regexp2.Regexp{
		htmlImgSrc,
		htmlAHref,
		htmlVideoSrc,
		htmlAudioSrc,
		htmlSourceSrc,
		htmlEmbedSrc,
		htmlObjectData,
	}

	// Group 1: attribute name, Group 2: quote type, Group 3: URL value.
	htmlAttrReplacePattern = regexp2.MustCompile(`(?i)(src|href|data)\s*=\s*(["'])([^"']+)\2`, regexp2.None)
)

var (
	markdownImagePattern = regexp.MustCompile(`!\[([^]]*)]\(([^)]+)\)`) // ![alt](url)
	markdownLinkPattern  = regexp.MustCompile(`\[([^]]*)]\(([^)]+)\)`)  // [text](url), allows empty text

	markdownURLPatterns = []*regexp.Regexp{
		markdownImagePattern,
		markdownLinkPattern,
	}
)

// isRelativeURL checks if a URL is a relative path (not http:// or https://)
func isRelativeURL(url string) bool {
	url = strings.TrimSpace(url)

	return url != "" &&
		!strings.HasPrefix(url, "http://") &&
		!strings.HasPrefix(url, "https://")
}

// extractHtmlURLs extracts all relative URLs from HTML content.
func extractHtmlURLs(content string) []string {
	if content == "" {
		return nil
	}

	urlSet := collections.NewHashSet[string]()

	for _, pattern := range htmlURLPatterns {
		// regexp2 requires iterative FindNextMatch instead of FindAllStringSubmatch
		match, err := pattern.FindStringMatch(content)
		for match != nil && err == nil {
			// Group 0: entire match, Group 1: quote, Group 2: URL
			groups := match.Groups()
			if len(groups) > 2 {
				url := strings.TrimSpace(groups[2].String())
				if isRelativeURL(url) {
					urlSet.Add(url)
				}
			}

			match, err = pattern.FindNextMatch(match)
		}
	}

	return urlSet.ToSlice()
}

// replaceHtmlURLs replaces URLs in HTML content based on the replacement map.
func replaceHtmlURLs(content string, replacements map[string]string) string {
	if content == "" || len(replacements) == 0 {
		return content
	}

	var (
		result    strings.Builder
		lastIndex int
	)

	match, err := htmlAttrReplacePattern.FindStringMatch(content)
	for match != nil && err == nil {
		if groups := match.Groups(); len(groups) > 3 {
			// Group 0: entire match, Group 1: attribute name, Group 2: quote, Group 3: URL
			attrName := groups[1].String()
			quote := groups[2].String()
			oldURL := groups[3].String()

			result.WriteString(content[lastIndex:groups[0].Index])

			if newURL, ok := replacements[oldURL]; ok {
				// Preserve original quote type to maintain HTML consistency
				result.WriteString(attrName)
				result.WriteByte('=')
				result.WriteString(quote)
				result.WriteString(newURL)
				result.WriteString(quote)
			} else {
				result.WriteString(groups[0].String())
			}

			lastIndex = groups[0].Index + groups[0].Length
		}

		match, err = htmlAttrReplacePattern.FindNextMatch(match)
	}

	result.WriteString(content[lastIndex:])

	return result.String()
}

// extractMarkdownURLs extracts all relative URLs from Markdown content.
func extractMarkdownURLs(content string) []string {
	if content == "" {
		return nil
	}

	urlSet := collections.NewHashSet[string]()

	for _, pattern := range markdownURLPatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 2 {
				url := strings.TrimSpace(match[2])
				// Markdown allows optional titles: (url "title") or (url 'title')
				// Strip the title to get just the URL
				if idx := strings.IndexAny(url, `"'`); idx > 0 {
					url = strings.TrimSpace(url[:idx])
				}

				if isRelativeURL(url) {
					urlSet.Add(url)
				}
			}
		}
	}

	return urlSet.ToSlice()
}

// buildMarkdownReplacement builds a replacement string for markdown image or link.
func buildMarkdownReplacement(prefix, text, newURL, title string) string {
	var sb strings.Builder

	sb.WriteString(prefix)
	sb.WriteByte('[')
	sb.WriteString(text)
	sb.WriteByte(']')
	sb.WriteByte('(')
	sb.WriteString(newURL)

	if title != "" {
		sb.WriteByte(' ')
		sb.WriteString(title)
	}

	sb.WriteByte(')')

	return sb.String()
}

// replaceMarkdownURLs replaces URLs in Markdown content based on the replacement map.
func replaceMarkdownURLs(content string, replacements map[string]string) string {
	if content == "" || len(replacements) == 0 {
		return content
	}

	replaceFunc := func(pattern *regexp.Regexp, prefix string) func(string) string {
		return func(match string) string {
			subMatches := pattern.FindStringSubmatch(match)
			if len(subMatches) <= 2 {
				return match
			}

			text := subMatches[1]
			url := strings.TrimSpace(subMatches[2])

			// Preserve optional title if present
			title := ""
			if index := strings.IndexAny(url, `"'`); index > 0 {
				title = url[index:]
				url = strings.TrimSpace(url[:index])
			}

			if newURL, ok := replacements[url]; ok {
				return buildMarkdownReplacement(prefix, text, newURL, title)
			}

			return match
		}
	}

	result := markdownImagePattern.ReplaceAllStringFunc(content, replaceFunc(markdownImagePattern, "!"))
	result = markdownLinkPattern.ReplaceAllStringFunc(result, replaceFunc(markdownLinkPattern, ""))

	return result
}

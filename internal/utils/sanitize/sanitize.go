package sanitize

import (
	"html"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// strict is a cached bluemonday policy that removes all HTML tags and attributes.
// It's safe for concurrent use as bluemonday.Policy is read-only after build.
// WARNING: Never call mutating helpers (e.g. AddAttr, AllowElements) on this policy
// after initialization as it would create a data race.
var strict = func() *bluemonday.Policy {
	p := bluemonday.StrictPolicy()
	p.AddSpaceWhenStrippingTag(true) // Prevents word concatenation
	return p
}()

// Sanitize strips all HTML from arbitrary user input while preserving readability.
//
// All note text must pass through sanitize.Sanitize before hitting the DB.
// Repositories assume already-sanitized input.
//
// Examples:
//   - "<script>alert('xss')</script>Hello" -> "Hello"
//   - "<p>Hello <b>world</b></p>" -> "Hello world " (note the space)
//   - "**markdown** text" -> "**markdown** text" (preserved)
func Sanitize(s string) string {
	return strict.Sanitize(s)
}

// Clean sanitizes HTML and normalizes whitespace for clean storage.
// This is the recommended function for processing user input before persistence.
//
// It performs the following steps:
//  1. Strips all HTML tags while preserving spacing
//  2. Trims leading/trailing whitespace
//  3. Unescapes HTML entities for clean plaintext
//  4. Collapses multiple consecutive spaces to single space
//  5. Normalizes non-breaking spaces to regular spaces
//
// Examples:
//   - "<p>hi</p>" -> "hi"
//   - "<b>a</b> <b>b</b>" -> "a b"
//   - "  <p>Hello</p>  " -> "Hello"
//   - "&nbsp;test&#13;" -> " test\r"
func Clean(s string) string {
	sanitized := strict.Sanitize(s)
	sanitized = strings.TrimSpace(sanitized)

	// Unescape HTML entities first to handle &#13; etc. as single chars
	sanitized = html.UnescapeString(sanitized)

	// Replace non-breaking spaces with regular spaces for better search/indexing
	sanitized = strings.ReplaceAll(sanitized, "\u00a0", " ")

	// Collapse multiple spaces efficiently while preserving newlines
	lines := strings.Split(sanitized, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	sanitized = strings.Join(lines, "\n")

	return sanitized
}

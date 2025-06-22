package sanitize

import (
	"strings"
	"testing"
)

const (
	JustPlainText = "Just plain text"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes script tags",
			input: `<script>alert('xss')</script>Hello world`,
			want:  "Hello world",
		},
		{
			name:  "removes image with onerror",
			input: `<img src=x onerror=alert(1)><p>Hello <b>world</b></p>`,
			want:  "  Hello  world  ",
		},
		{
			name:  "removes all HTML tags with proper spacing",
			input: `<div><p>Hello <b>world</b></p><br><a href="http://example.com">link</a></div>`,
			want:  "  Hello  world    link  ",
		},
		{
			name:  "preserves plain text",
			input: JustPlainText,
			want:  JustPlainText,
		},
		{
			name:  "handles empty string",
			input: "",
			want:  "",
		},
		{
			name:  "removes dangerous attributes",
			input: `<p onclick="alert('xss')">Safe text</p>`,
			want:  " Safe text ",
		},
		{
			name:  "preserves markdown-like syntax",
			input: "# Heading\n**bold** text\n[link](http://example.com)",
			want:  "# Heading\n**bold** text\n[link](http://example.com)",
		},
		{
			name:  "simple case for readability",
			input: `<p>Hello world</p>`,
			want:  " Hello world ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sanitize(tt.input)
			if got != tt.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}

			// Additional security check: ensure no HTML tags survive
			if strings.Contains(got, "<") || strings.Contains(got, ">") {
				t.Errorf("Sanitize(%q) still contains HTML tags: %q", tt.input, got)
			}
		})
	}
}

func TestClean(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "leading/trailing artefacts",
			input: "<p>hi</p>",
			want:  "hi",
		},
		{
			name:  "double spaces inside text",
			input: "<b>a</b> <b>b</b>",
			want:  "a b",
		},
		{
			name:  "leading and trailing whitespace",
			input: "  <p>Hello</p>  ",
			want:  "Hello",
		},
		{
			name:  "preserves plain text",
			input: JustPlainText,
			want:  JustPlainText,
		},
		{
			name:  "handles empty string",
			input: "",
			want:  "",
		},
		{
			name:  "removes script tags and cleans",
			input: `  <script>alert('xss')</script>Hello world  `,
			want:  "Hello world",
		},
		{
			name:  "preserves markdown-like syntax",
			input: "  # Heading\n**bold** text  ",
			want:  "# Heading\n**bold** text",
		},
		{
			name:  "multiple spaces collapsed",
			input: "<p>Hello</p>   <p>World</p>",
			want:  "Hello World",
		},
		{
			name:  "complex markup cleaned",
			input: "  <div><p>Hello <b>world</b></p><br><a href='#'>link</a></div>  ",
			want:  "Hello world link",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Clean(tt.input)
			if got != tt.want {
				t.Errorf("Clean(%q) = %q, want %q", tt.input, got, tt.want)
			}

			// Additional security check: ensure no HTML tags survive
			if strings.Contains(got, "<script") || strings.Contains(got, "onerror") {
				t.Errorf("Clean(%q) still contains dangerous content: %q", tt.input, got)
			}
		})
	}
}

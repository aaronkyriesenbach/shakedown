package cloudsync

import (
	"strings"
	"testing"
	"time"
)

func TestSanitizeSegment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal title",
			input:    "My Show",
			expected: "My Show",
		},
		{
			name:     "control chars and invalid path chars",
			input:    "Bad/Title\\with:invalid*chars?\"<>|\x00\x1F\x7F",
			expected: "Bad_Title_with_invalid_chars________",
		},
		{
			name:     "empty after sanitize",
			input:    "   ...   ",
			expected: "untitled",
		},
		{
			name:     "collapse whitespace",
			input:    "Title    with   spaces",
			expected: "Title with spaces",
		},
		{
			name:     "unicode/emoji title longer than 200 runes",
			input:    "a" + strings.Repeat("A", 198) + "😊b",
			expected: "a" + strings.Repeat("A", 198) + "😊",
		},
		{
			name:     "path traversal entirely treated as one segment",
			input:    "../../etc/passwd",
			expected: "_.._etc_passwd",
		},
	}

	longEmoji := ""
	for i := 0; i < 205; i++ {
		longEmoji += "😊"
	}
	expectedEmoji := ""
	for i := 0; i < 200; i++ {
		expectedEmoji += "😊"
	}

	tests = append(tests, struct {
		name     string
		input    string
		expected string
	}{
		name:     "unicode/emoji title > 200 runes",
		input:    longEmoji,
		expected: expectedEmoji,
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeSegment(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeSegment(%q) = %q, want %q", tt.input, result, tt.expected)
			}
			runes := []rune(result)
			if len(runes) > 200 {
				t.Errorf("SanitizeSegment result > 200 runes: got %d", len(runes))
			}
		})
	}
}

func TestRenderPath(t *testing.T) {
	recordedTime := time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)
	createdTime := time.Date(2023, 5, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		tmpl     string
		root     string
		meta     RecordingMeta
		expected string
	}{
		{
			name: "normal case with recorded_at",
			tmpl: "{year}/{date}/{title}.{ext}",
			root: "Shakedown",
			meta: RecordingMeta{
				ID:         "1234",
				Title:      "My Show",
				FileExt:    ".mp3",
				RecordedAt: recordedTime,
			},
			expected: "Shakedown/2024/2024-01-02/My Show.mp3",
		},
		{
			name: "fallback to created_at if recorded_at is zero",
			tmpl: "{year}/{month}/{date}/{title}.{ext}",
			root: "Shakedown",
			meta: RecordingMeta{
				ID:         "1234",
				Title:      "My Show",
				FileExt:    "mp3", // Test without leading dot
				RecordedAt: time.Time{},
				CreatedAt:  createdTime,
			},
			expected: "Shakedown/2023/05/2023-05-06/My Show.mp3",
		},
		{
			name: "unknown date if both are zero",
			tmpl: "{year}/{date}/{month}/{title}.{ext}",
			root: "Shakedown",
			meta: RecordingMeta{
				Title:   "My Show",
				FileExt: ".mp3",
			},
			expected: "Shakedown/unknown-date/unknown-date/unknown-date/My Show.mp3",
		},
		{
			name: "strips leading dot from file_ext",
			tmpl: "{title}.{ext}",
			root: "Shakedown",
			meta: RecordingMeta{
				Title:   "My Show",
				FileExt: ".mp3",
			},
			expected: "Shakedown/My Show.mp3",
		},
		{
			name: "path traversal in title is sanitized without escaping",
			tmpl: "{year}/{title}.{ext}",
			root: "Shakedown",
			meta: RecordingMeta{
				Title:      "../../etc/passwd",
				FileExt:    ".mp3",
				RecordedAt: recordedTime,
			},
			expected: "Shakedown/2024/_.._etc_passwd.mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderPath(tt.tmpl, tt.root, tt.meta)
			if result != tt.expected {
				t.Errorf("RenderPath() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSuffixPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		shortID  string
		expected string
	}{
		{
			name:     "normal path with extension",
			path:     "a/b/My Show.mp3",
			shortID:  "1a2b3c4d",
			expected: "a/b/My Show-1a2b3c4d.mp3",
		},
		{
			name:     "path without extension",
			path:     "a/b/noext",
			shortID:  "1a2b3c4d",
			expected: "a/b/noext-1a2b3c4d",
		},
		{
			name:     "path with multiple dots",
			path:     "a/b/archive.tar.gz",
			shortID:  "1a2b3c4d",
			expected: "a/b/archive.tar-1a2b3c4d.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SuffixPath(tt.path, tt.shortID)
			if result != tt.expected {
				t.Errorf("SuffixPath(%q, %q) = %q, want %q", tt.path, tt.shortID, result, tt.expected)
			}
		})
	}
}

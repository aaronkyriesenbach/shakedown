package cloudsync

import (
	"path"
	"regexp"
	"strings"
	"time"
)

// RecordingMeta represents the recording metadata needed for path templating.
// This mirrors the internal/recordings.Recording struct but is defined here
// to avoid a cyclic dependency.
type RecordingMeta struct {
	ID         string
	Title      string
	FileExt    string
	RecordedAt time.Time
	CreatedAt  time.Time
}

var (
	invalidChars = regexp.MustCompile(`[\/\\:\*\?"<>\|\x00-\x1F\x7F]`)
	multiSpace   = regexp.MustCompile(`\s+`)
)

func trimSpacesAndDots(s string) string {
	for {
		trimmed := strings.TrimSpace(s)
		trimmed = strings.Trim(trimmed, ".")
		if trimmed == s {
			return trimmed
		}
		s = trimmed
	}
}

// SanitizeSegment sanitizes a path segment (e.g. title) according to the rules:
// - Replace / \ : * ? " < > | and control characters with _
// - Collapse multiple internal whitespaces to a single space
// - Trim leading/trailing whitespace and dots
// - Truncate to exactly 200 RUNES (unicode-safe)
// - If empty after processing, return "untitled"
func SanitizeSegment(s string) string {
	s = invalidChars.ReplaceAllString(s, "_")
	s = multiSpace.ReplaceAllString(s, " ")
	s = trimSpacesAndDots(s)

	runes := []rune(s)
	if len(runes) > 200 {
		runes = runes[:200]
		s = string(runes)
		s = trimSpacesAndDots(s)
	} else {
		s = string(runes)
	}

	if s == "" {
		return "untitled"
	}

	return s
}

// RenderPath renders a remote path given a template, root path, and recording metadata.
func RenderPath(tmpl, root string, r RecordingMeta) string {
	year, month, date := "unknown-date", "unknown-date", "unknown-date"

	t := r.RecordedAt
	if t.IsZero() {
		t = r.CreatedAt
	}

	if !t.IsZero() {
		year = t.Format("2006")
		month = t.Format("01")
		date = t.Format("2006-01-02")
	}

	ext := strings.TrimPrefix(r.FileExt, ".")
	title := SanitizeSegment(r.Title)

	res := tmpl
	res = strings.ReplaceAll(res, "{year}", year)
	res = strings.ReplaceAll(res, "{month}", month)
	res = strings.ReplaceAll(res, "{date}", date)
	res = strings.ReplaceAll(res, "{title}", title)
	res = strings.ReplaceAll(res, "{ext}", ext)
	res = strings.ReplaceAll(res, "{id}", r.ID)

	// Since res is the templated path relative to root, we path.Join it with root.
	// path.Join handles cleaning and standardizing to forward slashes.
	if root == "" {
		return path.Clean(res)
	}
	return path.Join(root, res)
}

// SuffixPath inserts -<shortID> immediately before the final file extension.
// If there is no extension, it appends it to the end.
func SuffixPath(pathStr, shortID string) string {
	ext := path.Ext(pathStr)
	if ext == "" {
		return pathStr + "-" + shortID
	}
	base := strings.TrimSuffix(pathStr, ext)
	return base + "-" + shortID + ext
}

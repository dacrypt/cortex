package entity

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxTagLength = 32
const MaxTagWords = 3

// NormalizeTag converts input into a simple hashtag-style tag.
// It lowercases, strips leading '#', replaces spaces/underscores with hyphens,
// and removes punctuation.
func NormalizeTag(tag string) string {
	normalized := strings.TrimSpace(strings.ToLower(tag))
	if normalized == "" {
		return ""
	}

	normalized = strings.TrimLeft(normalized, "#")
	if normalized == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(normalized))
	prevHyphen := false

	for _, r := range normalized {
		switch {
		case unicode.IsSpace(r) || r == '_':
			if !prevHyphen {
				builder.WriteByte('-')
				prevHyphen = true
			}
		case r == '-':
			if !prevHyphen {
				builder.WriteByte('-')
				prevHyphen = true
			}
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			builder.WriteRune(unicode.ToLower(r))
			prevHyphen = false
		default:
			// Drop punctuation/symbols.
		}
	}

	cleaned := strings.Trim(builder.String(), "-")
	if cleaned == "" {
		return ""
	}
	if strings.Count(cleaned, "-")+1 > MaxTagWords {
		return ""
	}
	if utf8.RuneCountInString(cleaned) > MaxTagLength {
		return ""
	}
	return cleaned
}

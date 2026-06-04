package parser

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type fieldMatch struct {
	Key      string
	Value    string
	Optional bool
	Pattern  string
	Start    int
	End      int
}

// InferRegex builds a Fluent-Bit compatible regex from a log snippet and user-provided key/value pairs.
//
// The inference is intentionally conservative. Required fields are inferred from the
// exact value provided by the user. Optional fields are rendered as optional capture
// groups. For optional fields, a custom pattern is recommended because a generic
// pattern such as \S+ may steal the first token from the message when the optional
// field is absent.
func InferRegex(snippet string, fields []RegexField, enableMultiline bool) (string, error) {
	if strings.TrimSpace(snippet) == "" {
		return "", errors.New("snippet is empty")
	}

	if len(fields) == 0 {
		return "", errors.New("no regex fields provided")
	}

	matches := make([]fieldMatch, 0, len(fields))

	for _, field := range fields {
		key := strings.TrimSpace(field.Key)
		value := field.Value

		if key == "" {
			return "", errors.New("field key is empty")
		}

		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("field %q has empty value", key)
		}

		start := strings.Index(snippet, value)
		if start < 0 {
			return "", fmt.Errorf("value for field %q not found in snippet: %q", key, value)
		}

		// Detect ambiguous value.
		if strings.Count(snippet, value) > 1 {
			return "", fmt.Errorf("value for field %q appears multiple times in snippet: %q", key, value)
		}

		matches = append(matches, fieldMatch{
			Key:      sanitizeGroupName(key),
			Value:    value,
			Optional: field.Optional,
			Pattern:  strings.TrimSpace(field.Pattern),
			Start:    start,
			End:      start + len(value),
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Start < matches[j].Start
	})

	if err := validateNoOverlap(matches); err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("^")

	cursor := 0

	for _, match := range matches {
		isLastField := match.End == len(snippet)

		// Fixed text before this field.
		prefix := snippet[cursor:match.Start]

		if match.Optional {
			// Put the prefix inside the optional group.
			//
			// Example:
			//   level user message
			//
			// Required rendering would be:
			//   (?<level>[A-Z]+)\s+(?<user>\S+)\s+(?<message>[\s\S]*)
			//
			// Optional rendering should be:
			//   (?<level>[A-Z]+)(?:\s+(?<user>custom_pattern))?\s+(?<message>[\s\S]*)
			//
			// If the prefix is kept outside the optional group, the generated regex
			// would still require the separator even when the optional field is absent.
			builder.WriteString("(?:")
			builder.WriteString(renderLiteral(prefix))
			builder.WriteString("(?<")
			builder.WriteString(match.Key)
			builder.WriteString(">")
			builder.WriteString(inferOptionalValuePattern(match))
			builder.WriteString(")")
			builder.WriteString(")?")
		} else {
			builder.WriteString(renderLiteral(prefix))
			builder.WriteString("(?<")
			builder.WriteString(match.Key)
			builder.WriteString(">")
			builder.WriteString(inferValuePattern(match.Key, match.Value, match.End == len(snippet), match.Pattern))
			builder.WriteString(")")
		}

		cursor = match.End

		// For multiline logs, consume the final newline outside the capture group.
		// This keeps message clean while still matching the full multiline record.
		if enableMultiline && isLastField {
			builder.WriteString(`\n`)
		}
	}

	// Fixed text after last field.
	suffix := snippet[cursor:]
	builder.WriteString(renderLiteral(suffix))

	builder.WriteString("$")

	return builder.String(), nil
}

func validateNoOverlap(matches []fieldMatch) error {
	for i := 1; i < len(matches); i++ {
		prev := matches[i-1]
		curr := matches[i]

		if curr.Start < prev.End {
			return fmt.Errorf(
				"field %q overlaps with field %q",
				curr.Key,
				prev.Key,
			)
		}
	}

	return nil
}

func sanitizeGroupName(key string) string {
	key = strings.TrimSpace(key)

	var builder strings.Builder

	for i, r := range key {
		if r == '_' ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9' && i > 0) {
			builder.WriteRune(r)
			continue
		}

		builder.WriteRune('_')
	}

	out := builder.String()
	out = strings.Trim(out, "_")

	if out == "" {
		return "field"
	}

	return out
}

// renderLiteral escapes fixed text between user-provided values.
//
// It also makes whitespace flexible:
//
//	"   " -> \s+
func renderLiteral(s string) string {
	if s == "" {
		return ""
	}

	var builder strings.Builder
	var inSpace bool

	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !inSpace {
				builder.WriteString(`\s+`)
				inSpace = true
			}
			continue
		}

		inSpace = false
		builder.WriteString(regexp.QuoteMeta(string(r)))
	}

	return builder.String()
}

func inferOptionalValuePattern(match fieldMatch) string {
	if pattern := normalizeCustomPattern(match.Pattern); pattern != "" {
		return pattern
	}

	// Without a custom pattern, keep optional matching conservative by matching
	// the exact sample value. This avoids a generic \S+ optional field stealing
	// the first message token when the optional field is absent.
	//
	// Users can provide a custom pattern such as:
	//   itsm_[A-Za-z0-9_]+
	// when the optional value may vary across log lines.
	return regexp.QuoteMeta(match.Value)
}

func normalizeCustomPattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return ""
	}

	// Allow users to paste either:
	//   itsm_[A-Za-z0-9_]+
	// or:
	//   /itsm_[A-Za-z0-9_]+/
	if len(pattern) >= 2 && strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
		return strings.TrimSuffix(strings.TrimPrefix(pattern, "/"), "/")
	}

	return pattern
}

func isMessageField(key string) bool {
	lowerKey := strings.ToLower(key)

	return strings.Contains(lowerKey, "message") || strings.Contains(lowerKey, "msg")
}

func inferMessageRegex() string {
	return `[\s\S]*`
}

// inferValuePattern infers a regex pattern for one field value.
//
// This is intentionally opinionated and conservative.
func inferValuePattern(key string, value string, isLast bool, customPattern string) string {
	if pattern := normalizeCustomPattern(customPattern); pattern != "" {
		return pattern
	}

	lowerKey := strings.ToLower(key)

	switch {
	case looksLikeTimestamp(value):
		return inferTimestampRegex(value)

	case looksLikeLevel(lowerKey, value):
		return `[A-Z]+`

	case looksLikeInteger(value):
		return `\d+`

	case looksLikeFloat(value):
		return `\d+(?:\.\d+)?`

	case isMessageField(lowerKey):
		return inferMessageRegex()

	case isLast:
		// Last field is often message-like.
		return `.*`

	case strings.ContainsAny(value, " \t"):
		return `.+?`

	default:
		return `\S+`
	}
}

func looksLikeTimestamp(value string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^\d{4}[-/]\d{2}[-/]\d{2}[T\s_]\d{2}:\d{2}:\d{2}`),
		regexp.MustCompile(`^\d{4}[-/]\d{2}[-/]\d{2}$`),
	}

	for _, p := range patterns {
		if p.MatchString(value) {
			return true
		}
	}

	return false
}

func inferTimestampRegex(value string) string {
	tokens := Tokenize(value)

	var builder strings.Builder
	numberIndex := 0

	for _, token := range tokens {
		switch token.Type {
		case "number":
			switch numberIndex {
			case 0:
				builder.WriteString(`\d{4}`)
			default:
				builder.WriteString(`\d{2}`)
			}
			numberIndex++

		case "millisecond":
			if strings.HasPrefix(token.Value, ",") {
				builder.WriteString(`,\d+`)
			} else {
				builder.WriteString(`\.\d+`)
			}

		case "timezone":
			builder.WriteString(`(?:Z|z|[+-]\d{2}:?\d{2})`)

		case "literal":
			if token.Value == " " || token.Value == "\t" {
				builder.WriteString(`\s+`)
			} else {
				builder.WriteString(regexp.QuoteMeta(token.Value))
			}
		}
	}

	return builder.String()
}

func looksLikeLevel(key string, value string) bool {
	if strings.Contains(key, "level") {
		return true
	}

	switch value {
	case "TRACE", "DEBUG", "INFO", "WARN", "WARNING", "ERROR", "FATAL":
		return true
	default:
		return false
	}
}

func looksLikeInteger(value string) bool {
	return regexp.MustCompile(`^\d+$`).MatchString(value)
}

func looksLikeFloat(value string) bool {
	return regexp.MustCompile(`^\d+\.\d+$`).MatchString(value)
}

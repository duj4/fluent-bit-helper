package parser

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type fieldMatch struct {
	Key   string
	Value string
	Start int
	End   int
}

// InferRegex builds a Fluent-Bit compatible regex from a log snippet and user-provided key/value pairs.
//
// Example:
//
// snippet:
//
//	2026-05-18 10:23:44 INFO [main] com.demo.App - started successfully
//
// fields:
//
//	timestamp = 2026-05-18 10:23:44
//	level     = INFO
//	thread    = main
//	logger    = com.demo.App
//	message   = started successfully
//
// output:
//
//	^(?<timestamp>\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})\s+(?<level>[A-Z]+)\s+\[(?<thread>[^\]]+)\]\s+(?<logger>\S+)\s+\-\s+(?<message>.*)$
func InferRegex(snippet string, fields []RegexField) (string, error) {
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
			Key:   sanitizeGroupName(key),
			Value: value,
			Start: start,
			End:   start + len(value),
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
		// Fixed text before this field.
		prefix := snippet[cursor:match.Start]
		builder.WriteString(renderLiteral(prefix))

		// Named capture group.
		builder.WriteString("(?<")
		builder.WriteString(match.Key)
		builder.WriteString(">")
		builder.WriteString(inferValuePattern(match.Key, match.Value, match.End == len(snippet)))
		builder.WriteString(")")

		cursor = match.End
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

// inferValuePattern infers a regex pattern for one field value.
//
// This is intentionally opinionated and conservative.
func inferValuePattern(key string, value string, isLast bool) string {
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

	case strings.Contains(lowerKey, "message") || strings.Contains(lowerKey, "msg"):
		return `.*`

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
			builder.WriteString(`\.\d+`)

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

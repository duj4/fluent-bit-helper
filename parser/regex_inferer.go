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
func InferRegex(snippet string, fields []RegexField, enableMultiline bool) (string, error) {
	if strings.TrimSpace(snippet) == "" {
		return "", errors.New("snippet is empty")
	}

	if len(fields) == 0 {
		return "", errors.New("no regex fields provided")
	}

	candidates := make([][]fieldMatch, 0, len(fields))
	hasAmbiguousValue := false
	var ambiguousKey string
	var ambiguousValue string

	for _, field := range fields {
		key := strings.TrimSpace(field.Key)
		value := field.Value

		if key == "" {
			return "", errors.New("field key is empty")
		}

		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("field %q has empty value", key)
		}

		starts := findValueOccurrences(snippet, value)
		if len(starts) == 0 {
			return "", fmt.Errorf("value for field %q not found in snippet: %q", key, value)
		}

		if len(starts) > 1 && !hasAmbiguousValue {
			hasAmbiguousValue = true
			ambiguousKey = key
			ambiguousValue = value
		}

		fieldCandidates := make([]fieldMatch, 0, len(starts))
		for _, start := range starts {
			fieldCandidates = append(fieldCandidates, fieldMatch{
				Key:      sanitizeGroupName(key),
				Value:    value,
				Optional: field.Optional,
				Pattern:  strings.TrimSpace(field.Pattern),
				Start:    start,
				End:      start + len(value),
			})
		}
		candidates = append(candidates, fieldCandidates)
	}

	matches := make([]fieldMatch, 0, len(fields))
	if hasAmbiguousValue {
		var ok bool
		matches, ok = resolveOrderedFieldMatches(candidates)
		if !ok {
			return "", fmt.Errorf(
				"value for field %q appears multiple times in snippet: %q",
				ambiguousKey,
				ambiguousValue,
			)
		}
	} else {
		for _, fieldCandidates := range candidates {
			matches = append(matches, fieldCandidates[0])
		}
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
			pattern, err := optionalValuePattern(match)
			if err != nil {
				return "", err
			}

			// Access logs commonly replace an optional quoted value with an unquoted dash.
			if isDoubleQuotedField(snippet, match) {
				builder.WriteString("(?:")
				builder.WriteString(renderLiteral(prefix[:len(prefix)-1]))
				builder.WriteString(`(?:"(?<`)
				builder.WriteString(match.Key)
				builder.WriteString(">")
				builder.WriteString(pattern)
				builder.WriteString(`)"|-)`)
				builder.WriteString(")?")
				cursor = match.End + 1
				continue
			}

			// Keep the separator before an optional field inside the optional group.
			//
			// Example:
			//
			//   WARN user_a message
			//   ERROR message
			//
			// This must render as:
			//
			//   (?<level>[A-Z]+)(?:\s+(?<user>...))?\s+(?<message>...)
			//
			// rather than:
			//
			//   (?<level>[A-Z]+)\s+(?:(?<user>...))?\s+(?<message>...)
			//
			// Otherwise rows without the optional token would require two separators.
			builder.WriteString("(?:")
			builder.WriteString(renderLiteral(prefix))
			builder.WriteString("(?<")
			builder.WriteString(match.Key)
			builder.WriteString(">")
			builder.WriteString(pattern)
			builder.WriteString(")")
			builder.WriteString(")?")
		} else {
			builder.WriteString(renderLiteral(prefix))

			// Named capture group.
			builder.WriteString("(?<")
			builder.WriteString(match.Key)
			builder.WriteString(">")
			builder.WriteString(inferValuePatternForMatch(match, enableMultiline, isLastField))
			builder.WriteString(")")
		}

		cursor = match.End

		// For multiline logs, consume a trailing newline outside the capture group when it exists.
		// Some merged records do not end with a newline, so keep it optional.
		if enableMultiline && isLastField {
			builder.WriteString(`\n?`)
		}
	}

	// Fixed text after last field.
	suffix := snippet[cursor:]
	builder.WriteString(renderLiteral(suffix))

	if enableMultiline {
		builder.WriteString(`\z`)
	} else {
		builder.WriteString("$")
	}

	return builder.String(), nil
}

func findValueOccurrences(snippet string, value string) []int {
	var starts []int

	for offset := 0; offset <= len(snippet)-len(value); {
		index := strings.Index(snippet[offset:], value)
		if index < 0 {
			break
		}

		start := offset + index
		starts = append(starts, start)
		offset = start + len(value)
	}

	return starts
}

func isDoubleQuotedField(snippet string, match fieldMatch) bool {
	return match.Start > 0 &&
		match.End < len(snippet) &&
		snippet[match.Start-1] == '"' &&
		snippet[match.End] == '"'
}

func resolveOrderedFieldMatches(candidates [][]fieldMatch) ([]fieldMatch, bool) {
	lastUniqueEnd := -1
	for _, fieldCandidates := range candidates {
		if len(fieldCandidates) != 1 {
			continue
		}

		if fieldCandidates[0].Start < lastUniqueEnd {
			return nil, false
		}
		lastUniqueEnd = fieldCandidates[0].End
	}

	var solution []fieldMatch
	solutionCount := 0
	current := make([]fieldMatch, 0, len(candidates))

	var search func(index int, previousEnd int)
	search = func(index int, previousEnd int) {
		if solutionCount > 1 {
			return
		}
		if index == len(candidates) {
			solution = append([]fieldMatch(nil), current...)
			solutionCount++
			return
		}

		for _, candidate := range candidates[index] {
			if candidate.Start < previousEnd {
				continue
			}

			current = append(current, candidate)
			search(index+1, candidate.End)
			current = current[:len(current)-1]
		}
	}

	search(0, 0)
	return solution, solutionCount == 1
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

func shouldRenderOptionalTokenBeforeMessage(snippet string, current fieldMatch, next fieldMatch) bool {
	if !isMessageField(next.Key) {
		return false
	}

	// The heuristic only applies when the message has a stable visible
	// boundary. In many Java/Jira style logs, message starts with a component
	// enclosed in square brackets.
	if !strings.HasPrefix(next.Value, "[") {
		return false
	}

	if isStableRequiredField(current.Key) || isMessageField(current.Key) {
		return false
	}

	// Optional token inference is only safe for single-token fields.
	if strings.ContainsAny(strings.TrimSpace(current.Value), " \t") {
		return false
	}

	gap := snippet[current.End:next.Start]
	return strings.TrimSpace(gap) == ""
}

func isStableRequiredField(key string) bool {
	lowerKey := strings.ToLower(key)

	return lowerKey == "time" ||
		lowerKey == "timestamp" ||
		strings.Contains(lowerKey, "time") ||
		strings.Contains(lowerKey, "level") ||
		strings.Contains(lowerKey, "thread")
}

func isMessageField(key string) bool {
	lowerKey := strings.ToLower(key)

	return strings.Contains(lowerKey, "message") || strings.Contains(lowerKey, "msg")
}

func optionalValuePattern(match fieldMatch) (string, error) {
	pattern := strings.TrimSpace(match.Pattern)
	if pattern == "" {
		// Default for optional single-token fields.
		// It matches one token, but avoids capturing component/message prefixes
		// that commonly start with "[".
		pattern = `[^\s\[]+`
	}

	if _, err := regexp.Compile(pattern); err != nil {
		return "", fmt.Errorf("invalid custom pattern for optional field %q: %w", match.Key, err)
	}

	return pattern, nil
}

func inferMessageRegex(value string) string {
	return `[\s\S]*`
}

func inferValuePatternForMatch(match fieldMatch, enableMultiline bool, isLastField bool) string {
	pattern := inferValuePattern(match.Key, match.Value, isLastField)
	if !enableMultiline || !isLastField {
		return pattern
	}

	switch pattern {
	case `[\s\S]*`:
		return `[\s\S]*?`
	case `.*`:
		return `.*?`
	default:
		return pattern
	}
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
		return inferMessageRegex(value)

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
	if _, ok := inferTimestampParts(value); ok {
		return true
	}

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
	if parts, ok := inferTimestampParts(value); ok {
		return renderTimestampRegex(parts)
	}

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

func renderTimestampRegex(parts []timestampPart) string {
	var builder strings.Builder
	lastWasSpace := false

	for _, part := range parts {
		if part.Kind == "literal" && isWhitespaceLiteral(part.Value) {
			if !lastWasSpace {
				builder.WriteString(`\s+`)
				lastWasSpace = true
			}
			continue
		}

		lastWasSpace = false

		switch part.Kind {
		case "year":
			builder.WriteString(`\d{4}`)
		case "month", "day", "day_space":
			builder.WriteString(`\d+`)
		case "month_name":
			builder.WriteString(`\w+`)
		case "hour", "minute", "second":
			builder.WriteString(`\d{2}`)
		case "fraction":
			if strings.HasPrefix(part.Value, ",") {
				builder.WriteString(`,\d+`)
			} else {
				builder.WriteString(`\.\d+`)
			}
		case "timezone":
			builder.WriteString(`(?:Z|z|[+-]\d{2}:?\d{2})`)
		default:
			builder.WriteString(regexp.QuoteMeta(part.Value))
		}
	}

	return builder.String()
}

package parser

import (
	"fmt"
	"strings"
)

// BuildMultilineParser builds fluent-bit multiline parser configuration
func BuildMultilineParser(input ParserInput) (MultilineParser, error) {
	if strings.TrimSpace(input.MultilineParserName) == "" {
		return MultilineParser{}, fmt.Errorf("multiline parser name is empty")
	}

	startRegex, err := InferMultilineStartRegex(input.Snippet, input.TimeValue)
	if err != nil {
		return MultilineParser{}, err
	}

	contRegex := buildMultilineContRegex(startRegex)

	return MultilineParser{
		Name:         input.MultilineParserName,
		Type:         "regex",
		FlushTimeout: 1000,
		Rules: []MultilineRule{
			{
				State:     "start_state",
				Regex:     wrapFluentBitRegex(startRegex),
				NextState: "cont",
			},
			{
				State:     "cont",
				Regex:     wrapFluentBitRegex(contRegex),
				NextState: "cont",
			},
		},
	}, nil
}

// BuildCombinedMultilineParser builds one multiline parser whose start rule accepts all parser inputs.
func BuildCombinedMultilineParser(inputs []ParserInput) (MultilineParser, error) {
	name := combinedMultilineParserName(inputs)
	if strings.TrimSpace(name) == "" {
		return MultilineParser{}, fmt.Errorf("multiline parser name is empty")
	}

	var startRegexes []string
	for _, input := range inputs {
		startRegex, err := InferMultilineStartRegex(input.Snippet, input.TimeValue)
		if err != nil {
			return MultilineParser{}, err
		}

		startRegexes = append(startRegexes, strings.TrimPrefix(startRegex, "^"))
	}

	if len(startRegexes) == 0 {
		return MultilineParser{}, fmt.Errorf("no multiline start regexes generated")
	}

	startRegex := "^(?:" + strings.Join(startRegexes, "|") + ")"
	contRegex := buildMultilineContRegex(startRegex)

	return MultilineParser{
		Name:         name,
		Type:         "regex",
		FlushTimeout: 1000,
		Rules: []MultilineRule{
			{
				State:     "start_state",
				Regex:     wrapFluentBitRegex(startRegex),
				NextState: "cont",
			},
			{
				State:     "cont",
				Regex:     wrapFluentBitRegex(contRegex),
				NextState: "cont",
			},
		},
	}, nil
}

func combinedMultilineParserName(inputs []ParserInput) string {
	return CombinedMultilineParserName(inputs)
}

// CombinedMultilineParserName returns the shared multiline parser name used for combined inputs.
func CombinedMultilineParserName(inputs []ParserInput) string {
	for _, input := range inputs {
		if input.EnableMultiline && strings.TrimSpace(input.MultilineParserName) != "" {
			return normalizeCombinedMultilineParserName(input.MultilineParserName)
		}
	}

	return ""
}

func normalizeCombinedMultilineParserName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) <= 1 {
		return name
	}

	trimmed := strings.TrimRight(name, "0123456789")
	if trimmed == "" || trimmed == name {
		return name
	}

	return strings.TrimRight(trimmed, "_-")
}

// InferMultilineStartRegex builds multiline start regex from timestamp position.
//
// It assumes a new log event starts with the timestamp pattern.
// Example:
//
// snippet:
//
//	[2026-05-18 10:23:44] INFO started
//
// timestampValue:
//
//	2026-05-18 10:23:44
//
// output:
//
//	^\[\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}
func InferMultilineStartRegex(snippet string, timestampValue string) (string, error) {
	if strings.TrimSpace(snippet) == "" {
		return "", fmt.Errorf("log snippet is empty")
	}

	if strings.TrimSpace(timestampValue) == "" {
		return "", fmt.Errorf("timestamp value is empty")
	}

	if strings.Count(snippet, timestampValue) > 1 {
		return "", fmt.Errorf("timestamp value appears multiple times in snippet: %q", timestampValue)
	}

	start := strings.Index(snippet, timestampValue)
	if start < 0 {
		return "", fmt.Errorf("timestamp value not found in snippet: %q", timestampValue)
	}

	prefix := snippet[:start]

	return "^" + renderLiteral(prefix) + inferTimestampRegex(timestampValue), nil
}

// buildMultilineContRegex builds continuation rule from start regex.
//
// startRegex:
//
//	^\[\d{4}-\d{2}-\d{2}
//
// contRegex:
//
//	^(?!\[\d{4}-\d{2}-\d{2}).*
func buildMultilineContRegex(startRegex string) string {
	condition := strings.TrimPrefix(startRegex, "^")

	return "^(?!" + condition + ").*"
}

// wrapFluentBitRegex wraps regex with /.../ for multiline parser rules.
func wrapFluentBitRegex(regex string) string {
	escaped := strings.ReplaceAll(regex, "/", `\/`)

	return "/" + escaped + "/"
}

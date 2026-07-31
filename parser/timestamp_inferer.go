package parser

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var (
	reTimestampWord = regexp.MustCompile(`^[A-Za-z]+`)
)

type timestampPart struct {
	Kind  string
	Value string
}

type rawTimestampToken struct {
	Kind  string
	Value string
}

// InferTimeFormat converts timestamp value into Fluent-Bit time_format
func InferTimeFormat(value string) (string, error) {
	if parts, ok := inferTimestampParts(value); ok {
		return renderTimeFormat(parts), nil
	}

	tokens := Tokenize(value)
	if len(tokens) == 0 {
		return "", errors.New("failed to tokenize timestamp")
	}

	var builder strings.Builder

	numberIndex := 0

	for _, token := range tokens {
		switch token.Type {
		// number
		case "number":
			switch numberIndex {
			// year
			case 0:
				// year must be 4 digits
				if len(token.Value) != 4 {
					return "", errors.New("unsupported year format")
				}
				builder.WriteString("%Y")

			// month
			case 1:
				builder.WriteString("%m")

			// day
			case 2:
				builder.WriteString("%d")

			// hour
			case 3:
				builder.WriteString("%H")

			// minute
			case 4:
				builder.WriteString("%M")

			// second
			case 5:
				builder.WriteString("%S")

			default:
				return "", errors.New("unsupported timestamp format")
			}
			numberIndex++

		// literal
		case "literal":
			builder.WriteString(token.Value)

		// millisecond
		case "millisecond":
			if strings.HasPrefix(token.Value, ",") {
				builder.WriteString(",%L")
			} else {
				builder.WriteString(".%L")
			}

		// timezone
		case "timezone":
			builder.WriteString("%z")
		}
	}

	return builder.String(), nil
}

func inferTimestampParts(value string) ([]timestampPart, bool) {
	tokens := lexTimestamp(value)
	if len(tokens) == 0 {
		return nil, false
	}

	timeStart := findTimeStart(tokens)
	if timeStart < 0 {
		return inferDateOnlyParts(tokens)
	}

	dateKinds, ok := inferDatePartKinds(tokens[:timeStart])
	if !ok {
		return nil, false
	}

	parts := make([]timestampPart, 0, len(tokens))

	for i := 0; i < timeStart; i++ {
		if kind, ok := dateKinds[i]; ok {
			parts = append(parts, timestampPart{Kind: kind, Value: tokens[i].Value})
			continue
		}

		parts = append(parts, rawTokenPart(tokens[i]))
	}

	parts = append(parts,
		timestampPart{Kind: "hour", Value: tokens[timeStart].Value},
		rawTokenPart(tokens[timeStart+1]),
		timestampPart{Kind: "minute", Value: tokens[timeStart+2].Value},
		rawTokenPart(tokens[timeStart+3]),
		timestampPart{Kind: "second", Value: tokens[timeStart+4].Value},
	)

	for i := timeStart + 5; i < len(tokens); i++ {
		parts = append(parts, rawTokenPart(tokens[i]))
	}

	return parts, true
}

func lexTimestamp(value string) []rawTimestampToken {
	value = strings.TrimSpace(value)
	var tokens []rawTimestampToken

	for i := 0; i < len(value); {
		remaining := value[i:]

		if loc := reTimezone.FindStringIndex(remaining); loc != nil && loc[0] == 0 && shouldTreatAsTimezone(tokens) {
			tokens = append(tokens, rawTimestampToken{Kind: "timezone", Value: remaining[:loc[1]]})
			i += loc[1]
			continue
		}

		if loc := reMillisecond.FindStringIndex(remaining); loc != nil && loc[0] == 0 {
			tokens = append(tokens, rawTimestampToken{Kind: "fraction", Value: remaining[:loc[1]]})
			i += loc[1]
			continue
		}

		if loc := reNumber.FindStringIndex(remaining); loc != nil && loc[0] == 0 {
			tokens = append(tokens, rawTimestampToken{Kind: "number", Value: remaining[:loc[1]]})
			i += loc[1]
			continue
		}

		if loc := reTimestampWord.FindStringIndex(remaining); loc != nil && loc[0] == 0 {
			tokens = append(tokens, rawTimestampToken{Kind: "word", Value: remaining[:loc[1]]})
			i += loc[1]
			continue
		}

		tokens = append(tokens, rawTimestampToken{Kind: "literal", Value: string(value[i])})
		i++
	}

	return tokens
}

func shouldTreatAsTimezone(tokens []rawTimestampToken) bool {
	i := len(tokens) - 1
	for i >= 0 && tokens[i].Kind == "literal" && isWhitespaceLiteral(tokens[i].Value) {
		i--
	}

	if i < 0 {
		return false
	}

	if tokens[i].Kind == "fraction" {
		return true
	}

	return i >= 4 &&
		tokens[i].Kind == "number" &&
		tokens[i-1].Value == ":" &&
		tokens[i-2].Kind == "number" &&
		tokens[i-3].Value == ":" &&
		tokens[i-4].Kind == "number"
}

func findTimeStart(tokens []rawTimestampToken) int {
	for i := 0; i+4 < len(tokens); i++ {
		if !isValidTimeCandidate(tokens[i : i+5]) {
			continue
		}

		if _, ok := inferDatePartKinds(tokens[:i]); ok {
			return i
		}
	}

	return -1
}

func isValidTimeCandidate(tokens []rawTimestampToken) bool {
	if len(tokens) != 5 ||
		tokens[0].Kind != "number" ||
		tokens[1].Value != ":" ||
		tokens[2].Kind != "number" ||
		tokens[3].Value != ":" ||
		tokens[4].Kind != "number" {
		return false
	}

	return timestampNumberInRange(tokens[0].Value, 0, 23) &&
		timestampNumberInRange(tokens[2].Value, 0, 59) &&
		timestampNumberInRange(tokens[4].Value, 0, 59)
}

func timestampNumberInRange(value string, min int, max int) bool {
	if len(value) == 0 || len(value) > 2 {
		return false
	}

	number, err := strconv.Atoi(value)
	if err != nil || number < min || number > max {
		return false
	}

	return true
}

func inferDateOnlyParts(tokens []rawTimestampToken) ([]timestampPart, bool) {
	dateKinds, ok := inferDatePartKinds(tokens)
	if !ok || len(dateKinds) == 0 {
		return nil, false
	}

	parts := make([]timestampPart, 0, len(tokens))
	for i, token := range tokens {
		if kind, ok := dateKinds[i]; ok {
			parts = append(parts, timestampPart{Kind: kind, Value: token.Value})
			continue
		}

		parts = append(parts, rawTokenPart(token))
	}

	return parts, true
}

func inferDatePartKinds(tokens []rawTimestampToken) (map[int]string, bool) {
	var indexes []int
	for i, token := range tokens {
		if token.Kind == "number" || isMonthNameToken(token) {
			indexes = append(indexes, i)
		}
	}

	if len(indexes) == 0 {
		return map[int]string{}, true
	}

	kinds := make(map[int]string, len(indexes))

	if len(indexes) == 2 {
		first := tokens[indexes[0]]
		second := tokens[indexes[1]]

		switch {
		case isMonthNameToken(first) && second.Kind == "number":
			kinds[indexes[0]] = "month_name"
			kinds[indexes[1]] = "day_space"
			return kinds, true

		case first.Kind == "number" && second.Kind == "number":
			kinds[indexes[0]] = "month"
			kinds[indexes[1]] = "day"
			return kinds, true
		}
	}

	if len(indexes) != 3 {
		return nil, false
	}

	first := tokens[indexes[0]]
	second := tokens[indexes[1]]
	third := tokens[indexes[2]]

	switch {
	case isFourDigitYear(first) && second.Kind == "number" && third.Kind == "number":
		kinds[indexes[0]] = "year"
		kinds[indexes[1]] = "month"
		kinds[indexes[2]] = "day"
		return kinds, true

	case first.Kind == "number" && isMonthNameToken(second) && isFourDigitYear(third):
		kinds[indexes[0]] = "day"
		kinds[indexes[1]] = "month_name"
		kinds[indexes[2]] = "year"
		return kinds, true

	case isMonthNameToken(first) && second.Kind == "number" && isFourDigitYear(third):
		kinds[indexes[0]] = "month_name"
		kinds[indexes[1]] = "day"
		kinds[indexes[2]] = "year"
		return kinds, true
	}

	return nil, false
}

func rawTokenPart(token rawTimestampToken) timestampPart {
	switch token.Kind {
	case "fraction":
		return timestampPart{Kind: "fraction", Value: token.Value}
	case "timezone":
		return timestampPart{Kind: "timezone", Value: token.Value}
	default:
		return timestampPart{Kind: "literal", Value: token.Value}
	}
}

func isMonthNameToken(token rawTimestampToken) bool {
	return token.Kind == "word" && len(token.Value) >= 3
}

func isFourDigitYear(token rawTimestampToken) bool {
	return token.Kind == "number" && len(token.Value) == 4
}

func renderTimeFormat(parts []timestampPart) string {
	var builder strings.Builder
	lastWasSpace := false

	for _, part := range parts {
		if part.Kind == "literal" && isWhitespaceLiteral(part.Value) {
			if !lastWasSpace {
				builder.WriteString(" ")
				lastWasSpace = true
			}
			continue
		}

		lastWasSpace = false

		switch part.Kind {
		case "year":
			builder.WriteString("%Y")
		case "month":
			builder.WriteString("%m")
		case "month_name":
			builder.WriteString("%b")
		case "day":
			builder.WriteString("%d")
		case "day_space":
			builder.WriteString("%e")
		case "hour":
			builder.WriteString("%H")
		case "minute":
			builder.WriteString("%M")
		case "second":
			builder.WriteString("%S")
		case "fraction":
			if strings.HasPrefix(part.Value, ",") {
				builder.WriteString(",%L")
			} else {
				builder.WriteString(".%L")
			}
		case "timezone":
			builder.WriteString("%z")
		default:
			builder.WriteString(part.Value)
		}
	}

	return builder.String()
}

func isWhitespaceLiteral(value string) bool {
	return value == " " || value == "\t"
}

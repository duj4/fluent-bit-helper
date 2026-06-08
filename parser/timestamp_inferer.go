package parser

import (
	"errors"
	"regexp"
	"strings"
)

var (
	reMonthNameDateTime = regexp.MustCompile(`^\w+\s+\d+\s+\d{2}:\d{2}:\d{2}(?:[,.]\d+)?(?:\s*(?:Z|z|[+-]\d{2}:?\d{2}))?$`)
	reNumericDateTime   = regexp.MustCompile(`^\d{1,2}([-\/])\d{1,2}\s+\d{2}:\d{2}:\d{2}(?:[,.]\d+)?(?:\s*(?:Z|z|[+-]\d{2}:?\d{2}))?$`)
	reTimeOnly          = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}(?:[,.]\d+)?(?:\s*(?:Z|z|[+-]\d{2}:?\d{2}))?$`)
	reTimestampFraction = regexp.MustCompile(`[,.]\d+`)
	reTimestampTimezone = regexp.MustCompile(`(?:Z|z|[+-]\d{2}:?\d{2})$`)
	reSpacedTimezone    = regexp.MustCompile(`\s+(?:Z|z|[+-]\d{2}:?\d{2})$`)
)

// InferTimeFormat converts timestamp value into Fluent-Bit time_format
func InferTimeFormat(value string) (string, error) {
	if format, ok := inferTimeFormatFromShape(value); ok {
		return format, nil
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

func inferTimeFormatFromShape(value string) (string, bool) {
	value = strings.TrimSpace(value)

	switch {
	case reMonthNameDateTime.MatchString(value):
		return withTimestampSuffix(value, "%b %e %H:%M:%S"), true

	case reNumericDateTime.MatchString(value):
		separator := "-"
		if matches := reNumericDateTime.FindStringSubmatch(value); len(matches) > 1 {
			separator = matches[1]
		}

		return withTimestampSuffix(value, "%m"+separator+"%d %H:%M:%S"), true

	case reTimeOnly.MatchString(value):
		return withTimestampSuffix(value, "%H:%M:%S"), true

	default:
		return "", false
	}
}

func withTimestampSuffix(value string, format string) string {
	if match := reTimestampFraction.FindString(value); match != "" {
		if strings.HasPrefix(match, ",") {
			format += ",%L"
		} else {
			format += ".%L"
		}
	}

	if reTimestampTimezone.MatchString(value) {
		if reSpacedTimezone.MatchString(value) {
			format += " %z"
		} else {
			format += "%z"
		}
	}

	return format
}

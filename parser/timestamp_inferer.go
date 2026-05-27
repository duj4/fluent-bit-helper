package parser

import (
	"errors"
	"strings"
)

// InferTimeFormat converts timestamp value into Fluent-Bit time_format
func InferTimeFormat(value string) (string, error) {
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

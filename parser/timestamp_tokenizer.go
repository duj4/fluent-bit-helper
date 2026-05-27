package parser

import "regexp"

var (
	// Support timezone format like: Z, z, +08:00, -06:00, +0800, -0600
	reTimezone    = regexp.MustCompile(`^(Z|z|[+-]\d{2}:?\d{2})`)
	reMillisecond = regexp.MustCompile(`^[\.,]\d+`)
	reNumber      = regexp.MustCompile(`^\d+`)
)

func Tokenize(input string) []Token {
	var tokens []Token

	for i := 0; i < len(input); {
		remaining := input[i:]

		// timezone
		if loc := reTimezone.FindStringIndex(remaining); loc != nil && loc[0] == 0 {
			tokens = append(tokens, Token{
				Type:  "timezone",
				Value: remaining[:loc[1]],
			})
			i += loc[1]
			continue
		}

		// millisecond
		if loc := reMillisecond.FindStringIndex(remaining); loc != nil && loc[0] == 0 {
			tokens = append(tokens, Token{
				Type:  "millisecond",
				Value: remaining[:loc[1]],
			})
			i += loc[1]
			continue
		}

		// number
		if loc := reNumber.FindStringIndex(remaining); loc != nil && loc[0] == 0 {
			tokens = append(tokens, Token{
				Type:  "number",
				Value: remaining[:loc[1]],
			})
			i += loc[1]
			continue
		}

		// literal
		tokens = append(tokens, Token{
			Type:  "literal",
			Value: string(input[i]),
		})

		i++
	}

	return tokens
}

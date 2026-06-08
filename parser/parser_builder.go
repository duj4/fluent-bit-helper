package parser

import "fmt"

// BuildParser builds Fluent-Bit parser config from user input
func BuildParser(userInput ParserInput) (Parser, error) {
	timeFormat, err := InferTimeFormat(userInput.TimeValue)
	if err != nil {
		return Parser{}, err
	}

	p := Parser{
		Name:       userInput.ParserName,
		Format:     userInput.Format,
		TimeKey:    userInput.TimeKey,
		TimeFormat: timeFormat,
		TimeKeep:   "off",
	}

	// only non-GMT timezone needs time_offset, by far it supports GMT+8 only
	switch userInput.Timezone {
	case "GMT":
		// do nothing
	case "GMT+8":
		p.TimeOffset = "+0800"
	}

	switch userInput.Format {
	case "json":
		return p, nil
	case "regex":
		regex, err := InferRegex(userInput.Snippet, userInput.RegexFields, userInput.EnableMultiline)
		if err != nil {
			return Parser{}, err
		}
		p.Regex = regex

		return p, nil
	default:
		return Parser{}, fmt.Errorf("unsupported parser format: %s", userInput.Format)
	}
}

// BuildFilter builds Fluent-Bit parser filter config from user input
func BuildFilter(userInput ParserInput) Filter {
	return Filter{
		Name:        "parser",
		Match:       "${TAG}",
		KeyName:     userInput.KeyName,
		Parser:      userInput.ParserName,
		ReserveData: "on",
	}
}

package parser

// BuildParser builds Fluent-Bit parser config from user input
func BuildParser(input ParseInput) (Parser, error) {
	timeFormat, err := InferTimeFormat(input.TimeValue)
	if err != nil {
		return Parser{}, err
	}

	p := Parser{
		Name:       input.ParserName,
		Format:     input.Format,
		TimeKey:    input.TimeKey,
		TimeFormat: timeFormat,
		TimeKeep:   "off",
	}

	// only non-GMT timezone needs time_offset, by far it supports GMT+8 only
	switch input.Timezone {
	case "GMT":
		// do nothing
	case "GMT+8":
		p.TimeOffset = "+0800"
	}

	return p, nil
}

// BuildFilter builds Fluent-Bit parser filter config from user input
func BuildFilter(input ParseInput) Filter {
	return Filter{
		Name:        "parser",
		Match:       input.Match,
		KeyName:     input.KeyName,
		Parser:      input.ParserName,
		ReserveData: "on",
	}
}

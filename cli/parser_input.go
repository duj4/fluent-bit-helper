package cli

import (
	"flbctl/parser"
	"fmt"
	"strings"

	"charm.land/huh/v2"
)

// CollectInput collects parser input from user
func CollectParserInput() (parser.ParserInput, error) {
	input := parser.ParserInput{}

	if err := collectFormat(&input); err != nil {
		return input, err
	}

	switch input.Format {
	case "json":
		if err := collectJSONParserInput(&input); err != nil {
			return input, err
		}

	case "regex":
		if err := collectRegexInput(&input); err != nil {
			return input, err
		}

		if err := collectRegexTimestampInput(&input); err != nil {
			return input, err
		}

		if err := collectMultlineInput(&input); err != nil {
			return input, err
		}
	}

	if err := collectCommonParserInput(&input); err != nil {
		return input, err
	}

	return input, nil
}

// collectFormat collects parser format from user
func collectFormat(input *parser.ParserInput) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Log format").
				Description("JSON parses structured logs, regex parses plain text logs by generated regular expression").
				Options(
					huh.NewOption("json", "json"),
					huh.NewOption("regex", "regex"),
				).
				Value(&input.Format),
		),
	)

	return form.Run()
}

// collectJSONParserInput collects JSON parser input from user
func collectJSONParserInput(input *parser.ParserInput) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Original timestamp key").
				Description("Example: timestamp, ts").
				Value(&input.TimeKey).
				Validate(required),

			huh.NewInput().
				Title("Original timestamp value").
				Description("Example: 2026-01-02T15:04:05.678+0800, 2026-01-02T15:04:05z").
				Value(&input.TimeValue).
				Validate(required),
		),
	)

	return form.Run()
}

// collectCommonParserInput collects common parser and filter input from user
func collectCommonParserInput(input *parser.ParserInput) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Original log timezone").
				Description("Select the timezone used in the original log").
				Options(
					huh.NewOption("GMT", "GMT"),
					huh.NewOption("GMT+8", "GMT+8"),
				).
				Value(&input.Timezone),

			huh.NewInput().
				Title("Parser name").
				Description("Example: parser_app, app_parser").
				Value(&input.ParserName).
				Validate(required),

			huh.NewInput().
				Title("Filter match").
				Description("Match pattern for parser filter, for example: app, app.*, ${TAG}").
				Value(&input.Match).
				Validate(required),

			huh.NewInput().
				Title("Filter key_name").
				Description("The record key to parse, for tail input this is usually log").
				Value(&input.KeyName).
				Validate(required),
		),
	)

	return form.Run()
}

// collectRegexInput collects regex parser input from user
func collectRegexInput(input *parser.ParserInput) error {
	regexForm := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("Log snippet").
				Description("Paste one single-line log example").
				Value(&input.Snippet).
				Validate(required),
		),
	)

	if err := regexForm.Run(); err != nil {
		return err
	}

	for {
		var key string
		var value string
		var addAnother bool

		fieldForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Field key").
					Description("Example: timestamp, level, message, logger").
					Value(&key).
					Validate(required),

				huh.NewInput().
					Title("Field value").
					Description("Exact value as it appears in the log snippet").
					Value(&value).
					Validate(required),

				huh.NewConfirm().
					Title("Add another field?").
					Value(&addAnother),
			),
		)

		if err := fieldForm.Run(); err != nil {
			return err
		}

		input.RegexFields = append(input.RegexFields, parser.RegexField{
			Key:   strings.TrimSpace(key),
			Value: value,
		})

		if !addAnother {
			break
		}
	}

	return nil
}

// collectRegexTimestampInput collects timestamp field from regex fields
func collectRegexTimestampInput(input *parser.ParserInput) error {
	if len(input.RegexFields) == 0 {
		return fmt.Errorf("no regex fields configured")
	}

	for {
		var selected string

		options := make([]huh.Option[string], 0, len(input.RegexFields))

		for _, field := range input.RegexFields {
			label := fmt.Sprintf("%s = %s", field.Key, field.Value)
			options = append(options, huh.NewOption(label, field.Key))
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Timestamp field").
					Description("Select which extracted field should be used as Fluent-Bit time_key").
					Options(options...).
					Value(&selected),
			),
		)

		if err := form.Run(); err != nil {
			return err
		}

		for _, field := range input.RegexFields {
			if field.Key == selected {
				input.TimeKey = field.Key
				input.TimeValue = field.Value
				return nil
			}
		}

		fmt.Printf("⚠️ Selected timestamp field not found: %s, please select again\n", selected)
	}
}

// collectMultlineInput collects multiline parser input from user
func collectMultlineInput(input *parser.ParserInput) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable multiline_parser?").
				Description("Use timestamp-start rule to merge stack trace or multiline logs").
				Value(&input.EnableMultiline),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if !input.EnableMultiline {
		return nil
	}

	nameForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Multiline parser name").
				Description("Example: multi_app, multiline_app").
				Value(&input.MultilineParserName).
				Validate(required),
		),
	)

	return nameForm.Run()
}

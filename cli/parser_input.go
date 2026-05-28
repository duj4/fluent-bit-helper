package cli

import (
	"flbctl/parser"
	"fmt"
	"strings"

	"charm.land/huh/v2"
)

// CollectInput collects parser input from user
func CollectParserInput(inputTag string) (parser.ParserInput, error) {
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

	if err := collectCommonParserInput(&input, inputTag); err != nil {
		return input, err
	}

	return input, nil
}

// collectFormat collects parser format from user
func collectFormat(input *parser.ParserInput) error {
	form := huh.NewForm(
		huh.NewGroup(
			sectionNote("Parser - Select log format for parsing"),

			huh.NewSelect[string]().
				Title("Log content format").
				Options(
					huh.NewOption("json - log line is a JSON object", "json"),
					huh.NewOption("plain text - generate regex from sample log", "regex"),
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
			sectionNote("Parser - JSON log timestamp settings"),

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
func collectCommonParserInput(input *parser.ParserInput, inputTag string) error {
	form := huh.NewForm(
		huh.NewGroup(
			sectionNote("Parser - Common parser settings"),

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

			sectionNote("Filter - Parser filter settings"),

			huh.NewSelect[string]().
				Title("Apply parser to which tag?").
				Options(
					huh.NewOption(inputTag, inputTag),
				).
				Value(&input.Match),

			huh.NewInput().
				Title("Raw log field to parse").
				Description("Usually: log").
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
			sectionNote("Parser - Mark fields from a sample log"),

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

		fieldIndex := len(input.RegexFields) + 1

		fieldForm := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title(fmt.Sprintf("Log snippet - field %d", fieldIndex)).
					Description(formatSnippetNote(input.Snippet, input.RegexFields)),

				huh.NewInput().
					Title("Field name").
					Description("Example: timestamp, level, message, logger").
					Value(&key).
					Validate(validateRegexFieldKey(input.RegexFields)),

				huh.NewInput().
					Title("Exact value from sample log").
					Description("Copy the exact value for this field without surrounding separators like spaces, but preserve the ones inside the value if any").
					Value(&value).
					Validate(validateRegexFieldValue(input.Snippet, &key)),

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

// formatSnippetNote formats log snippet and added fields for display
func formatSnippetNote(snippet string, fields []parser.RegexField) string {
	return fmt.Sprintf(
		"Raw log snippet:\n%s\n\n\n%s",
		escapeNoteText(snippet),
		formatRegexFields(fields),
	)
}

// escapeNoteText escapes characters that may be interpreted by huh note rendering
func escapeNoteText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "_", "\\_")
	s = strings.ReplaceAll(s, "*", "\\*")
	s = strings.ReplaceAll(s, "`", "\\`")

	return s
}

// formatRegexFields formats already added regex fields
func formatRegexFields(fields []parser.RegexField) string {
	if len(fields) == 0 {
		return "Added fields: none"
	}

	var builder strings.Builder

	builder.WriteString("Added fields: \n")

	for _, field := range fields {
		builder.WriteString("- ")
		builder.WriteString(escapeNoteText(field.Key))
		builder.WriteString(" = ")
		builder.WriteString(escapeNoteText(field.Value))
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String())
}

// validateRegexFieldValue validates that field value exists in log snippet
func validateRegexFieldValue(snippet string, key *string) func(string) error {
	return func(value string) error {
		value = strings.TrimSpace(value)

		if value == "" {
			return fmt.Errorf("required")
		}

		fieldKey := strings.TrimSpace(*key)
		if fieldKey == "" {
			fieldKey = "field"
		}

		if !strings.Contains(snippet, value) {
			return fmt.Errorf("value for field %q not found in log snippet", fieldKey)
		}

		if strings.Count(snippet, value) > 1 {
			return fmt.Errorf("value for field %q appears multiple times in log snippet", fieldKey)
		}

		return nil
	}
}

// validateRegexFieldKey validates regex field key
func validateRegexFieldKey(existingFields []parser.RegexField) func(string) error {
	return func(key string) error {
		key = strings.TrimSpace(key)

		if key == "" {
			return fmt.Errorf("required")
		}

		for _, field := range existingFields {
			if field.Key == key {
				return fmt.Errorf("field key %q already exists", key)
			}
		}

		return nil
	}
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
			sectionNote("Multiline Parser - Optional multiline parser settings"),

			huh.NewConfirm().
				Title("Merge multiline log events?").
				Description("Enable this for stack traces or logs where one event spans multiple lines").
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

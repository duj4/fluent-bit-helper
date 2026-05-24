package cli

import (
	"flbctl/parser"

	"charm.land/huh/v2"
)

// CollectInput collects parser input from user
func CollectInput() (parser.ParseInput, error) {
	input := parser.ParseInput{}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Log format").
				Description("Only JSON format is supported at this time").
				Options(
					huh.NewOption("json", "json"),
				).
				Value(&input.Format),

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
				Description("Value of tag defined in input plugin").
				Value(&input.Match).
				Validate(required),

			huh.NewInput().
				Title("Filter key_name").
				Description("Example: log").
				Value(&input.KeyName).
				Validate(required),
		),
	)

	if err := form.Run(); err != nil {
		return input, err
	}

	return input, nil
}

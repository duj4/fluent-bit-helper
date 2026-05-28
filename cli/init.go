package cli

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"flbctl/config"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
)

// output YAML file path.
var outputFile string

// initCmd launches the interactive wizard.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactively generate a Fluent-Bit YAML config",
	RunE: func(cmd *cobra.Command, args []string) error {
		userInput, err := collectUserInput()
		if err != nil {
			return err
		}

		cfg, err := config.Build(userInput)
		if err != nil {
			return err
		}

		data, err := config.ToYAML(cfg)
		if err != nil {
			return err
		}

		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			return err
		}

		fmt.Printf("✅ %s generated\n", outputFile)

		if userInput.FluentBitLogFile != "" {
			fmt.Println("⚠️ Notes: Fluent-Bit does not rotate its own log file, please configure logrotate separately.")
		}

		return nil
	},
}

// init registers CLI flags.
func init() {
	initCmd.Flags().StringVarP(&outputFile, "output", "o", "fluent-bit.yaml", "output YAML file")
}

// collectUserInput interactively collect user input
func collectUserInput() (config.UserInput, error) {
	var userInput config.UserInput
	var logPathsText string

	if err := collectMode(&userInput); err != nil {
		return userInput, err
	}

	// quick-test mode uses relaxed defaults and skips production-only fields.
	if userInput.Mode == "quick-test" {
		userInput.FluentBitLogFile = ""
		userInput.DBPath = ""
		userInput.ReadFromHead = true
	}

	if err := collectServiceInput(&userInput); err != nil {
		return userInput, err
	}

	if err := collectTailInput(&userInput, &logPathsText); err != nil {
		return userInput, err
	}

	userInput.LogPaths = normalizePaths(logPathsText)

	if len(userInput.LogPaths) == 0 {
		return userInput, errors.New("at least one log path is required")
	}

	if err := collectParserChoice(&userInput); err != nil {
		return userInput, err
	}

	if userInput.EnableParser {
		parserInput, err := CollectParserInput(userInput.Tag)
		if err != nil {
			return userInput, err
		}

		userInput.ParserInput = parserInput
	}

	if err := collectOutputInput(&userInput); err != nil {
		return userInput, err
	}

	if userInput.Output == "loki" {
		if err := collectLokiLabelInput(&userInput); err != nil {
			return userInput, err
		}

		if err := collectLokiOutputInput(&userInput); err != nil {
			return userInput, err
		}
	}

	return userInput, nil
}

// collectMode collects usage mode from user
func collectMode(userInput *config.UserInput) error {
	form := huh.NewForm(
		huh.NewGroup(
			sectionNote("Mode - Select how this config will be used"),

			huh.NewSelect[string]().
				Title("Usage mode").
				Options(
					huh.NewOption("quick test - relaxed settings for local validation", "quick-test"),
					huh.NewOption("production - safer defaults for real log collection", "production"),
				).
				Value(&userInput.Mode),
		),
	)

	return form.Run()
}

// collectServiceInput collects Fluent-Bit service related input.
func collectServiceInput(userInput *config.UserInput) error {
	fields := []huh.Field{
		sectionNote("Service - Configure Fluent-Bit's own runtime settings"),
	}

	if userInput.Mode == "production" {
		fields = append(
			fields,
			huh.NewInput().
				Title("Fluent-Bit's own log file").
				Description("Example: /d/d1/app/fluent-bit/log/fluent-bit.log\nRecommended for production and configure logrotate separately").
				Value(&userInput.FluentBitLogFile),
		)
	}

	fields = append(
		fields,
		huh.NewInput().
			Title("Filesystem buffering directory").
			Description("Example: /d/d1/app/fluent-bit/data/storage/  Must already exist.").
			Value(&userInput.StoragePath).
			Validate(required),
	)

	form := huh.NewForm(
		huh.NewGroup(fields...),
	)

	return form.Run()
}

// collectTailInput collects tail input related input.
func collectTailInput(userInput *config.UserInput, logPathsText *string) error {
	fields := []huh.Field{
		sectionNote("Input - Tail - Configure log files monitored by Fluent-Bit tail input"),

		huh.NewInput().
			Title("Fluent-Bit monitored log path(s)").
			Description("Comma-separated paths, example: /d/d1/app/log/app.log,/d/d1/app/log/access.log").
			Value(logPathsText).
			Validate(required),
	}

	if userInput.Mode == "production" {
		fields = append(
			fields,
			huh.NewInput().
				Title("Tail offset database file").
				Description("Example: /d/d1/app/fluent-bit/data/database/\nRecommended for production, parent folder must already exist").
				Value(&userInput.DBPath),

			huh.NewConfirm().
				Title("Read existing files from head?").
				Description("Strongly recommended to keep disabled in production to avoid backfilling old logs").
				Value(&userInput.ReadFromHead),
		)
	} else {
		userInput.DBPath = ""
		userInput.ReadFromHead = true
	}

	fields = append(
		fields,
		huh.NewInput().
			Title("Tag for pipeline routing").
			Description("Example: app").
			Value(&userInput.Tag).
			Validate(required),
	)

	form := huh.NewForm(
		huh.NewGroup(fields...),
	)

	return form.Run()
}

// collectParserChoice collects whether parser/filter configuration is needed.
func collectParserChoice(userInput *config.UserInput) error {
	form := huh.NewForm(
		huh.NewGroup(
			sectionNote("Parser and filter - Optional log parsing settings"),

			huh.NewConfirm().
				Title("Parse log content into fields?").
				Description("Enable this if you want to extract fields such as time, level, user or message from each log line").
				Value(&userInput.EnableParser),
		),
	)

	return form.Run()
}

// collectOutputInput collects output destination.
func collectOutputInput(userInput *config.UserInput) error {
	form := huh.NewForm(
		huh.NewGroup(
			sectionNote("Output - Configure where Fluent-Bit sends logs"),

			huh.NewSelect[string]().
				Title("Output destination").
				Options(
					huh.NewOption("stdout", "stdout"),
					huh.NewOption("loki", "loki"),
				).
				Value(&userInput.Output),
		),
	)

	return form.Run()
}

// collectLokiLabelInput collects optional Loki labels from parser fields.
func collectLokiLabelInput(userInput *config.UserInput) error {
	if !userInput.EnableParser {
		return nil
	}

	switch userInput.ParserInput.Format {
	case "json":
		return collectJSONLokiLabelNotice()

	case "regex":
		return collectRegexLokiLabelInput(userInput)

	default:
		return nil
	}
}

// collectJSONLokiLabelNotice shows Loki label notice for JSON parser.
func collectJSONLokiLabelNotice() error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("JSON parser detected").
				Description("Default labels filename and instance will be used. Add JSON fields as labels manually after generation if needed"),
		),
	)

	return form.Run()
}

// collectRegexLokiLabelInput collects optional Loki labels from regex parser fields.
func collectRegexLokiLabelInput(userInput *config.UserInput) error {
	options := make([]huh.Option[string], 0, len(userInput.ParserInput.RegexFields))

	for _, field := range userInput.ParserInput.RegexFields {
		key := strings.TrimSpace(field.Key)
		if key == "" {
			continue
		}

		options = append(options, huh.NewOption(key, key))
	}

	if len(options) == 0 {
		return nil
	}

	for {
		options := buildAvailableLokiLabelFieldOptions(userInput)
		if len(options) == 0 {
			return nil
		}

		var addLabel bool

		addForm := huh.NewForm(
			huh.NewGroup(
				sectionNote("Output - Loki labels"),

				huh.NewNote().
					Title("Current Loki labels").
					Description(formatLokiLabels(userInput.LokiLabels)),

				huh.NewConfirm().
					Title("Add additional Loki label?").
					Description("filename and instance are always included. Choose stable fields like level/service/component. Avoid unique fields like timestamp/message/trace_id/request_id/user_id").
					Value(&addLabel),
			),
		)

		if err := addForm.Run(); err != nil {
			return err
		}

		if !addLabel {
			break
		}

		var field string
		var labelName string

		labelForm := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("Current Loki labels").
					Description(formatLokiLabels(userInput.LokiLabels)),

				huh.NewSelect[string]().
					Title("Parser field").
					Description("Select the extracted field used as Loki label value").
					Options(options...).
					Value(&field),

				huh.NewInput().
					Title("Loki label name").
					Description("Example: level, service, component, username").
					Value(&labelName).
					Validate(validateLokiLabelName(userInput.LokiLabels)),
			),
		)

		if err := labelForm.Run(); err != nil {
			return err
		}

		userInput.LokiLabels = append(userInput.LokiLabels, config.LokiLabel{
			Name:  strings.TrimSpace(labelName),
			Field: strings.TrimSpace(field),
		})
	}

	return nil
}

func buildAvailableLokiLabelFieldOptions(userInput *config.UserInput) []huh.Option[string] {
	usedFields := make(map[string]struct{})

	for _, label := range userInput.LokiLabels {
		field := strings.TrimSpace(label.Field)
		if field != "" {
			usedFields[field] = struct{}{}
		}
	}

	options := make([]huh.Option[string], 0, len(userInput.ParserInput.RegexFields))

	for _, field := range userInput.ParserInput.RegexFields {
		key := strings.TrimSpace(field.Key)
		if key == "" {
			continue
		}

		if _, used := usedFields[key]; used {
			continue
		}

		options = append(options, huh.NewOption(key, key))
	}

	return options
}

// formatLokiLabels formats default and already added Loki labels.
func formatLokiLabels(labels []config.LokiLabel) string {
	var builder strings.Builder

	builder.WriteString("Default Loki labels:\n")
	builder.WriteString("- filename = $filename\n")
	builder.WriteString("- instance = ${HOSTNAME}\n")

	if len(labels) == 0 {
		builder.WriteString("\nAdditional Loki labels: none")
		return builder.String()
	}

	builder.WriteString("\nAdditional Loki labels:\n")

	for _, label := range labels {
		name := strings.TrimSpace(label.Name)
		field := strings.TrimSpace(label.Field)

		if name == "" || field == "" {
			continue
		}

		builder.WriteString("- ")
		builder.WriteString(name)
		builder.WriteString(" = $")
		builder.WriteString(field)
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String())
}

// collectLokiOutputInput collects Loki output related input
func collectLokiOutputInput(userInput *config.UserInput) error {
	form := huh.NewForm(
		huh.NewGroup(
			sectionNote("Output - Loki - Configure Loki endpoint, tenant and TLS settings"),

			huh.NewInput().
				Title("Loki endpoint").
				Description("Refer https://docs.srv.ms.com.cn/cod-esm-o11y/observability-docs/LaaS/reference/instance.html for the endpoint").
				Value(&userInput.LokiHost).
				Validate(required),

			huh.NewInput().
				Title("TenantID").
				Description("Check your LaaS onboarding ticket for the TenantID").
				Value(&userInput.TenantID).
				Validate(required),

			huh.NewInput().
				Title("TLS CA file").
				Description("Absolute path to the CA file").
				Value(&userInput.TLSCAFile).
				Validate(required),

			huh.NewInput().
				Title("TLS Cert file").
				Description("Absolute path to the TLS cert file of systemid with RW permission upon the tenantID").
				Value(&userInput.TLSCrtFile).
				Validate(required),

			huh.NewInput().
				Title("TLS Key file").
				Description("Absolute path to the TLS key file of the systemid with RW permission upon the tenantID").
				Value(&userInput.TLSKeyFile).
				Validate(required),
		),
	)

	return form.Run()
}

// sectionNote shows current Fluent-Bit config section
func sectionNote(title string) huh.Field {
	line := strings.Repeat("=", len(title))
	return huh.NewNote().Title(fmt.Sprintf("%s\n%s\n%s", line, title, line))
}

// required makes sure the field is filled
func required(v string) error {
	if strings.TrimSpace(v) == "" {
		return errors.New("required")
	}
	return nil
}

// normalizePaths transforms the log path into string slice.
func normalizePaths(raw string) []string {
	parts := strings.Split(raw, ",")

	paths := make([]string, 0, len(parts))

	for _, part := range parts {
		path := strings.TrimSpace(part)

		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths
}

// validateLokiLabelName validates the label name is allowed
func validateLokiLabelName(existingLabels []config.LokiLabel) func(string) error {
	return func(v string) error {
		v = strings.TrimSpace(v)

		if v == "" {
			return errors.New("required")
		}

		matched, err := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, v)
		if err != nil {
			return err
		}

		if !matched {
			return errors.New("label name must start with a letter or underscore and contain only letters, numbers and underscores")
		}

		for _, label := range existingLabels {
			if label.Name == v {
				return fmt.Errorf("label name %q already exists", v)
			}
		}

		return nil
	}
}

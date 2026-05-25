package cli

import (
	"errors"
	"fmt"
	"os"
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

		fmt.Println("⚠️ Notes: Fluent-Bit does not rotate its own log file, please configure logrotate separately.")

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

	// basic form
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Fluent-Bit's own log file").
				Description("Example: /d/d1/app/fluent-bit/log/fluent-bit.log").
				Value(&userInput.FluentBitLogFile).
				Validate(required),

			huh.NewInput().
				Title("Diretory path for filesystem buffering").
				Description("Example: /d/d1/app/fluent-bit/data/storage/").
				Value(&userInput.StoragePath).
				Validate(required),

			huh.NewText().
				Title("Fluent-Bit monitored log path(s)").
				Description("One path per line, example:\n/d/d1/app/log/app.log\n/d/d1/app/log/access.log").
				Value(&logPathsText).
				Validate(required),

			huh.NewInput().
				Title("Absolute path of database file to keep track of monitored file(s) and offsets.").
				Description("Example: /d/d1/app/fluent-bit/data/database/app.db, parent folder must exist in prior").
				Value(&userInput.DBPath).
				Validate(required),

			huh.NewInput().
				Title("Tag for pipeline routing").
				Description("Example: app").
				Value(&userInput.Tag).
				Validate(required),

			huh.NewConfirm().
				Title("Enable parser/filter configuration?").
				Description("Generate parser and parser filter sections").
				Value(&userInput.EnableParser),

			huh.NewSelect[string]().
				Title("Output destination").
				Options(
					huh.NewOption("stdout", "stdout"),
					huh.NewOption("loki", "loki"),
				).
				Value(&userInput.Output),
		),
	)

	if err := form.Run(); err != nil {
		return userInput, err
	}

	// collect parser input
	if userInput.EnableParser {
		parserInput, err := CollectParserInput()
		if err != nil {
			return userInput, err
		}

		userInput.ParserInput = parserInput
	}

	// if loki output is selected
	if userInput.Output == "loki" {

		lokiFields := []huh.Field{
			huh.NewInput().
				Title("Loki endpoint").
				Description("Refer https://docs.srv.ms.com.cn/cod-esm-o11y/observability-docs/LaaS/reference/instance.html for the endpoint.").
				Value(&userInput.LokiHost).
				Validate(required),

			huh.NewInput().
				Title("TenantID").
				Description("Check your LaaS onboardig ticket for the TenantID.").
				Value(&userInput.TenantID).
				Validate(required),

			huh.NewInput().
				Title("TLS CA file").
				Description("Absolute path to the ca file.").
				Value(&userInput.TLSCAFile).
				Validate(required),

			huh.NewInput().
				Title("TLS Cert file").
				Description("Absolute path to the TLS cert file of systemid with RW permission upon the tenantID.").
				Value(&userInput.TLSCrtFile).
				Validate(required),

			huh.NewInput().
				Title("TLS Key file").
				Description("Absolute path to the TLS key file of the systemid with RW permission upon the tenantID.").
				Value(&userInput.TLSKeyFile).
				Validate(required),
		}

		lokiForm := huh.NewForm(
			huh.NewGroup(
				lokiFields...,
			),
		)

		if err := lokiForm.Run(); err != nil {
			return userInput, err
		}
	}

	userInput.LogPaths = normalizePaths(logPathsText)

	if len(userInput.LogPaths) == 0 {
		return userInput, errors.New("at least one log path is requred")
	}

	return userInput, nil
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
	lines := strings.Split(raw, "\n")

	paths := make([]string, 0, len(lines))

	for _, line := range lines {
		path := strings.TrimSpace(line)

		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths
}

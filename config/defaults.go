package config

import "strings"

// ApplyModeDefaults applies mode-specific defaults before config generation.
func ApplyModeDefaults(input *UserInput) {
	if strings.TrimSpace(input.Mode) == "" {
		input.Mode = "production"
	}

	if input.Mode == "quick-test" {
		input.FluentBitLogFile = ""
		input.StoragePath = ""
		input.DBPath = ""
		input.ReadFromHead = true
		return
	}

	if input.Mode == "production" {
		input.ReadFromHead = false
	}
}

// BuildWarnings returns user-facing warnings for generated config.
func BuildWarnings(input UserInput) []string {
	var warnings []string

	if input.Mode == "quick-test" {
		warnings = append(
			warnings,
			"Mode: quick-test enables read_from_head and does not configure filesystem buffering and tail offset database",
		)
	}

	if input.Mode == "production" {
		if strings.TrimSpace(input.FluentBitLogFile) == "" {
			warnings = append(warnings, "Service: Fluent-Bit's own log file is not configured")
		}

		if strings.TrimSpace(input.StoragePath) == "" {
			warnings = append(warnings, "Service: filesystem buffering directory is not configured")
		}

		if strings.TrimSpace(input.DBPath) == "" {
			warnings = append(warnings, "Tail input: tail offset database is not configured")
		}
	}

	if input.Output == "loki" && len(input.LokiLabels) > 0 {
		warnings = append(warnings, "Loki output: make sure selected labels are low-cardinality fields")
	}

	return warnings
}

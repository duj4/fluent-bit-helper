package config

import (
	"fmt"
	"strings"
)

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
			"Mode: quick test is intended for local validation and skips production buffering and offset tracking",
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

	if input.Output == "loki" {
		duplicatedNames := findDuplicatedLokiNames(input.LokiLabels, input.LokiStructuredMetadata)
		if len(duplicatedNames) > 0 {
			warnings = append(
				warnings,
				fmt.Sprintf(
					"Loki output: label name and structured metadata name must be unique: %s",
					strings.Join(duplicatedNames, ", "),
				),
			)
		}
	}

	return warnings
}

func findDuplicatedLokiNames(labels []LokiLabel, metadata []LokiStructuredMetadata) []string {
	labelNames := make(map[string]struct{})

	for _, label := range labels {
		name := strings.TrimSpace(label.Name)
		if name == "" {
			continue
		}

		labelNames[name] = struct{}{}
	}

	var duplicated []string
	seen := make(map[string]struct{})

	for _, item := range metadata {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}

		if _, ok := labelNames[name]; !ok {
			continue
		}

		if _, ok := seen[name]; ok {
			continue
		}

		duplicated = append(duplicated, name)
		seen[name] = struct{}{}
	}

	return duplicated
}

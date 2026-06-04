package config

import (
	"fmt"
	"strings"
)

// ApplyModeDefaults applies mode-specific defaults before config generation.
func ApplyModeDefaults(userInput *UserInput) {
	if strings.TrimSpace(userInput.Mode) == "" {
		userInput.Mode = "production"
	}

	if userInput.Mode == "quick-test" {
		userInput.FluentBitLogFile = ""
		userInput.StoragePath = ""
		userInput.DBPath = ""
		userInput.ReadFromHead = true
		return
	}

	if userInput.Mode == "production" {
		userInput.ReadFromHead = false
	}
}

// BuildWarnings returns user-facing warnings for generated config.
func BuildWarnings(userInput UserInput) []string {
	var warnings []string

	if userInput.Mode == "quick-test" {
		warnings = append(
			warnings,
			"Mode: quick test is intended for local validation and skips production buffering and offset tracking",
		)
	}

	if userInput.Mode == "production" {
		if strings.TrimSpace(userInput.FluentBitLogFile) == "" {
			warnings = append(warnings, "Service: Fluent-Bit's own log file is not configured")
		}

		if strings.TrimSpace(userInput.StoragePath) == "" {
			warnings = append(warnings, "Service: filesystem buffering directory is not configured")
		}

		if strings.TrimSpace(userInput.DBPath) == "" {
			warnings = append(warnings, "Tail userInput: tail offset database is not configured")
		}
	}

	if userInput.Output == "loki" && len(userInput.LokiLabels) > 0 {
		warnings = append(warnings, "Loki output: make sure selected labels are low-cardinality fields")
	}

	if userInput.Output == "loki" {
		duplicatedNames := findDuplicatedLokiNames(userInput.LokiLabels, userInput.LokiStructuredMetadata)
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

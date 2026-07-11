package config

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"fluent-bit-helper/parser"

	"gopkg.in/yaml.v3"
)

// Build converts user input into Fluent-Bit domain model.
func Build(userInput UserInput) (FluentBitConfig, error) {
	parserInputs := normalizeParserInputs(userInput)

	cfg := FluentBitConfig{
		Env: buildBaseEnv(userInput),

		Service: buildService(userInput),

		Pipeline: Pipeline{
			Inputs: []interface{}{
				buildTailInput(userInput),
			},
		},
	}

	// build parser/filter if enabled
	if userInput.EnableParser {
		multilineEnabled := hasMultilineParser(parserInputs)
		for _, parserInput := range parserInputs {
			parserInputForParser := parserInput
			if multilineEnabled {
				parserInputForParser.EnableMultiline = true
			}

			parserObj, err := parser.BuildParser(parserInputForParser)
			if err != nil {
				return FluentBitConfig{}, err
			}

			cfg.Parsers = append(cfg.Parsers, parserObj)

			filterObj := parser.BuildFilter(parserInput)
			cfg.Pipeline.Filters = append(cfg.Pipeline.Filters, filterObj)
		}

		if multilineEnabled {
			multilineParserObj, err := parser.BuildCombinedMultilineParser(parserInputs)
			if err != nil {
				return FluentBitConfig{}, err
			}

			cfg.MultilineParsers = []parser.MultilineParser{multilineParserObj}
		}
	}

	// output section
	if userInput.Output == "loki" {
		cfg.Env = buildLokiEnv(cfg.Env, userInput)

		cfg.Pipeline.Outputs = []interface{}{
			buildLokiOutput(userInput),
		}

		return cfg, nil
	}

	cfg.Pipeline.Outputs = []interface{}{
		buildStdoutOutput(),
	}

	return cfg, nil
}

func normalizeParserInputs(userInput UserInput) []parser.ParserInput {
	if !userInput.EnableParser {
		return nil
	}

	if len(userInput.ParserInputs) > 0 {
		var inputs []parser.ParserInput
		for _, parserInput := range userInput.ParserInputs {
			if strings.TrimSpace(parserInput.ParserName) == "" &&
				strings.TrimSpace(parserInput.TimeKey) == "" &&
				strings.TrimSpace(parserInput.TimeValue) == "" {
				continue
			}

			inputs = append(inputs, parserInput)
		}

		if len(inputs) > 0 {
			return inputs
		}
	}

	return []parser.ParserInput{userInput.ParserInput}
}

func hasMultilineParser(parserInputs []parser.ParserInput) bool {
	for _, parserInput := range parserInputs {
		if parserInput.EnableMultiline {
			return true
		}
	}

	return false
}

func buildBaseEnv(userInput UserInput) Env {
	env := Env{
		LogPath: strings.Join(userInput.LogPaths, ","),
		Tag:     userInput.Tag,
	}

	if userInput.Mode == "production" {
		env.FLBLogFile = userInput.FluentBitLogFile
		env.StoragePath = userInput.StoragePath
		env.DBPath = userInput.DBPath
	}

	return env
}

func buildLokiEnv(env Env, userInput UserInput) Env {
	env.LokiHost = userInput.LokiHost
	env.TenantID = userInput.TenantID
	env.TLSCAFile = userInput.TLSCAFile
	env.TLSCrtFile = userInput.TLSCrtFile
	env.TLSKeyFile = userInput.TLSKeyFile

	return env
}

func buildService(userInput UserInput) Service {
	if userInput.Mode == "quick-test" {
		return Service{
			LogLevel: "debug",
		}
	}

	service := Service{
		LogLevel: "error",

		SchedulerBase: 5,
		SchedulerCap:  300,

		StoragePath:                      "${STORAGE_PATH}",
		StorageSync:                      "full",
		StorageDeleteIrrecoverableChunks: "on",
	}

	if userInput.FluentBitLogFile != "" {
		service.LogFile = "${FLB_LOG_FILE}"
	}

	return service
}

func buildTailInput(userInput UserInput) TailInput {
	tailInput := TailInput{
		Name: "tail",

		Tag:     "${TAG}",
		Path:    "${LOG_PATH}",
		PathKey: "filename",

		SkipEmptyLines:  "on",
		RefreshInterval: 5,
		Threaded:        "on",
	}

	if userInput.Mode == "production" {
		tailInput.StorageType = "filesystem"
		if userInput.DBPath != "" {
			tailInput.DB = "${DB_PATH}"
		}
	}

	if userInput.ReadFromHead {
		tailInput.ReadFromHead = "on"
	}

	parserInputs := normalizeParserInputs(userInput)
	if hasMultilineParser(parserInputs) {
		tailInput.MultilineParser = multilineParserName(parserInputs)
	}

	return tailInput
}

func multilineParserName(parserInputs []parser.ParserInput) string {
	return parser.CombinedMultilineParserName(parserInputs)
}

func buildStdoutOutput() StdoutOutput {
	return StdoutOutput{
		Name:  "stdout",
		Match: "${TAG}",
	}
}

func buildLokiOutput(userInput UserInput) LokiOutput {
	output := LokiOutput{
		Name:  "loki",
		Match: "${TAG}",

		Host:               "${LOKI_HOST}",
		Port:               443,
		TenantID:           "${TENANT_ID}",
		Labels:             buildLokiLabels(userInput),
		StructuredMetadata: buildLokiStructuredMetadata(userInput),

		LineFormat: buildLokiLineFormat(userInput),

		TLS:        "on",
		TLSCAFile:  "${TLS_CA_FILE}",
		TLSCrtFile: "${TLS_CRT_FILE}",
		TLSKeyFile: "${TLS_KEY_FILE}",

		RetryLimit: "no_limits",

		NetDNSResolver: "ASYNC",

		Workers: 2,
	}

	if userInput.Mode == "production" {
		output.StorageTotalLimitSize = "1G"
		output.NetConnectTimeout = 310
		output.NetKeepaliveIdleTimeout = 300
	}

	return output
}

func buildLokiLabels(userInput UserInput) string {
	labels := []string{
		"filename=$filename",
		"instance=${HOSTNAME}",
	}

	for _, label := range userInput.LokiLabels {
		name := strings.TrimSpace(label.Name)
		field := strings.TrimSpace(label.Field)

		if name == "" || field == "" {
			continue
		}

		labels = append(labels, fmt.Sprintf("%s=$%s", name, field))
	}

	return strings.Join(labels, ", ")
}

func buildLokiStructuredMetadata(UserInput UserInput) string {
	var metadata []string
	for _, item := range UserInput.LokiStructuredMetadata {
		name := strings.TrimSpace(item.Name)
		field := strings.TrimSpace(item.Field)

		if name == "" || field == "" {
			continue
		}

		metadata = append(metadata, fmt.Sprintf("%s=$%s", name, field))
	}

	return strings.Join(metadata, ", ")
}

func buildLokiLineFormat(userInput UserInput) string {
	switch strings.TrimSpace(userInput.LokiLineFormat) {
	case "key_value":
		return "key_value"
	case "json":
		return "json"
	default:
		return "json"
	}
}

// ToYAML renders FluentBitConfig into YAML.
func ToYAML(cfg FluentBitConfig) ([]byte, error) {
	var buf bytes.Buffer

	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(cfg); err != nil {
		return nil, err
	}

	output := buf.String()

	// format yaml section spacing
	output = strings.ReplaceAll(output, "\nservice:", "\n\nservice:")
	output = strings.ReplaceAll(output, "\nparsers:", "\n\nparsers:")
	output = strings.ReplaceAll(output, "\nmultiline_parsers:", "\n\nmultiline_parsers:")
	output = strings.ReplaceAll(output, "\npipeline:", "\n\npipeline:")
	output = strings.ReplaceAll(output, "\n  filters:", "\n\n  filters:")
	output = strings.ReplaceAll(output, "\n  outputs:", "\n\n  outputs:")

	// remove quotes for known fluent-bit string fields
	var onPattern = regexp.MustCompile(`(?m): "(on|off|\+0800)"$`)
	output = onPattern.ReplaceAllString(output, `: $1`)

	return []byte(output), nil
}

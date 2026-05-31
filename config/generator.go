package config

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"flbctl/parser"

	"gopkg.in/yaml.v3"
)

// Build converts user input into Fluent-Bit domain model.
func Build(userInput UserInput) (FluentBitConfig, error) {
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
		parserObj, err := parser.BuildParser(userInput.ParserInput)
		if err != nil {
			return FluentBitConfig{}, err
		}

		cfg.Parsers = []parser.Parser{
			parserObj,
		}

		filterObj := parser.BuildFilter(userInput.ParserInput)
		cfg.Pipeline.Filters = []interface{}{
			filterObj,
		}

		if userInput.ParserInput.EnableMultiline {
			multilineParserObj, err := parser.BuildMultilineParser(userInput.ParserInput)
			if err != nil {
				return FluentBitConfig{}, err
			}

			cfg.MultilineParsers = []parser.MultilineParser{
				multilineParserObj,
			}
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

func buildBaseEnv(userInput UserInput) Env {
	env := Env{
		LogPath: strings.Join(userInput.LogPaths, ","),
		DBPath:  userInput.DBPath,
		Tag:     userInput.Tag,
	}

	if userInput.Mode == "production" {
		env.FLBLogFile = userInput.FluentBitLogFile
		env.StoragePath = userInput.StoragePath
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
	service := Service{
		LogLevel: "error",

		SchedulerBase: 5,
		SchedulerCap:  300,
	}

	if userInput.Mode == "production" {
		if userInput.FluentBitLogFile != "" {
			service.LogFile = "${FLB_LOG_FILE}"
		}
		service.StoragePath = "${STORAGE_PATH}"
		service.StorageSync = "full"
		service.StorageDeleteIrrecoverableChunks = "on"
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
		StorageType:     "filesystem",
		Threaded:        "on",
	}

	if userInput.DBPath != "" {
		tailInput.DB = "${DB_PATH}"
	}

	if userInput.ReadFromHead {
		tailInput.ReadFromHead = "on"
	}

	if userInput.Mode == "production" {
		tailInput.StorageType = "filesystem"
	}

	if userInput.EnableParser && userInput.ParserInput.EnableMultiline {
		tailInput.MultilineParser = userInput.ParserInput.MultilineParserName
	}

	return tailInput
}

func buildStdoutOutput() StdoutOutput {
	return StdoutOutput{
		Name:  "stdout",
		Match: "${TAG}",
	}
}

func buildLokiOutput(userInput UserInput) LokiOutput {
	return LokiOutput{
		Name:  "loki",
		Match: "${TAG}",

		Host:     "${LOKI_HOST}",
		Port:     443,
		TenantID: "${TENANT_ID}",
		Labels:   buildLokiLabels(userInput),

		TLS:        "on",
		TLSCAFile:  "${TLS_CA_FILE}",
		TLSCrtFile: "${TLS_CRT_FILE}",
		TLSKeyFile: "${TLS_KEY_FILE}",

		RetryLimit: "no_limits",

		StorageTotalLimitSize: "1G",

		NetConnectTimeout:       310,
		NetKeepaliveIdleTimeout: 30,
		NetDNSResolver:          "ASYNC",

		Workers: 2,
	}
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

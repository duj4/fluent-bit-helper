package config

import (
	"bytes"
	"regexp"
	"strings"

	"flbctl/parser"

	"gopkg.in/yaml.v3"
)

// Build converts user input into Fluent-Bit domain model.
func Build(userInput UserInput) (FluentBitConfig, error) {
	cfg := FluentBitConfig{
		Env: buildBaseEnv(userInput),

		Service: buildService(),

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
			buildLokiOutput(),
		}

		return cfg, nil
	}

	cfg.Pipeline.Outputs = []interface{}{
		buildStdoutOutput(),
	}

	return cfg, nil
}

func buildBaseEnv(userInput UserInput) Env {
	return Env{
		FLBLogFile:  userInput.FluentBitLogFile,
		StoragePath: userInput.StoragePath,

		LogPath: strings.Join(userInput.LogPaths, ","),
		DBPath:  userInput.DBPath,
		Tag:     userInput.Tag,
	}
}

func buildLokiEnv(env Env, userInput UserInput) Env {
	env.LokiHost = userInput.LokiHost
	env.TenantID = userInput.TenantID
	env.TLSCAFile = userInput.TLSCAFile
	env.TLSCrtFile = userInput.TLSCrtFile
	env.TLSKeyFile = userInput.TLSKeyFile

	return env
}

func buildService() Service {
	return Service{
		LogFile:  "${FLB_LOG_FILE}",
		LogLevel: "error",

		SchedulerBase: 5,
		SchedulerCap:  300,

		StoragePath:                      "${STORAGE_PATH}",
		StorageChecksum:                  "on",
		StorageDeleteIrrecoverableChunks: "on",
	}
}

func buildTailInput(userInput UserInput) TailInput {
	tailInput := TailInput{
		Name: "tail",

		Tag:     "${TAG}",
		Path:    "${LOG_PATH}",
		PathKey: "filename",
		DB:      "${DB_PATH}",

		SkipEmptyLines:  "on",
		RefreshInterval: 5,
		StorageType:     "filesystem",
		Threaded:        "on",
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

func buildLokiOutput() LokiOutput {
	return LokiOutput{
		Name:  "loki",
		Match: "${TAG}",

		Host:     "${LOKI_HOST}",
		Port:     443,
		TenantID: "${TENANT_ID}",
		Labels:   "filename=$filename, instance=${HOSTNAME}",

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

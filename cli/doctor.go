package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"flbctl/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// custom YAML configuration file
var doctorFile string

var doctorCmd = &cobra.Command{
	Use:     "doctor <path to the Fluent-Bit config file>",
	Short:   "Check an existing Fluent-Bit config file for baseline compliance",
	Example: "flbctl doctor fluent-bit.yaml",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		doctorFile := args[0]

		content, err := os.ReadFile(doctorFile)
		if err != nil {
			return err
		}

		var cfg config.FluentBitConfig
		if err := yaml.Unmarshal(content, &cfg); err != nil {
			return err
		}

		// check each section
		checkService(cfg.Service)
		fmt.Println()
		checkInputs(cfg.Pipeline.Inputs)
		fmt.Println()
		checkOutputs(cfg.Pipeline.Outputs)

		return nil
	},
}

// checkService validates service section baseline
func checkService(service config.Service) {
	fmt.Println("== Service ==")

	if service == (config.Service{}) {
		fmt.Println("[ERROR] service section is not configured")
		return
	}

	// log_file
	checkPathString("log_file", service.LogFile, "storing fluent-bit's log for troubleshooting")

	// scheduler.base
	checkEqual("scheduler.base", service.SchedulerBase, 5)

	// scheduler.cap
	checkEqual("scheduler.cap", service.SchedulerCap, 300)

	// storage.path
	checkPathString("storage.path", service.StoragePath, "storing the buffered log chunks when storage.type in input is set as \"filesystem\"")

	// storage.checksum
	checkEqual("storage.checksum", service.StorageChecksum, "on")

	// storage.delete_irrecoverable_chunks
	checkEqual("storage.delete_irrecoverable_chunks", service.StorageDeleteIrrecoverableChunks, "on")
}

// checkInputs validates tail inputs baseline configuration
func checkInputs(inputs []interface{}) {
	fmt.Println("== Inputs ==")

	if len(inputs) == 0 {
		fmt.Println("[ERROR] inputs is empty, configure at least one input plugin")
		return
	}

	// load all inputs
	for i, input := range inputs {
		fmt.Printf("input %d:\n", i)

		// check tail input only
		// TODO: add more input plugin
		inputMap, ok := input.(map[string]interface{})
		if !ok {
			fmt.Printf("[ERROR] input %d has invalid format, skipping\n", i)
			continue
		}

		if inputMap["name"] != "tail" {
			fmt.Printf("[WARN] input %d is not tail input, skipping\n", i)
			continue
		}

		// tag
		checkMapValue("tag", inputMap, nil)

		// db
		checkPathFromMap("db", inputMap, "storing the offset of monitored file")

		// skip_empty_lines
		checkMapValue("skip_empty_lines", inputMap, "on")

		// refresh_interval
		checkMapValueMin("refresh_interval", inputMap, 1, 5)

		// storage.type
		checkMapValue("storage.type", inputMap, "filesystem")

		// threaded
		checkMapValue("threaded", inputMap, "on")

		// read_from_head
		checkMapValue("read_from_head", inputMap, "off")
	}
}

// checkOutputs validates stdout/loki outputs baseline
func checkOutputs(outputs []interface{}) {
	fmt.Println("== Outputs ==")

	if len(outputs) == 0 {
		fmt.Println("[ERROR] outputs is empty, configure at least one output plugin")
		return
	}

	// load all outputs
	for i, output := range outputs {
		fmt.Printf("output %d:\n", i)

		outputMap, ok := output.(map[string]interface{})
		if !ok {
			fmt.Printf("[ERROR] output %d has invalid format, skipping\n", i)
			continue
		}

		switch outputMap["name"] {
		case "stdout":
			// match
			checkMapValue("match", outputMap, nil)
		case "loki":
			// match
			checkMapValue("match", outputMap, nil)

			// host
			checkMapValue("host", outputMap, nil)

			// port
			checkMapValue("port", outputMap, 443)

			// tenant_id
			checkMapValue("tenant_id", outputMap, nil)

			// labels
			checkMapValue("labels", outputMap, nil)

			// tls
			checkMapValue("tls", outputMap, "on")

			// tls.ca_file
			checkPathFromMap("tls.ca_file", outputMap, "the absolute path to CA file")

			// tls.crt_file
			checkPathFromMap("tls.crt_file", outputMap, "the absolute path to certificate file")

			// tls.key_file
			checkPathFromMap("tls.key_file", outputMap, "the absolute path to key file")

			// retry_limit
			checkMapValue("retry_limit", outputMap, "no_limits")

			// storage.total_limit_size
			checkMapValueMin("storage.total_limit_size", outputMap, "1G", nil)

			// net.connect_timeout
			checkMapValue("net.connect_timeout", outputMap, 310)

			// net.keepalive_idle_timeout
			checkMapValue("net.keepalive_idle_timeout", outputMap, 300)

			// net.dns.resolver
			checkMapValue("net.dns.resolver", outputMap, "ASYNC")

			// workers
			checkMapValue("workers", outputMap, 2)
		default:
			fmt.Printf("[WARN] unknown output %d\n", i)
		}
	}
}

// checkPathFromMap checks a path v from a weakly-typed map
func checkPathFromMap(name string, inputMap map[string]interface{}, notes string) {
	raw, exists := inputMap[name]
	if !exists || raw == nil {
		fmt.Printf("[%s] %s is not configured, it is %s\n", severity(name), name, notes)
		return
	}
	path, ok := raw.(string)
	if !ok {
		fmt.Printf("[%s] %s should be a string\n", severity(name), name)
		return
	}
	checkPathString(name, path, notes)
}

// checkPathString checks if a path exists and prints appropriate info for strongly-typed value
func checkPathString(name, path, notes string) {
	if strings.TrimSpace(path) == "" {
		fmt.Printf("[%s] %s is not configured, it is %s\n", severity(name), name, notes)
		return
	}

	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("[%s] %s does not exist: %s\n", severity(name), name, path)
		} else {
			fmt.Printf("[%s] faild to access %s: %v\n", severity(name), path, err)
		}
		return
	}

	fmt.Printf("[OK] %s exists: %s\n", name, path)
}

// checkMapValue checks a field value from a weakly-typed map against an expected value
func checkMapValue(name string, inputMap map[string]interface{}, expected interface{}) {
	raw, exists := inputMap[name]
	if !exists || raw == nil {
		if expected == nil {
			fmt.Printf("[%s] %s is not configured\n", severity(name), name)
		} else {
			fmt.Printf("[%s] %s is not configured, recommended: %v\n", severity(name), name, expected)
		}
		return
	}

	if expected == nil {
		fmt.Printf("[OK] %s = %v\n", name, raw)
		return
	}

	switch expected.(type) {
	case int:
		if _, ok := raw.(int); !ok {
			fmt.Printf("[%s] %s should be an integer, recommended: %v\n", severity(name), name, expected)
			return
		}
	case string:
		v, ok := raw.(string)
		if !ok {
			fmt.Printf("[%s] %s should be a string, recommended: %v\n", severity(name), name, expected)
			return
		}
		if strings.TrimSpace(v) == "" {
			fmt.Printf("[%s] %s is not configured, recommended: %v\n", severity(name), name, expected)
			return
		}
	}

	checkEqual(name, raw, expected)
}

// checkEqual checks if a strongly-typed value matches the expected baseline
func checkEqual(name string, got, expected interface{}) {
	if got != expected {
		fmt.Printf("[%s] %s should be %v\n", severity(name), name, expected)
	} else {
		fmt.Printf("[OK] %s = %v\n", name, got)
	}
}

// checkMapValueMin checks if a value matches a minimum threshold
// For string values like "1G", "500m", "100K", it parses and compares in bytes
// For int values, it compares directly
func checkMapValueMin(name string, inputMap map[string]interface{}, min interface{}, recommended interface{}) {
	raw, exists := inputMap[name]
	if !exists || raw == nil {
		fmt.Printf("[%s] %s is not configured, recommended: %v\n", severity(name), name, recommended)
		return
	}

	switch min.(type) {
	case int:
		v, ok := raw.(int)
		if !ok {
			fmt.Printf("[%s] %s should be an integer.\n", severity(name), name)
			return
		}
		if v < min.(int) {
			fmt.Printf("[%s] %s should be >= %v\n", severity(name), name, min)
		} else if rec, ok := recommended.(int); ok && v < rec {
			fmt.Printf("[OK] %s = %v (recommended >= %v)\n", name, v, recommended)
		} else {
			fmt.Printf("[OK] %s = %v\n", name, v)
		}

	case string:
		v, ok := raw.(string)
		if !ok {
			fmt.Printf("[%s] %s should be a string\n", severity(name), name)
			return
		}
		minBytes, _ := parseSize(min.(string))
		actualBytes, err := parseSize(v)
		if err != nil {
			fmt.Printf("[%s] %s has invalid format, expected e.g. 1G, 500M, 100k, but we have: %s\n", severity(name), name, v)
			return
		}
		if actualBytes < minBytes {
			fmt.Printf("[%s] %s should be at least %v\n", severity(name), name, min)
		} else {
			fmt.Printf("[OK] %s = %v\n", name, v)
		}
	}
}

// parseSize parses size strings like "1G", "500m", "100K" into bytes
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	units := map[string]int64{
		"K": 1024,
		"M": 1024 * 1024,
		"G": 1024 * 1024 * 1024,
	}
	for suffix, multiplier := range units {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			num, err := strconv.ParseInt(strings.TrimSpace(numStr), 10, 64)
			if err != nil {
				return 0, err
			}
			return num * multiplier, nil
		}
	}

	// treat value as bytes if no suffix
	return strconv.ParseInt(s, 10, 64)
}

// errorKeys defines fields whose absence or invalidity is critical
var errorKeys = map[string]bool{
	"storage.path":   true,
	"tag":            true,
	"db":             true,
	"storage.type":   true,
	"read_from_head": true,
	"match":          true,
	"host":           true,
	"port":           true,
	"tenant_id":      true,
	"tls":            true,
	"tls.ca_file":    true,
	"tls.crt_file":   true,
	"tls.key_file":   true,
}

func severity(name string) string {
	if errorKeys[name] {
		return "ERROR"
	}
	return "WARN"
}

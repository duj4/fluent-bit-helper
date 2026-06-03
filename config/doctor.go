package config

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DoctorModeProduction = "production"
	DoctorModeQuickTest  = "quick-test"
)

// DoctorLevel represents the severity of one doctor check.
// These values are returned directly to the Web UI, so keep them stable.
type DoctorLevel string

const (
	// DoctorOK means the checked field matches the selected baseline.
	DoctorOK DoctorLevel = "OK"

	// DoctorWarn means the config is usable but not fully aligned with the selected baseline.
	DoctorWarn DoctorLevel = "WARN"

	// DoctorError means the config is missing or misconfigured for an important baseline field.
	DoctorError DoctorLevel = "ERROR"
)

// DoctorCheck is one structured check result returned by DoctorYAMLWithMode.
// The Web UI groups checks by Section and displays Level, Field, Message, Actual, and Expect.
type DoctorCheck struct {
	Section string      `json:"section"`
	Level   DoctorLevel `json:"level"`
	Field   string      `json:"field,omitempty"`
	Message string      `json:"message"`
	Actual  string      `json:"actual,omitempty"`
	Default string      `json:"default,omitempty"`
	Expect  string      `json:"expect,omitempty"`
}

// DoctorReport is the complete doctor result for one Fluent-Bit YAML document.
// The count fields are pre-calculated so the frontend does not need to derive them.
type DoctorReport struct {
	Mode         string        `json:"mode"`
	Checks       []DoctorCheck `json:"checks"`
	ErrorCount   int           `json:"error_count"`
	WarningCount int           `json:"warning_count"`
	OKCount      int           `json:"ok_count"`
}

// doctorCollector accumulates doctor checks and updates summary counters.
// It keeps the checking functions simple and avoids printing from the config package.
type doctorCollector struct {
	mode   string
	report DoctorReport
}

// DoctorYAML validates an existing Fluent-Bit YAML against the production baseline.
// It is kept as a convenience wrapper for callers that do not provide a mode.
func DoctorYAML(content []byte) (DoctorReport, error) {
	return DoctorYAMLWithMode(content, DoctorModeProduction)
}

// DoctorYAMLWithMode validates an existing Fluent-Bit YAML against the selected baseline.
// It accepts YAML content directly instead of a file path so it can be reused by the Web API.
func DoctorYAMLWithMode(content []byte, mode string) (DoctorReport, error) {
	if strings.TrimSpace(string(content)) == "" {
		return DoctorReport{}, fmt.Errorf("yaml content is empty")
	}

	mode = normalizeDoctorMode(mode)

	var root map[string]interface{}
	if err := yaml.Unmarshal(content, &root); err != nil {
		return DoctorReport{}, err
	}

	collector := &doctorCollector{
		mode: mode,
		report: DoctorReport{
			Mode: mode,
		},
	}

	collector.checkRoot(root)

	return collector.report, nil
}

func normalizeDoctorMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case DoctorModeQuickTest:
		return DoctorModeQuickTest
	default:
		return DoctorModeProduction
	}
}

func (c *doctorCollector) checkRoot(root map[string]interface{}) {
	serviceMap, hasService := asDoctorMap(root["service"])
	if c.mode == DoctorModeQuickTest {
		c.checkQuickTestService(serviceMap, hasService)
	} else {
		c.checkProductionService(serviceMap, hasService)
	}

	pipelineMap, hasPipeline := asDoctorMap(root["pipeline"])
	if !hasPipeline {
		c.add("Pipeline", DoctorError, "pipeline", "pipeline section is not configured", nil, nil)
		return
	}

	inputs, _ := asDoctorList(pipelineMap["inputs"])
	c.checkInputs(inputs)

	outputs, _ := asDoctorList(pipelineMap["outputs"])
	c.checkOutputs(outputs)
}

// add records one check and updates the corresponding OK/WARN/ERROR counter.
func (c *doctorCollector) add(section string, level DoctorLevel, field string, message string, actual interface{}, expect interface{}) {
	c.addWithDefault(section, level, field, message, actual, nil, expect)
}

func (c *doctorCollector) addWithDefault(section string, level DoctorLevel, field string, message string, actual interface{}, defaultValue interface{}, expect interface{}) {
	check := DoctorCheck{
		Section: section,
		Level:   level,
		Field:   field,
		Message: message,
		Actual:  stringifyDoctorValue(actual),
		Default: stringifyDoctorValue(defaultValue),
		Expect:  stringifyDoctorValue(expect),
	}

	c.report.Checks = append(c.report.Checks, check)

	switch level {
	case DoctorOK:
		c.report.OKCount++
	case DoctorWarn:
		c.report.WarningCount++
	case DoctorError:
		c.report.ErrorCount++
	}
}

func (c *doctorCollector) checkProductionService(serviceMap map[string]interface{}, exists bool) {
	section := "Service"

	if !exists {
		c.add(section, DoctorError, "service", "service section is not configured", nil, nil)
		return
	}

	c.checkOptionalPath(section, "log_file", serviceMap, "log_file is not configured; recommended for troubleshooting. Logrotate should be configured separately")
	c.checkMapValueWithDefault(section, "log_level", serviceMap, "error", DoctorWarn)
	c.checkMapValueWithDefault(section, "scheduler.base", serviceMap, 5, DoctorWarn)
	c.checkMapValueWithDefault(section, "scheduler.cap", serviceMap, 300, DoctorWarn)
	c.checkPathFromMapWithDefault(section, "storage.path", serviceMap, "${STORAGE_PATH}", DoctorWarn)
	c.checkMapValue(section, "storage.sync", serviceMap, "full")
	c.checkMapValue(section, "storage.delete_irrecoverable_chunks", serviceMap, "on")
}

func (c *doctorCollector) checkQuickTestService(serviceMap map[string]interface{}, exists bool) {
	section := "Service"

	if !exists {
		c.add(section, DoctorWarn, "service", "service section is not configured; log_level debug is recommended for quick test", nil, "log_level: debug")
		return
	}

	c.checkMapValue(section, "log_level", serviceMap, "debug")
	c.checkShouldBeAbsent(section, "scheduler.base", serviceMap)
	c.checkShouldBeAbsent(section, "scheduler.cap", serviceMap)
	c.checkShouldBeAbsent(section, "storage.path", serviceMap)
}

// checkInputs validates pipeline.inputs.
// The current baseline only supports tail input; other input plugins are skipped with a warning.
func (c *doctorCollector) checkInputs(inputs []interface{}) {
	if len(inputs) == 0 {
		c.add("Inputs", DoctorError, "inputs", "inputs is empty; configure at least one input plugin", nil, nil)
		return
	}

	for i, input := range inputs {
		section := fmt.Sprintf("Input %d", i)

		inputMap, ok := asDoctorMap(input)
		if !ok {
			c.add(section, DoctorError, "input", "input has invalid format", nil, nil)
			continue
		}

		if inputMap["name"] != "tail" {
			c.add(section, DoctorWarn, "name", "input is not tail input; skipped", inputMap["name"], "tail")
			continue
		}

		c.checkMapValue(section, "tag", inputMap, nil)
		c.checkPathFromMap(section, "path", inputMap, "log path to monitor")
		c.checkMapValue(section, "skip_empty_lines", inputMap, "on")
		c.checkMapValueMin(section, "refresh_interval", inputMap, 1, 5)
		c.checkMapValue(section, "threaded", inputMap, "on")

		if c.mode == DoctorModeQuickTest {
			c.checkShouldBeAbsent(section, "db", inputMap)
			c.checkShouldBeAbsent(section, "storage.type", inputMap)
			c.checkMapValue(section, "read_from_head", inputMap, "on")
			continue
		}

		c.checkPathFromMap(section, "db", inputMap, "tail offset database file is recommended")
		c.checkMapValueWithDefault(section, "storage.type", inputMap, "filesystem", DoctorWarn)
		c.checkReadFromHead(section, inputMap)
	}
}

// checkOutputs validates pipeline.outputs.
// stdout is only lightly checked, while loki is checked against the selected baseline.
func (c *doctorCollector) checkOutputs(outputs []interface{}) {
	if len(outputs) == 0 {
		c.add("Outputs", DoctorError, "outputs", "outputs is empty; configure at least one output plugin", nil, nil)
		return
	}

	for i, output := range outputs {
		section := fmt.Sprintf("Output %d", i)

		outputMap, ok := asDoctorMap(output)
		if !ok {
			c.add(section, DoctorError, "output", "output has invalid format", nil, nil)
			continue
		}

		switch outputMap["name"] {
		case "stdout":
			c.checkMapValue(section, "match", outputMap, nil)

		case "loki":
			c.checkLokiOutput(section, outputMap)

		default:
			c.add(section, DoctorWarn, "name", "unknown output plugin", outputMap["name"], nil)
		}
	}
}

func (c *doctorCollector) checkLokiOutput(section string, outputMap map[string]interface{}) {
	c.checkMapValue(section, "match", outputMap, nil)
	c.checkMapValue(section, "host", outputMap, nil)
	c.checkMapValue(section, "port", outputMap, 443)
	c.checkMapValue(section, "tenant_id", outputMap, nil)
	c.checkMapValue(section, "labels", outputMap, nil)
	c.checkMapValue(section, "tls", outputMap, "on")
	c.checkPathFromMap(section, "tls.ca_file", outputMap, "absolute path to CA file")
	c.checkPathFromMap(section, "tls.crt_file", outputMap, "absolute path to certificate file")
	c.checkPathFromMap(section, "tls.key_file", outputMap, "absolute path to key file")
	c.checkMapValue(section, "retry_limit", outputMap, "no_limits")
	c.checkMapValue(section, "net.dns.resolver", outputMap, "ASYNC")
	c.checkMapValue(section, "workers", outputMap, 2)

	if c.mode == DoctorModeQuickTest {
		c.checkShouldBeAbsent(section, "storage.total_limit_size", outputMap)
		c.checkShouldBeAbsent(section, "net.connect_timeout", outputMap)
		c.checkShouldBeAbsent(section, "net.keepalive_idle_timeout", outputMap)
	} else {
		c.checkMapValueWithDefault(section, "storage.total_limit_size", outputMap, "1G", DoctorWarn)
		c.checkMapValueWithDefault(section, "net.connect_timeout", outputMap, 310, DoctorWarn)
		c.checkMapValueWithDefault(section, "net.keepalive_idle_timeout", outputMap, 300, DoctorWarn)
	}

	// structured_metadata is optional. If it exists, only verify that it has a value.
	if _, exists := outputMap["structured_metadata"]; exists {
		c.checkMapValue(section, "structured_metadata", outputMap, nil)
	}
}

func (c *doctorCollector) checkShouldBeAbsent(section string, name string, inputMap map[string]interface{}) {
	raw, exists := inputMap[name]
	if !exists || raw == nil || strings.TrimSpace(fmt.Sprintf("%v", raw)) == "" {
		return
	}

	c.add(section, DoctorWarn, name, fmt.Sprintf("%s can be removed for quick test", name), raw, nil)
}

// checkPathFromMap reads a path-like field from a generic map and delegates the actual path validation.
func (c *doctorCollector) checkPathFromMap(section string, name string, inputMap map[string]interface{}, notes string) {
	raw, exists := inputMap[name]
	if !exists || raw == nil {
		c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s is not configured: %s", name, notes), nil, nil)
		return
	}

	path, ok := raw.(string)
	if !ok {
		c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s should be a string", name), raw, "string")
		return
	}

	c.checkPathString(section, name, path, notes)
}

func (c *doctorCollector) checkPathFromMapWithDefault(section string, name string, inputMap map[string]interface{}, expected interface{}, missingLevel DoctorLevel) {
	raw, exists := inputMap[name]
	if !exists || raw == nil || strings.TrimSpace(fmt.Sprintf("%v", raw)) == "" {
		c.add(section, missingLevel, name, fmt.Sprintf("%s is not configured", name), nil, expected)
		return
	}

	path, ok := raw.(string)
	if !ok {
		c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s should be a string", name), raw, "string")
		return
	}

	c.checkPathString(section, name, path, expected)
}

func (c *doctorCollector) checkOptionalPath(section string, name string, inputMap map[string]interface{}, missingMessage string) {
	raw, exists := inputMap[name]
	if !exists || raw == nil || strings.TrimSpace(fmt.Sprintf("%v", raw)) == "" {
		c.add(section, DoctorWarn, name, missingMessage, nil, nil)
		return
	}

	path, ok := raw.(string)
	if !ok {
		c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s should be a string", name), raw, "string")
		return
	}

	c.checkPathString(section, name, path, nil)
}

// checkPathString validates a path-like value without checking the server filesystem.
// This is intentional for the Web service: the YAML paths belong to the user's Fluent-Bit host,
// not to the fluent-bit-helper Web server. Therefore, we only check whether the path is configured and
// whether it looks like an absolute path or a variable-based path such as ${TLS_CA_FILE}.
func (c *doctorCollector) checkPathString(section string, name string, path string, expect interface{}) {
	path = strings.TrimSpace(path)

	if path == "" {
		c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s is not configured", name), nil, expect)
		return
	}

	if !isDoctorPathAcceptable(path) {
		c.add(section, DoctorWarn, name, fmt.Sprintf("%s should usually be an absolute path", name), path, expect)
		return
	}

	c.add(section, DoctorOK, name, fmt.Sprintf("%s is configured", name), path, expect)
}

// checkMapValue validates a scalar field from a generic map.
// If expected is nil, the field only needs to be present and non-nil.
func (c *doctorCollector) checkMapValue(section string, name string, inputMap map[string]interface{}, expected interface{}) {
	raw, exists := inputMap[name]
	if !exists || raw == nil {
		if expected == nil {
			c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s is not configured", name), nil, nil)
		} else {
			c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s is not configured", name), nil, expected)
		}
		return
	}

	if expected == nil {
		c.add(section, DoctorOK, name, fmt.Sprintf("%s is configured", name), raw, nil)
		return
	}

	if !sameDoctorValue(raw, expected) {
		c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s should be %v", name, expected), raw, expected)
		return
	}

	c.add(section, DoctorOK, name, fmt.Sprintf("%s = %v", name, raw), raw, expected)
}

func (c *doctorCollector) checkMapValueWithDefault(section string, name string, inputMap map[string]interface{}, expected interface{}, missingLevel DoctorLevel) {
	raw, exists := inputMap[name]
	if !exists || raw == nil || strings.TrimSpace(fmt.Sprintf("%v", raw)) == "" {
		c.add(section, missingLevel, name, fmt.Sprintf("%s is not configured", name), nil, expected)
		return
	}

	if !sameDoctorValue(raw, expected) {
		c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s should be %v", name, expected), raw, expected)
		return
	}

	c.add(section, DoctorOK, name, fmt.Sprintf("%s = %v", name, raw), raw, expected)
}

// checkReadFromHead handles read_from_head specially.
// For production baseline, an omitted value is acceptable because Fluent-Bit defaults it to off.
func (c *doctorCollector) checkReadFromHead(section string, inputMap map[string]interface{}) {
	raw, exists := inputMap["read_from_head"]
	if !exists || raw == nil {
		c.add(section, DoctorOK, "read_from_head", "read_from_head is not configured and defaults to off", nil, "off")
		return
	}

	if !sameDoctorValue(raw, "off") {
		c.add(section, DoctorWarn, "read_from_head", "read_from_head should be off for production baseline", raw, "off")
		return
	}

	c.add(section, DoctorOK, "read_from_head", fmt.Sprintf("read_from_head = %v", raw), raw, "off")
}

// checkMapValueMin validates minimum numeric or size thresholds.
// Integer values are compared directly. Size strings such as 1G, 500M, or 100K are normalized to bytes.
func (c *doctorCollector) checkMapValueMin(section string, name string, inputMap map[string]interface{}, min interface{}, recommended interface{}) {
	raw, exists := inputMap[name]
	if !exists || raw == nil {
		c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s is not configured", name), nil, recommended)
		return
	}

	switch minValue := min.(type) {
	case int:
		v, ok := doctorIntValue(raw)
		if !ok {
			c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s should be an integer", name), raw, "integer")
			return
		}

		if v < minValue {
			c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s should be >= %v", name, minValue), v, minValue)
			return
		}

		if rec, ok := recommended.(int); ok && v < rec {
			c.add(section, DoctorOK, name, fmt.Sprintf("%s = %v; recommended >= %v", name, v, rec), v, rec)
			return
		}

		c.add(section, DoctorOK, name, fmt.Sprintf("%s = %v", name, v), v, minValue)

	case string:
		v, ok := raw.(string)
		if !ok {
			c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s should be a string", name), raw, "string")
			return
		}

		minBytes, _ := parseDoctorSize(minValue)
		actualBytes, err := parseDoctorSize(v)
		if err != nil {
			c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s has invalid size format", name), v, minValue)
			return
		}

		if actualBytes < minBytes {
			c.add(section, doctorSeverity(name), name, fmt.Sprintf("%s should be at least %v", name, minValue), v, minValue)
			return
		}

		c.add(section, DoctorOK, name, fmt.Sprintf("%s = %v", name, v), v, minValue)
	}
}

// asDoctorMap normalizes map values produced by YAML unmarshalling into map[string]interface{}.
func asDoctorMap(value interface{}) (map[string]interface{}, bool) {
	m, ok := value.(map[string]interface{})
	if ok {
		return m, true
	}

	stringMap, ok := value.(map[string]string)
	if !ok {
		return nil, false
	}

	converted := make(map[string]interface{}, len(stringMap))
	for k, v := range stringMap {
		converted[k] = v
	}

	return converted, true
}

func asDoctorList(value interface{}) ([]interface{}, bool) {
	items, ok := value.([]interface{})
	if ok {
		return items, true
	}

	return nil, false
}

// sameDoctorValue compares YAML values with the expected baseline while handling common YAML type differences.
func sameDoctorValue(raw interface{}, expected interface{}) bool {
	switch expectedValue := expected.(type) {
	case int:
		rawInt, ok := doctorIntValue(raw)
		return ok && rawInt == expectedValue
	case string:
		rawString, ok := raw.(string)
		return ok && strings.TrimSpace(rawString) == expectedValue
	default:
		return raw == expected
	}
}

// doctorIntValue converts YAML numeric values to int when the value is safely representable as an integer.
func doctorIntValue(raw interface{}) (int, bool) {
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		if v == float64(int(v)) {
			return int(v), true
		}
		return 0, false
	default:
		return 0, false
	}
}

// stringifyDoctorValue converts optional Actual/Expect values to strings for JSON output.
func stringifyDoctorValue(value interface{}) string {
	if value == nil {
		return ""
	}

	return fmt.Sprintf("%v", value)
}

// parseDoctorSize parses size strings like 1G, 500M, or 100K into bytes.
func parseDoctorSize(s string) (int64, error) {
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

	return strconv.ParseInt(s, 10, 64)
}

// isDoctorPathAcceptable returns true for absolute paths or variable-based paths.
// Variable-based paths are accepted because fluent-bit-helper-generated YAML may use values such as ${TLS_CA_FILE}.
func isDoctorPathAcceptable(path string) bool {
	if filepath.IsAbs(path) {
		return true
	}

	return strings.Contains(path, "${")
}

// doctorErrorKeys defines fields that should be treated as ERROR when missing or invalid.
// Fields not listed here fall back to WARN.
var doctorErrorKeys = map[string]bool{
	"pipeline":     true,
	"inputs":       true,
	"outputs":      true,
	"tag":          true,
	"path":         true,
	"db":           true,
	"match":        true,
	"host":         true,
	"port":         true,
	"tenant_id":    true,
	"tls":          true,
	"tls.ca_file":  true,
	"tls.crt_file": true,
	"tls.key_file": true,
}

// doctorSeverity returns ERROR for required baseline fields and WARN for recommended fields.
func doctorSeverity(name string) DoctorLevel {
	if doctorErrorKeys[name] {
		return DoctorError
	}

	return DoctorWarn
}

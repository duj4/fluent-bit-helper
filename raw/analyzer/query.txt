package analyzer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/prometheus/prometheus/model/labels"
	promparser "github.com/prometheus/prometheus/promql/parser"
)

var metricName = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

// parseInput substitutes known time macros only outside quoted strings/comments.
// It never substitutes an unknown variable into a fabricated metric identity.
func parseInput(query string) (string, error) {
	if len(query) > 32768 {
		return "", fmt.Errorf("query exceeds the 32 KiB analysis limit")
	}
	var out strings.Builder
	quote := byte(0)
	comment := false
	depth := 0
	squareDepth := 0
	for i := 0; i < len(query); {
		c := query[i]
		if comment {
			out.WriteByte(c)
			i++
			if c == '\n' {
				comment = false
			}
			continue
		}
		if quote != 0 {
			out.WriteByte(c)
			i++
			if c == '\\' && quote != '`' && i < len(query) {
				out.WriteByte(query[i])
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '#' {
			comment = true
			out.WriteByte(c)
			i++
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			quote = c
			out.WriteByte(c)
			i++
			continue
		}
		if c == '(' || c == '{' || c == '[' {
			depth++
			if depth > 128 {
				return "", fmt.Errorf("query nesting exceeds the analysis limit")
			}
		}
		if c == '[' {
			squareDepth++
		}
		if c == ']' {
			squareDepth--
		}
		if c == ')' || c == '}' || c == ']' {
			depth--
		}
		if c != '$' {
			out.WriteByte(c)
			i++
			continue
		}
		start := i
		i++
		var name string
		if i < len(query) && query[i] == '{' {
			end := strings.IndexByte(query[i:], '}')
			if end < 0 {
				return "", fmt.Errorf("unterminated Grafana variable")
			}
			name = query[i+1 : i+end]
			i += end + 1
		} else {
			begin := i
			for i < len(query) && (unicode.IsLetter(rune(query[i])) || unicode.IsDigit(rune(query[i])) || query[i] == '_') {
				i++
			}
			name = query[begin:i]
		}
		// Do not accept a macro embedded in a metric name or another token.
		if start > 0 && (unicode.IsLetter(rune(query[start-1])) || unicode.IsDigit(rune(query[start-1])) || query[start-1] == '_' || (query[start-1] == ':' && squareDepth == 0)) {
			return "", fmt.Errorf("Grafana variable in a metric or structural position needs manual review")
		}
		if i < len(query) && (unicode.IsLetter(rune(query[i])) || unicode.IsDigit(rune(query[i])) || query[i] == '_' || (query[i] == ':' && squareDepth == 0)) {
			return "", fmt.Errorf("Grafana variable joined to another token needs manual review")
		}
		switch name {
		case "__interval", "__rate_interval", "__range":
			out.WriteString("5m")
		case "__range_s", "__interval_ms":
			out.WriteString("300")
		default:
			return "", fmt.Errorf("unresolved Grafana variable in a metric or structural position; expand it before analysis")
		}
	}
	return out.String(), nil
}

// extractMetrics uses the existing PromQL parser only for exact selector names.
// Parser errors and original expressions are deliberately absent from the report.
func extractMetrics(expr string) ([]string, []string) {
	input, err := parseInput(expr)
	if err != nil {
		return nil, []string{err.Error()}
	}
	ast, err := promparser.ParseExpr(input)
	if err != nil {
		return nil, []string{"PromQL metric extraction failed; check syntax and unresolved variables."}
	}
	names := map[string]bool{}
	var issues []string
	promparser.Inspect(ast, func(node promparser.Node, _ []promparser.Node) error {
		vs, ok := node.(*promparser.VectorSelector)
		if !ok {
			return nil
		}
		name := vs.Name
		for _, m := range vs.LabelMatchers {
			if m.Name == "__name__" && m.Type == labels.MatchEqual {
				name = m.Value
			}
		}
		if !metricName.MatchString(name) {
			issues = appendUnique(issues, "A selector has no exact metric name; regex or dynamic metric selectors are not fully covered.")
			return nil
		}
		names[name] = true
		return nil
	})
	return sortedNames(names), issues
}

func sortedNames(names map[string]bool) []string {
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func appendUnique(values []string, value string) []string {
	for _, s := range values {
		if s == value {
			return values
		}
	}
	return append(values, value)
}

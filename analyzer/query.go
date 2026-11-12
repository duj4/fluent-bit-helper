package analyzer

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/prometheus/prometheus/model/labels"
	promparser "github.com/prometheus/prometheus/promql/parser"
)

type selectorContext struct {
	name        string
	matchers    []*labels.Matcher
	labelRefs   map[string]bool
	functions   map[string]bool
	labelStatus map[string]Status
	selector    *promparser.VectorSelector
	path        []promparser.Node
}

// A grouping or function reference can observe a label created or discarded
// below it. Such references cannot prove that an exporter label must change.
func (ctx *selectorContext) reference(label string, position int, path []promparser.Node) {
	level := RequiresChange
	for _, child := range path[position+1:] {
		switch n := child.(type) {
		case *promparser.AggregateExpr:
			contains := false
			for _, l := range n.Grouping {
				contains = contains || l == label
			}
			if (n.Without && contains) || (!n.Without && !contains) {
				level = Review
			}
		case *promparser.Call:
			if (n.Func.Name == "label_replace" || n.Func.Name == "label_join") && len(n.Args) > 1 {
				if target, ok := n.Args[1].(*promparser.StringLiteral); ok && target.Val == label {
					level = Review
				}
			}
		}
	}
	ctx.labelRefs[label] = true
	if ctx.labelStatus[label] != RequiresChange {
		ctx.labelStatus[label] = level
	}
}

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

func (e *Engine) analyzeQuery(refID, expr string) QueryResult {
	result := QueryResult{RefID: refID, Expr: expr, Status: NoImpact, Complete: true, Findings: []Finding{}}
	input, err := parseInput(expr)
	if err != nil {
		result.Complete = false
		result.Status = Review
		result.Issues = []string{err.Error()}
		return result
	}
	ast, err := promparser.ParseExpr(input)
	if err != nil {
		result.Complete = false
		result.Status = Review
		result.Issues = []string{"PromQL could not be parsed; check syntax and unresolved variables. " + err.Error()}
		return result
	}
	merged := map[string]*Finding{}
	promparser.Inspect(ast, func(node promparser.Node, path []promparser.Node) error {
		vs, ok := node.(*promparser.VectorSelector)
		if !ok {
			return nil
		}
		ctx := selectorContext{name: vs.Name, matchers: vs.LabelMatchers, labelRefs: map[string]bool{}, functions: map[string]bool{}, labelStatus: map[string]Status{}, selector: vs, path: path}
		for _, m := range vs.LabelMatchers {
			ctx.labelRefs[m.Name] = true
			ctx.labelStatus[m.Name] = RequiresChange
			if m.Name == "__name__" && m.Type == labels.MatchEqual {
				ctx.name = m.Value
			}
		}
		if ctx.name == "" || strings.Contains(ctx.name, "$") {
			result.Complete = false
			result.Issues = appendUnique(result.Issues, "A selector without an exact metric name (including __name__ regex selectors) cannot be exhaustively matched; review this query.")
			return nil
		}
		for pi, parent := range path {
			switch n := parent.(type) {
			case *promparser.Call:
				ctx.functions[n.Func.Name] = true
				if n.Func.Name == "label_replace" || n.Func.Name == "label_join" {
					for i, arg := range n.Args {
						if s, ok := arg.(*promparser.StringLiteral); ok {
							if (n.Func.Name == "label_replace" && i == 3) || (n.Func.Name == "label_join" && i >= 3) {
								ctx.reference(s.Val, pi, path)
							}
						}
					}
				}
			case *promparser.AggregateExpr:
				for _, l := range n.Grouping {
					ctx.reference(l, pi, path)
				}
			case *promparser.BinaryExpr:
				if n.VectorMatching != nil {
					for _, l := range n.VectorMatching.MatchingLabels {
						ctx.reference(l, pi, path)
					}
					var branch promparser.Node = node
					if pi+1 < len(path) {
						branch = path[pi+1]
					}
					if (n.VectorMatching.Card == promparser.CardManyToOne && branch == n.RHS) || (n.VectorMatching.Card == promparser.CardOneToMany && branch == n.LHS) {
						for _, l := range n.VectorMatching.Include {
							ctx.reference(l, pi, path)
						}
					}
				}
			}
		}
		f := e.match(ctx)
		if len(f.Changes) == 0 {
			return nil
		}
		if old := merged[ctx.name]; old != nil {
			for _, c := range f.Changes {
				old.Changes = uniqueChange(old.Changes, c)
			}
			for _, s := range f.Notes {
				old.Notes = appendUnique(old.Notes, s)
			}
			for _, s := range f.Functions {
				old.Functions = appendUnique(old.Functions, s)
			}
			for _, s := range f.NewMetrics {
				old.NewMetrics = appendUnique(old.NewMetrics, s)
			}
			old.Status = combine(old.Status, f.Status)
		} else {
			merged[ctx.name] = &f
		}
		return nil
	})
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f := *merged[name]
		sort.Strings(f.Functions)
		result.Findings = append(result.Findings, f)
		result.Status = combine(result.Status, f.Status)
	}
	if !result.Complete {
		result.Status = combine(result.Status, Review)
	}
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
func uniqueChange(values []Change, c Change) []Change {
	for i, v := range values {
		if v.Kind == c.Kind && v.From == c.From && v.To == c.To {
			if c.Status == RequiresChange && v.Status != RequiresChange {
				values[i] = c
			}
			return values
		}
	}
	return append(values, c)
}

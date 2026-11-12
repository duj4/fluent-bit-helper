package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// Analyze accepts only the two supported exports, in memory. Malformed inputs
// are issues, not empty successful analyses, so partial failures remain visible.
func (e *Engine) Analyze(dashboards, alerts []byte) Report {
	r := Report{Source: e.source, Target: e.target, Reference: e.reference, Complete: true, Status: NoImpact, Dashboards: []DashboardResult{}, Alerts: []AlertResult{}, Issues: []Issue{}}
	if len(dashboards) == 0 && len(alerts) == 0 {
		r.issue("input", "", "Upload at least one JSON export.")
		return r
	}
	if len(dashboards) > 0 {
		e.dashboards(&r, dashboards)
	}
	if len(alerts) > 0 {
		e.alerts(&r, alerts)
	}
	for _, d := range r.Dashboards {
		for _, p := range d.Panels {
			for _, q := range p.Queries {
				r.countQuery(q)
			}
		}
	}
	for _, a := range r.Alerts {
		for _, q := range a.Queries {
			r.countQuery(q)
		}
	}
	if r.Stats.Queries == 0 && r.Stats.SkippedQueries > 0 {
		r.issue("input", "", "No supported Prometheus queries were available; skipped data sources or expression-only inputs are not covered.")
	}
	if !r.Complete {
		r.Status = combine(r.Status, Review)
	}
	return r
}

func (r *Report) issue(input, resource, message string) {
	r.Complete = false
	r.Status = combine(r.Status, Review)
	r.Issues = append(r.Issues, Issue{Input: input, Resource: resource, Message: message})
	r.Stats.Review++
}
func (r *Report) countQuery(q QueryResult) {
	r.Stats.Queries++
	if !q.Complete {
		r.Complete = false
	}
	r.Status = combine(r.Status, q.Status)
	for _, f := range q.Findings {
		if f.Status == RequiresChange {
			r.Stats.RequiresChange++
		}
		for _, c := range f.Changes {
			if c.Status == Review {
				r.Stats.Review++
				break
			}
		}
	}
	if !q.Complete {
		r.Stats.Review++
	}
}

type datasource struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

func sourceType(raw json.RawMessage) (string, string) {
	var d datasource
	if json.Unmarshal(raw, &d) == nil {
		return strings.ToLower(d.Type), d.UID
	}
	var uid string
	_ = json.Unmarshal(raw, &uid)
	return "", uid
}
func nonPrometheus(raw json.RawMessage) bool {
	t, uid := sourceType(raw)
	return expressionSource(t, uid) || (t != "" && t != "prometheus")
}
func expressionSource(t, uid string) bool {
	return t == "__expr__" || uid == "__expr__" || uid == "-100"
}

type panelInput struct {
	ID         int64             `json:"id"`
	Title      string            `json:"title"`
	Panels     []json.RawMessage `json:"panels"`
	Datasource json.RawMessage   `json:"datasource"`
	Targets    []json.RawMessage `json:"targets"`
}

func (e *Engine) dashboards(r *Report, data []byte) {
	var list struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Continue string `json:"continue"`
		} `json:"metadata"`
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		r.issue("dashboards", "", "Invalid Dashboard JSON; export a Kubernetes-style DashboardList.")
		return
	}
	if list.Kind != "DashboardList" || list.Items == nil {
		r.issue("dashboards", "", "Unsupported Dashboard format: expected kind DashboardList with items[].metadata.name and items[].spec.")
		return
	}
	if list.Metadata.Continue != "" {
		r.issue("dashboards", "", "metadata.continue is non-empty. This export is paginated; upload all pages before claiming complete coverage.")
	}
	for i, raw := range list.Items {
		var item struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec *struct {
				Title  string            `json:"title"`
				Panels []json.RawMessage `json:"panels"`
			} `json:"spec"`
		}
		if err := json.Unmarshal(raw, &item); err != nil || item.Spec == nil || item.Metadata.Name == "" || item.Spec.Title == "" || item.Spec.Panels == nil {
			r.issue("dashboards", fmt.Sprintf("item %d", i+1), "Dashboard is missing metadata.name, spec.title or a supported spec.panels array.")
			continue
		}
		d := DashboardResult{UID: item.Metadata.Name, Title: item.Spec.Title, Panels: []PanelResult{}}
		e.panels(r, &d, item.Spec.Panels, 0)
		r.Dashboards = append(r.Dashboards, d)
		r.Stats.Dashboards++
	}
}

func (e *Engine) panels(r *Report, d *DashboardResult, rawPanels []json.RawMessage, depth int) {
	if depth > 32 {
		r.issue("dashboards", d.UID, "Nested panels exceed the supported depth.")
		return
	}
	for _, raw := range rawPanels {
		var input panelInput
		if err := json.Unmarshal(raw, &input); err != nil || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
			r.issue("dashboards", d.UID, "A panel has an unsupported structure.")
			continue
		}
		r.Stats.Panels++
		p := PanelResult{ID: input.ID, Title: input.Title, Queries: []QueryResult{}}
		for _, rawTarget := range input.Targets {
			var target struct {
				RefID      string          `json:"refId"`
				Expr       *string         `json:"expr"`
				Datasource json.RawMessage `json:"datasource"`
			}
			if err := json.Unmarshal(rawTarget, &target); err != nil {
				r.issue("dashboards", d.UID, "A panel target is malformed.")
				continue
			}
			ds := target.Datasource
			if len(ds) == 0 {
				ds = input.Datasource
			}
			if nonPrometheus(ds) {
				r.Stats.SkippedQueries++
				t, uid := sourceType(ds)
				if expressionSource(t, uid) {
					r.issue("dashboards", d.UID, fmt.Sprintf("Panel %d target %s is a Grafana expression; its expression body and dependencies are not covered by Dashboard analysis.", input.ID, target.RefID))
				}
				continue
			}
			if target.Expr == nil || strings.TrimSpace(*target.Expr) == "" {
				r.issue("dashboards", d.UID, fmt.Sprintf("Panel %d target %s has no supported PromQL expr.", input.ID, target.RefID))
				continue
			}
			if r.parsedQueries >= 5000 {
				r.issue("dashboards", d.UID, "Query limit reached (5000); split the export.")
				if len(p.Queries) > 0 {
					d.Panels = append(d.Panels, p)
				}
				return
			}
			r.parsedQueries++
			if target.RefID == "" {
				r.issue("dashboards", d.UID, fmt.Sprintf("Panel %d query has no RefID; its location is incomplete.", input.ID))
			}
			p.Queries = append(p.Queries, e.analyzeQuery(target.RefID, *target.Expr))
		}
		if len(p.Queries) > 0 {
			d.Panels = append(d.Panels, p)
		}
		if len(input.Panels) > 0 {
			e.panels(r, d, input.Panels, depth+1)
		}
	}
}

func (e *Engine) alerts(r *Report, data []byte) {
	var rules []json.RawMessage
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("[")) || json.Unmarshal(data, &rules) != nil {
		r.issue("alert_rules", "", "Unsupported or invalid Alert JSON: expected the provisioning /api/v1/provisioning/alert-rules array.")
		return
	}
	for i, raw := range rules {
		var input struct {
			UID       string            `json:"uid"`
			Title     string            `json:"title"`
			FolderUID string            `json:"folderUID"`
			RuleGroup string            `json:"ruleGroup"`
			Condition string            `json:"condition"`
			Data      []json.RawMessage `json:"data"`
		}
		if json.Unmarshal(raw, &input) != nil || input.UID == "" || input.Title == "" || len(input.Data) == 0 {
			r.issue("alert_rules", fmt.Sprintf("item %d", i+1), "Alert is missing uid, title or data queries.")
			continue
		}
		a := AlertResult{UID: input.UID, Title: input.Title, FolderUID: input.FolderUID, RuleGroup: input.RuleGroup, Condition: input.Condition, Queries: []QueryResult{}}
		for _, rawDatum := range input.Data {
			var datum struct {
				RefID         string `json:"refId"`
				DatasourceUID string `json:"datasourceUid"`
				Model         struct {
					Expr       string          `json:"expr"`
					Expression string          `json:"expression"`
					Type       string          `json:"type"`
					Datasource json.RawMessage `json:"datasource"`
					Reducer    string          `json:"reducer"`
					Conditions json.RawMessage `json:"conditions"`
				} `json:"model"`
			}
			if json.Unmarshal(rawDatum, &datum) != nil {
				r.issue("alert_rules", input.UID, "An alert data query is malformed.")
				continue
			}
			t, uid := sourceType(datum.Model.Datasource)
			expression := expressionSource(t, uid) || expressionSource("", datum.DatasourceUID) || slicesExpression(datum.Model.Type)
			if datum.RefID == "" {
				r.issue("alert_rules", input.UID, "An alert data entry has no RefID; expression dependencies are incomplete.")
			}
			if expression {
				if len(datum.Model.Expression) > 32768 {
					r.issue("alert_rules", input.UID, "A Grafana expression exceeds the 32 KiB analysis limit.")
					continue
				}
				a.Expressions = append(a.Expressions, ExpressionContext{RefID: datum.RefID, Type: datum.Model.Type, Expression: datum.Model.Expression, Reducer: datum.Model.Reducer, Conditions: datum.Model.Conditions})
				r.Stats.SkippedQueries++
				if !slicesExpression(datum.Model.Type) {
					r.issue("alert_rules", input.UID, fmt.Sprintf("Unsupported Grafana expression type %q at RefID %s; its expression and dependencies are not covered.", datum.Model.Type, datum.RefID))
					continue
				}
				var classic []json.RawMessage
				validClassic := strings.EqualFold(datum.Model.Type, "classic_conditions") && json.Unmarshal(datum.Model.Conditions, &classic) == nil && len(classic) > 0
				if strings.TrimSpace(datum.Model.Expression) == "" && !validClassic {
					r.issue("alert_rules", input.UID, "A Grafana expression has no supported expression body or dependencies; its chain is incomplete.")
				}
				continue
			}
			if t != "" && t != "prometheus" {
				r.Stats.SkippedQueries++
				continue
			}
			if strings.TrimSpace(datum.Model.Expr) == "" {
				r.issue("alert_rules", input.UID, "A non-expression data query has no supported PromQL expr.")
				continue
			}
			if r.parsedQueries >= 5000 {
				r.issue("alert_rules", input.UID, "Query limit reached (5000); split the export.")
				r.Alerts = append(r.Alerts, a)
				r.Stats.AlertRules++
				return
			}
			r.parsedQueries++
			a.Queries = append(a.Queries, e.analyzeQuery(datum.RefID, datum.Model.Expr))
		}
		if len(a.Queries) == 0 && len(a.Expressions) > 0 {
			r.issue("alert_rules", input.UID, "Only Grafana expressions are present; their Prometheus query dependencies cannot be checked.")
		}
		refs := map[string]bool{}
		for _, q := range a.Queries {
			refs[q.RefID] = true
		}
		for _, x := range a.Expressions {
			if slicesExpression(x.Type) {
				refs[x.RefID] = true
			}
		}
		if input.Condition != "" && !refs[input.Condition] {
			r.issue("alert_rules", input.UID, "The condition RefID is missing or its data source could not be analyzed.")
		}
		for _, x := range a.Expressions {
			if !slicesExpression(x.Type) {
				continue
			}
			var dependencies []string
			switch strings.ToLower(x.Type) {
			case "math":
				var valid bool
				dependencies, valid = mathExpressionRefs(x.Expression)
				if !valid {
					r.issue("alert_rules", input.UID, fmt.Sprintf("Grafana Math expression at RefID %s has an invalid or incomplete RefID reference; its dependencies could not be fully parsed.", x.RefID))
				}
			case "reduce", "threshold", "resample":
				// These input fields contain a RefID, not a Math expression.
				// Preserve the entire name, including whitespace, for exact lookup.
				if x.Expression != "" {
					dependencies = append(dependencies, x.Expression)
				}
			}
			var conditions []struct {
				Query struct {
					Params []json.RawMessage `json:"params"`
				} `json:"query"`
			}
			if json.Unmarshal(x.Conditions, &conditions) == nil {
				for _, condition := range conditions {
					if len(condition.Query.Params) > 0 {
						var ref string
						if json.Unmarshal(condition.Query.Params[0], &ref) == nil && ref != "" {
							dependencies = append(dependencies, ref)
						}
					}
				}
			}
			for _, ref := range dependencies {
				if !refs[ref] {
					r.issue("alert_rules", input.UID, fmt.Sprintf("Expression RefID %s references a missing or unsupported query RefID: %s", x.RefID, ref))
				}
			}
		}
		if len(a.Expressions) > 0 {
			for qi := range a.Queries {
				for fi := range a.Queries[qi].Findings {
					f := &a.Queries[qi].Findings[fi]
					for _, c := range f.Changes {
						if c.Kind == "unit_change" {
							f.Changes = uniqueChange(f.Changes, Change{Kind: "alert_threshold", Status: Review, Description: "Review the linked Grafana Math/Reduce/Threshold expression chain and condition RefID; threshold conversions are not inferred or applied."})
							break
						}
					}
				}
			}
		}
		r.Alerts = append(r.Alerts, a)
		r.Stats.AlertRules++
	}
}

// mathExpressionRefs scans dependencies only; it does not evaluate Math or
// validate its operators/functions. Grafana variables use Unicode letters,
// digits and underscores, with whitespace additionally allowed inside ${...}.
// See Grafana's pkg/expr/mathexp/parse/lex.go (lexVar and isVarchar).
func mathExpressionRefs(expression string) ([]string, bool) {
	chars := []rune(expression)
	var refs []string
	valid := true
	variableChar := func(r rune) bool {
		return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
	}
	for i := 0; i < len(chars); {
		switch chars[i] {
		case '"':
			// Dollar signs inside Grafana string tokens are not dependencies.
			i++
			for i < len(chars) && chars[i] != '"' {
				i++
			}
			if i == len(chars) {
				return refs, false
			}
			i++
		case '$':
			i++
			if i < len(chars) && chars[i] == '{' {
				i++
				start, supported := i, true
				for i < len(chars) && chars[i] != '}' {
					supported = supported && (variableChar(chars[i]) || unicode.IsSpace(chars[i]))
					i++
				}
				if i == len(chars) {
					return refs, false
				}
				if i == start || !supported {
					valid = false
				} else {
					refs = append(refs, string(chars[start:i]))
				}
				i++
				continue
			}
			start := i
			for i < len(chars) && variableChar(chars[i]) {
				i++
			}
			if i == start {
				valid = false
				continue
			}
			refs = append(refs, string(chars[start:i]))
			if i < len(chars) && !unicode.IsSpace(chars[i]) && !strings.ContainsRune("+-*/%!=<>&|(),", chars[i]) {
				valid = false
			}
		case '{', '}':
			valid = false
			i++
		default:
			i++
		}
	}
	return refs, valid
}

func slicesExpression(t string) bool {
	switch strings.ToLower(t) {
	case "math", "reduce", "threshold", "classic_conditions", "resample":
		return true
	}
	return false
}

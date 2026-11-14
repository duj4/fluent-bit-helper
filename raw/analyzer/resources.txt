package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Analyze handles the provided inventories in memory, without contacting Grafana.
// Complete reflects input validation and analysis limits within this inventory.
// Metric extraction failures are outside the migration matching scope.
func (e *Engine) Analyze(dashboards, alerts []byte) Report {
	r := Report{SchemaVersion: 2, Source: e.source, Target: e.target, Reference: e.reference, Complete: true, Status: NoImpact,
		Dashboards: []DashboardResult{}, Alerts: []AlertResult{}, Issues: []Issue{}}
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
	if len(r.Dashboards) == 0 && len(r.Alerts) == 0 && len(r.Issues) == 0 {
		r.issue("input", "", "The provided exports contain no Dashboard or Alert inventories; no metric coverage is available.")
	}
	all, matched := map[string]bool{}, map[string]bool{}
	count := func(metrics []string, findings []Finding, coverage []CoverageNotice) {
		for _, m := range metrics {
			all[m] = true
		}
		for _, f := range findings {
			matched[f.Metric] = true
			r.Stats.MetricFindings++
			r.Status = combine(r.Status, f.Status)
			for _, c := range f.Changes {
				if c.Status == RequiresChange {
					r.Stats.RequiresChange++
				} else if c.Status == Review {
					r.Stats.Review++
				}
			}
		}
		for _, n := range coverage {
			if n.Uncertain {
				r.Complete = false
				r.Stats.CoverageWarnings += n.Count
			}
		}
	}
	for _, d := range r.Dashboards {
		count(d.Metrics, d.Findings, d.Coverage)
		if len(d.Findings) > 0 {
			r.Stats.AffectedDashboards++
		}
	}
	for _, a := range r.Alerts {
		count(a.Metrics, a.Findings, a.Coverage)
		if len(a.Findings) > 0 {
			r.Stats.AffectedAlerts++
		}
	}
	r.Stats.UniqueMetrics = len(all)
	r.Stats.MatchedMetrics = len(matched)
	if !r.Complete {
		r.Status = combine(r.Status, Review)
	}
	return r
}

func (r *Report) issue(input, resource, message string) {
	r.Complete = false
	r.Status = combine(r.Status, Review)
	r.Stats.CoverageWarnings++
	r.Issues = append(r.Issues, Issue{Input: input, Resource: resource, Message: message})
}

func notice(notices *[]CoverageNotice, kind, message string, uncertain bool) {
	for i := range *notices {
		n := &(*notices)[i]
		if n.Kind == kind && n.Message == message && n.Uncertain == uncertain {
			n.Count++
			return
		}
	}
	*notices = append(*notices, CoverageNotice{Kind: kind, Message: message, Count: 1, Uncertain: uncertain})
}

func (e *Engine) dashboards(r *Report, data []byte) {
	var input struct {
		Dashboards []json.RawMessage `json:"dashboards"`
	}
	if json.Unmarshal(data, &input) != nil || input.Dashboards == nil {
		r.issue("dashboards", "", "Unsupported or invalid Dashboard JSON: expected Mimirtool JSON with dashboards[] (uid, title, metrics).")
		return
	}
	for i, raw := range input.Dashboards {
		var item struct {
			UID     string          `json:"uid"`
			Title   string          `json:"title"`
			Metrics json.RawMessage `json:"metrics"`
		}
		if json.Unmarshal(raw, &item) != nil || item.UID == "" || item.Title == "" {
			r.issue("dashboards", fmt.Sprintf("item %d", i+1), "Dashboard is missing a valid uid or title.")
			continue
		}
		d := DashboardResult{UID: item.UID, Title: item.Title, Metrics: []string{}, Findings: []Finding{}, Coverage: []CoverageNotice{}}
		var rawMetrics []json.RawMessage
		if len(item.Metrics) == 0 || json.Unmarshal(item.Metrics, &rawMetrics) != nil {
			notice(&d.Coverage, "invalid_metrics", "Dashboard metrics is missing or is not an array of metric names.", true)
		} else {
			names := map[string]bool{}
			for _, rawMetric := range rawMetrics {
				var name string
				if json.Unmarshal(rawMetric, &name) != nil || !metricName.MatchString(name) {
					notice(&d.Coverage, "invalid_metric_name", "An inventory entry is not an exact metric name and was not matched.", true)
					continue
				}
				names[name] = true
			}
			d.Metrics = sortedNames(names)
		}
		d.Findings, d.Status = e.findings(d.Metrics)
		r.Dashboards = append(r.Dashboards, d)
		r.Stats.Dashboards++
	}
}

type datasource struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

func sourceType(raw json.RawMessage) (string, string) {
	var d datasource
	if json.Unmarshal(raw, &d) == nil {
		return strings.ToLower(strings.TrimSpace(d.Type)), d.UID
	}
	var uid string
	_ = json.Unmarshal(raw, &uid)
	return "", uid
}
func expressionSource(t, uid string) bool {
	return t == "__expr__" || uid == "__expr__" || uid == "-100"
}
func expressionType(t string) bool {
	switch strings.ToLower(t) {
	case "reduce", "math", "threshold", "classic_conditions", "resample":
		return true
	}
	return false
}

type alertDatum struct {
	RefID         string `json:"refId"`
	DatasourceUID string `json:"datasourceUid"`
	Model         *struct {
		Expr       string          `json:"expr"`
		Type       string          `json:"type"`
		Datasource json.RawMessage `json:"datasource"`
	} `json:"model"`
}

func (d alertDatum) expression() bool {
	if d.Model == nil {
		return expressionSource("", d.DatasourceUID)
	}
	t, uid := sourceType(d.Model.Datasource)
	return expressionSource(t, uid) || expressionSource("", d.DatasourceUID) || expressionType(d.Model.Type)
}

type alertInput struct {
	UID   string            `json:"uid"`
	Title string            `json:"title"`
	Data  []json.RawMessage `json:"data"`
}

func (e *Engine) alerts(r *Report, data []byte) {
	var rawRules []json.RawMessage
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("[")) || json.Unmarshal(data, &rawRules) != nil {
		r.issue("alert_rules", "", "Unsupported or invalid Alert JSON: expected the provisioning /api/v1/provisioning/alert-rules array.")
		return
	}
	for i, raw := range rawRules {
		var input alertInput
		if json.Unmarshal(raw, &input) != nil || input.UID == "" || input.Title == "" || input.Data == nil {
			r.issue("alert_rules", fmt.Sprintf("item %d", i+1), "Alert is missing a valid uid, title or data array.")
			continue
		}
		a := AlertResult{UID: input.UID, Title: input.Title,
			Metrics: []string{}, Findings: []Finding{}, Coverage: []CoverageNotice{}}
		names := map[string]bool{}
		for _, rawDatum := range input.Data {
			var d alertDatum
			if json.Unmarshal(rawDatum, &d) != nil {
				notice(&a.Coverage, "invalid_target", "An alert target has an unsupported structure.", true)
				continue
			}
			// Exclude Grafana expressions before attempting PromQL extraction.
			if d.expression() {
				continue
			}
			if d.Model == nil {
				notice(&a.Coverage, "invalid_target", "An alert target is missing its query model.", true)
				continue
			}
			t, _ := sourceType(d.Model.Datasource)
			if t != "" && t != "prometheus" {
				continue
			}
			if strings.TrimSpace(d.Model.Expr) == "" {
				continue
			}
			if r.Stats.ParsedQueries >= 5000 {
				notice(&a.Coverage, "query_limit", "PromQL extraction limit (5000 targets) reached; split the export.", true)
				break
			}
			r.Stats.ParsedQueries++
			// Unknown sources may contain PromQL too. Match exact names from the
			// existing parser and silently ignore anything it cannot extract.
			metrics, _ := extractMetrics(d.Model.Expr)
			for _, m := range metrics {
				names[m] = true
			}
		}
		a.Metrics = sortedNames(names)
		a.Findings, a.Status = e.findings(a.Metrics)
		r.Alerts = append(r.Alerts, a)
		r.Stats.AlertRules++
	}
}

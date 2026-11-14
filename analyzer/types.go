// Package analyzer inventories metric names and matches the frozen upgrade rules.
package analyzer

type Status string

const (
	RequiresChange Status = "requires_change"
	Review         Status = "review"
	NoImpact       Status = "no_impact"
)

// A Change is a static rule notice. Only unconditional metric identity changes
// require a change; labels, units, types and conditional rules need manual checks.
type Change struct {
	Kind        string            `json:"kind"`
	Status      Status            `json:"status"`
	RuleMetric  string            `json:"rule_metric"`
	From        string            `json:"from,omitempty"`
	To          string            `json:"to,omitempty"`
	Label       string            `json:"label,omitempty"`
	Factor      *float64          `json:"factor,omitempty"`
	Conditional bool              `json:"conditional,omitempty"`
	Conditions  map[string]string `json:"conditions,omitempty"`
}

type Finding struct {
	Metric           string   `json:"metric"`
	Status           Status   `json:"status"`
	NewMetrics       []string `json:"new_metrics,omitempty"`
	ReplacementQuery string   `json:"replacement_query,omitempty"`
	Changes          []Change `json:"changes"`
	Notes            []string `json:"notes,omitempty"`
}

// Count is the number of input events summarized by a notice. Informational
// unsupported panels and skipped data sources do not become migration reviews.
type CoverageNotice struct {
	Kind      string `json:"kind"`
	Message   string `json:"message"`
	Count     int    `json:"count"`
	Uncertain bool   `json:"uncertain"`
}

type DashboardResult struct {
	UID      string           `json:"uid"`
	Title    string           `json:"title"`
	Status   Status           `json:"status"`
	Metrics  []string         `json:"metrics"`
	Findings []Finding        `json:"findings"`
	Coverage []CoverageNotice `json:"coverage"`
}

type AlertResult struct {
	UID      string           `json:"uid"`
	Title    string           `json:"title"`
	Status   Status           `json:"status"`
	Metrics  []string         `json:"metrics"`
	Findings []Finding        `json:"findings"`
	Coverage []CoverageNotice `json:"coverage"`
}

type Issue struct {
	Input    string `json:"input"`
	Resource string `json:"resource,omitempty"`
	Message  string `json:"message"`
}

// Global metric counts are unique names across both inputs. Change/notice counts
// are individual rule items per resource; a repeated resource/metric is counted once.
type Stats struct {
	Dashboards         int `json:"dashboards"`
	AlertRules         int `json:"alert_rules"`
	UniqueMetrics      int `json:"unique_metrics"`
	AffectedDashboards int `json:"affected_dashboards"`
	AffectedAlerts     int `json:"affected_alerts"`
	MatchedMetrics     int `json:"matched_metrics"`
	MetricFindings     int `json:"metric_findings"`
	RequiresChange     int `json:"requires_change"`
	Review             int `json:"review"`
	CoverageWarnings   int `json:"coverage_warnings"`
	ParsedQueries      int `json:"parsed_queries"`
}

type Report struct {
	SchemaVersion int               `json:"schema_version"`
	Source        string            `json:"source"`
	Target        string            `json:"target"`
	Reference     string            `json:"reference"`
	Complete      bool              `json:"complete"`
	Status        Status            `json:"status"`
	Stats         Stats             `json:"stats"`
	Dashboards    []DashboardResult `json:"dashboards"`
	Alerts        []AlertResult     `json:"alerts"`
	Issues        []Issue           `json:"issues"`
}

func combine(a, b Status) Status {
	if a == RequiresChange || b == RequiresChange {
		return RequiresChange
	}
	if a == Review || b == Review {
		return Review
	}
	return NoImpact
}

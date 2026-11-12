// Package analyzer performs read-only impact analysis for the reviewed Windows exporter upgrade.
package analyzer

import "encoding/json"

type Status string

const (
	RequiresChange Status = "requires_change"
	Review         Status = "review"
	NoImpact       Status = "no_impact"
)

type Change struct {
	Kind        string   `json:"kind"`
	Status      Status   `json:"status"`
	From        string   `json:"from,omitempty"`
	To          string   `json:"to,omitempty"`
	Factor      *float64 `json:"factor,omitempty"`
	Description string   `json:"description"`
}

type Finding struct {
	Metric           string   `json:"metric"`
	Status           Status   `json:"status"`
	NewMetrics       []string `json:"new_metrics,omitempty"`
	ReplacementQuery string   `json:"replacement_query,omitempty"`
	Functions        []string `json:"functions,omitempty"`
	Changes          []Change `json:"changes"`
	Notes            []string `json:"notes,omitempty"`
}

type QueryResult struct {
	RefID    string    `json:"ref_id"`
	Expr     string    `json:"expr"`
	Status   Status    `json:"status"`
	Complete bool      `json:"complete"`
	Findings []Finding `json:"findings"`
	Issues   []string  `json:"issues,omitempty"`
}

type PanelResult struct {
	ID      int64         `json:"id"`
	Title   string        `json:"title"`
	Queries []QueryResult `json:"queries"`
}

type DashboardResult struct {
	UID    string        `json:"uid"`
	Title  string        `json:"title"`
	Panels []PanelResult `json:"panels"`
}

type ExpressionContext struct {
	RefID      string          `json:"ref_id"`
	Type       string          `json:"type"`
	Expression string          `json:"expression,omitempty"`
	Reducer    string          `json:"reducer,omitempty"`
	Conditions json.RawMessage `json:"conditions,omitempty"`
}

type AlertResult struct {
	UID         string              `json:"uid"`
	Title       string              `json:"title"`
	FolderUID   string              `json:"folder_uid,omitempty"`
	RuleGroup   string              `json:"rule_group,omitempty"`
	Condition   string              `json:"condition,omitempty"`
	Queries     []QueryResult       `json:"queries"`
	Expressions []ExpressionContext `json:"expressions,omitempty"`
}

type Issue struct {
	Input    string `json:"input"`
	Resource string `json:"resource,omitempty"`
	Message  string `json:"message"`
}

type Stats struct {
	Dashboards     int `json:"dashboards"`
	Panels         int `json:"panels"`
	AlertRules     int `json:"alert_rules"`
	Queries        int `json:"queries"`
	SkippedQueries int `json:"skipped_queries"`
	RequiresChange int `json:"requires_change"`
	Review         int `json:"review"`
}

type Report struct {
	Source        string            `json:"source"`
	Target        string            `json:"target"`
	Reference     string            `json:"reference"`
	Complete      bool              `json:"complete"`
	Status        Status            `json:"status"`
	Stats         Stats             `json:"stats"`
	Dashboards    []DashboardResult `json:"dashboards"`
	Alerts        []AlertResult     `json:"alerts"`
	Issues        []Issue           `json:"issues"`
	parsedQueries int
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

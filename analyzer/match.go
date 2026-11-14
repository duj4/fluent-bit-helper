package analyzer

import (
	"slices"
	"sort"
	"strconv"
	"strings"
)

func patternMatch(r Rule, name string) bool {
	return !slices.Contains(r.ExcludeMetrics, name) && (r.Metric == name ||
		(strings.HasSuffix(r.Metric, "*") && strings.HasPrefix(name, strings.TrimSuffix(r.Metric, "*"))))
}

// matchMetric composes exact/alias rules and family notices using only names.
// No selector, label origin, function, threshold or expression context is used.
func (e *Engine) matchMetric(name string) Finding {
	f := Finding{Metric: name, Status: NoImpact, Changes: []Change{}}
	identities := []string{name}
	for _, r := range e.rules {
		if r.Metric == name || slices.Contains(r.MetricAliases, name) {
			identities = appendUnique(identities, r.Metric)
			if r.NewMetric != "" {
				identities = appendUnique(identities, r.NewMetric)
			}
		}
		if r.NewMetric == name {
			identities = appendUnique(identities, r.Metric)
		}
	}
	for _, r := range e.rules {
		legacy := r.Metric == name || slices.Contains(r.MetricAliases, name)
		matched := legacy
		if strings.HasSuffix(r.Metric, "*") {
			for _, id := range identities {
				matched = matched || patternMatch(r, id)
			}
			if slices.Contains(r.ExcludeMetrics, name) {
				matched = false
			}
		}
		if !matched {
			continue
		}
		before := len(f.Changes)
		add := func(c Change) {
			c.RuleMetric = r.Metric
			if len(r.LabelMatchers) > 0 {
				c.Conditional = true
				c.Status = Review
				c.Conditions = map[string]string{}
				for k, v := range r.LabelMatchers {
					c.Conditions[k] = v
				}
			}
			f.Changes = append(f.Changes, c)
			f.Status = combine(f.Status, c.Status)
		}
		if legacy {
			target := r.NewMetric
			if target == "" && slices.Contains(r.MetricAliases, name) && !r.Removed && len(r.NewMetrics) == 0 {
				target = r.Metric
			}
			if target != "" && target != name {
				add(Change{Kind: "metric_rename", Status: RequiresChange, From: name, To: target})
				f.NewMetrics = appendUnique(f.NewMetrics, target)
			}
			if len(r.NewMetrics) > 0 {
				for _, m := range r.NewMetrics {
					f.NewMetrics = appendUnique(f.NewMetrics, m)
				}
				f.ReplacementQuery = r.ReplacementQuery
				add(Change{Kind: "metric_split", Status: RequiresChange, From: name})
			}
			if r.Removed {
				f.ReplacementQuery = r.ReplacementQuery
				add(Change{Kind: "metric_removed", Status: RequiresChange, From: name})
			}
			if u := r.UnitChange; u != nil {
				add(Change{Kind: "unit_change", Status: Review, From: u.From, To: u.To, Factor: u.Factor})
			}
			if t := r.TypeChange; t != nil {
				add(Change{Kind: "type_change", Status: Review, From: t.From, To: t.To})
			}
		}
		keys := make([]string, 0, len(r.LabelRenames))
		for key := range r.LabelRenames {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, old := range keys {
			add(Change{Kind: "label_rename", Status: Review, From: old, To: r.LabelRenames[old]})
		}
		for _, c := range r.ConditionalLabelRenames {
			add(Change{Kind: "conditional_label_rename", Status: Review, From: c.From, To: c.To, Conditional: true,
				Conditions: map[string]string{"label_source": c.When.LabelSource, "honor_labels": strconv.FormatBool(c.When.HonorLabels), "on_false": c.OnFalse, "on_unknown": c.OnUnknown}})
		}
		for _, l := range r.RemovedLabels {
			add(Change{Kind: "label_removed", Status: Review, From: l})
		}
		for _, l := range r.AddedLabels {
			add(Change{Kind: "label_added", Status: Review, To: l})
		}
		for _, v := range r.LabelValueChanges {
			add(Change{Kind: "label_value_change", Status: Review, Label: v.Label, From: v.From, To: v.To})
		}
		// Omit only exact, reviewed boilerplate already expressed by structured
		// changes. Keep every other note intact, including mixed caveat/guidance notes.
		note := r.Note
		switch note {
		case "Adjust query scaling and alert thresholds.":
			if r.UnitChange != nil && r.UnitChange.Factor != nil {
				note = ""
			}
		case "Review rate() or increase() usage for the counter.":
			if r.TypeChange != nil && r.TypeChange.To == "counter" {
				note = ""
			}
		case "Query the gauge directly; remove rate() or increase() where used.":
			if r.TypeChange != nil && r.TypeChange.To == "gauge" {
				note = ""
			}
		case "Adjust query scaling and alert thresholds. Query the gauge directly; remove rate() or increase() where used.",
			"Adjust query scaling and alert thresholds. Review rate() or increase() usage for the counter.":
			if r.UnitChange != nil && r.UnitChange.Factor != nil && r.TypeChange != nil {
				note = ""
			}
		case "Review aggregation and vector matching for the added label.", "Review aggregation and vector matching for the added labels.":
			if len(r.AddedLabels) > 0 {
				note = ""
			}
		}
		if len(f.Changes) > before && note != "" {
			f.Notes = appendUnique(f.Notes, note)
		}
	}
	return f
}

func (e *Engine) findings(metrics []string) ([]Finding, Status) {
	result := []Finding{}
	status := NoImpact
	for _, name := range metrics {
		f := e.matchMetric(name)
		if len(f.Changes) == 0 {
			continue
		}
		status = combine(status, f.Status)
		result = append(result, f)
	}
	return result, status
}

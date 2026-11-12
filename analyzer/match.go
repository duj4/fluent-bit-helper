package analyzer

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
)

func patternMatch(r Rule, name string) bool {
	if slices.Contains(r.ExcludeMetrics, name) {
		return false
	}
	return r.Metric == name || (strings.HasSuffix(r.Metric, "*") && strings.HasPrefix(name, strings.TrimSuffix(r.Metric, "*")))
}

func (e *Engine) match(ctx selectorContext) Finding {
	f := Finding{Metric: ctx.name, Status: NoImpact, Changes: []Change{}}
	for fn := range ctx.functions {
		f.Functions = append(f.Functions, fn)
	}
	// Source, observed alias and target identities allow family label rules to
	// compose with exact renames; a target match never re-applies old units.
	identities := []string{ctx.name}
	for _, r := range e.rules {
		if r.Metric == ctx.name || slices.Contains(r.MetricAliases, ctx.name) {
			identities = appendUnique(identities, r.Metric)
			if r.NewMetric != "" {
				identities = appendUnique(identities, r.NewMetric)
			}
		}
		if r.NewMetric == ctx.name {
			identities = appendUnique(identities, r.Metric)
		}
	}
	add := func(c Change) { f.Changes = uniqueChange(f.Changes, c); f.Status = combine(f.Status, c.Status) }
	for _, r := range e.rules {
		legacy := r.Metric == ctx.name || slices.Contains(r.MetricAliases, ctx.name)
		matched := legacy || r.NewMetric == ctx.name
		if strings.HasSuffix(r.Metric, "*") {
			for _, id := range identities {
				if patternMatch(r, id) {
					matched = true
				}
			}
			// Exclusions apply to the actual identity, even if an alias resolves elsewhere.
			if slices.Contains(r.ExcludeMetrics, ctx.name) {
				matched = false
			}
		}
		if !matched {
			continue
		}
		conditional := false
		excluded := false
		for key, value := range r.LabelMatchers {
			found := false
			for _, m := range ctx.matchers {
				if m.Name != key {
					continue
				}
				found = true
				if strings.Contains(m.Value, "$") {
					conditional = true
					continue
				}
				if !m.Matches(value) {
					excluded = true
				}
				if m.Type != labels.MatchEqual || m.Value != value {
					conditional = true
				}
			}
			if !found {
				conditional = true
			}
		}
		if excluded {
			continue
		}
		level := RequiresChange
		if conditional {
			level = Review
		}
		countBefore := len(f.Changes)
		if legacy {
			target := r.NewMetric
			if target == "" && slices.Contains(r.MetricAliases, ctx.name) && !r.Removed && len(r.NewMetrics) == 0 {
				target = r.Metric
			}
			if target != "" && target != ctx.name {
				add(Change{Kind: "metric_rename", Status: level, From: ctx.name, To: target})
				f.NewMetrics = appendUnique(f.NewMetrics, target)
			}
			if len(r.NewMetrics) > 0 {
				for _, m := range r.NewMetrics {
					f.NewMetrics = appendUnique(f.NewMetrics, m)
				}
				f.ReplacementQuery = r.ReplacementQuery
				add(Change{Kind: "metric_split", Status: RequiresChange, From: ctx.name, Description: "Rule detail: The old value was a running numerator, not a percentage. The suggested ratio of counters is not numerically equivalent."})
			}
			if r.Removed {
				f.ReplacementQuery = r.ReplacementQuery
				description := "Rule detail: Removed; upstream specifies no replacement."
				if r.ReplacementQuery != "" {
					description = "Rule detail: " + r.Note
				}
				if conditional {
					description += " Analyzer advice: Confirm whether this query selects the removed label series."
				}
				add(Change{Kind: "metric_removed", Status: level, From: ctx.name, Description: description})
			}
			if u := r.UnitChange; u != nil {
				status := RequiresChange
				description := "Analyzer advice: Check Grafana display units and alert thresholds."
				if u.Factor == nil {
					status = Review
					description = "Rule detail: No fixed conversion factor is specified. Analyzer advice: Check the old calculation and thresholds."
				}
				if ctx.name == r.NewMetric {
					status = Review
					description = "Analyzer advice: The historical OpenMetrics name equals the new name but the old unit differs. Verify exporter version before converting values or thresholds."
				}
				if conditional {
					status = Review
				}
				add(Change{Kind: "unit_change", Status: status, From: u.From, To: u.To, Factor: u.Factor, Description: description})
			}
		}
		if t := r.TypeChange; t != nil {
			counterFunctionOnGauge := t.To == "gauge" && (ctx.functions["rate"] || ctx.functions["increase"] || ctx.functions["irate"])
			// A migrated name needs type advice only when its current usage is
			// incompatible. Renames and old-unit conversions remain legacy-only.
			if legacy || counterFunctionOnGauge {
				status := Review
				description := "Analyzer advice: Confirm whether the intended counter value is a rate or a period count."
				if t.To == "gauge" {
					description = "Rule detail: Query the gauge directly."
				}
				if counterFunctionOnGauge {
					status = RequiresChange
					description = "Analyzer advice: The new value is a gauge; review or remove rate()/increase()/irate() applied to it."
				}
				if _, detail, ok := strings.Cut(r.Note, "Divide "); ok {
					description = "Rule detail: Divide " + strings.TrimSuffix(detail, " Adjust query scaling and alert thresholds.")
				} else if strings.Contains(r.Note, "Use increase() for the event count.") {
					description = "Rule detail: Use increase() for the event count."
				}
				if conditional {
					status = Review
				}
				add(Change{Kind: "type_change", Status: status, From: t.From, To: t.To, Description: description})
			}
		}
		keys := make([]string, 0, len(r.LabelRenames))
		for k := range r.LabelRenames {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, old := range keys {
			if ctx.labelRefs[old] {
				status := RequiresChange
				description := "Update this label reference in the selector, aggregation or vector matching clause."
				if ctx.labelStatus[old] == Review {
					status = Review
					description = "An intervening aggregation or label function can discard or synthesize this label; review its source before renaming downstream references."
				}
				add(Change{Kind: "label_rename", Status: status, From: old, To: r.LabelRenames[old], Description: description})
			}
		}
		for _, c := range r.ConditionalLabelRenames {
			if ctx.labelRefs[c.From] {
				// Neither label origin nor scrape configuration is available from
				// dashboard/alert exports. Never infer origin from a label value.
				add(Change{Kind: "conditional_label_rename", Status: Review, From: c.From, To: c.To, Description: fmt.Sprintf("Preserve target %s. Only consider %s → %s after confirming label_source=%s and honor_labels=%t; unknown provenance requires review.", c.From, c.From, c.To, c.When.LabelSource, c.When.HonorLabels)})
			}
		}
		for _, l := range r.RemovedLabels {
			if ctx.labelRefs[l] {
				level := RequiresChange
				description := "Rule detail: " + r.Note
				if ctx.labelStatus[l] == Review {
					level = Review
					description = "This label may be query-created or discarded below its reference; review its provenance before applying the exporter label removal."
				}
				add(Change{Kind: "label_removed", Status: level, From: l, Description: description})
			}
		}
		for _, l := range r.AddedLabels {
			if reason := ctx.addedLabelImpact(l); reason != "" {
				if strings.Contains(r.Note, "A process restart starts a new series.") {
					reason += " Rule detail: " + r.Note
				}
				add(Change{Kind: "label_added", Status: Review, To: l, Description: "Analyzer advice: " + reason})
			}
		}
		for _, v := range r.LabelValueChanges {
			used := false
			for _, m := range ctx.matchers {
				if m.Name == "__name__" {
					continue
				}
				if v.Label == "" {
					if m.Name != "instance" && m.Name != "job" {
						used = true
					}
					continue
				}
				if m.Name != v.Label {
					continue
				}
				format := strings.Contains(v.From, "<") || strings.Contains(v.From, "\\") || v.From == "CPU mode" || v.From == "session name"
				if format || strings.Contains(m.Value, "$") || m.Matches(v.From) {
					used = true
				}
			}
			if v.Label != "owner" && (v.From == "CPU mode" || v.From == "session name") && ctx.labelRefs[v.Label] {
				used = true
			}
			if used {
				add(Change{Kind: "label_value_change", Status: Review, From: v.From, To: v.To, Description: "Review the referenced label values or formats against their intended meaning. This is not an unconditional replacement or a guarantee of numerical equivalence."})
			}
		}
		if len(f.Changes) > countBefore && r.Note != "" {
			f.Notes = appendUnique(f.Notes, r.Note)
		}
	}
	return f
}

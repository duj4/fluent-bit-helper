package analyzer

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	migrationrules "fluent-bit-helper/migration-rules"
)

type unitChange struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	Factor *float64 `json:"factor,omitempty"`
}
type typeChange struct {
	From string `json:"from"`
	To   string `json:"to"`
}
type valueChange struct {
	Label string `json:"label"`
	From  string `json:"from"`
	To    string `json:"to"`
}
type conditionalRename struct {
	From string `json:"from"`
	To   string `json:"to"`
	When struct {
		LabelSource string `json:"label_source"`
		HonorLabels bool   `json:"honor_labels"`
	} `json:"when"`
	OnFalse   string `json:"on_false"`
	OnUnknown string `json:"on_unknown"`
}
type Rule struct {
	Metric                  string              `json:"metric"`
	NewMetric               string              `json:"new_metric"`
	MetricAliases           []string            `json:"metric_aliases"`
	NewMetrics              []string            `json:"new_metrics"`
	UnitChange              *unitChange         `json:"unit_change"`
	TypeChange              *typeChange         `json:"type_change"`
	LabelRenames            map[string]string   `json:"label_renames"`
	ConditionalLabelRenames []conditionalRename `json:"conditional_label_renames"`
	LabelValueChanges       []valueChange       `json:"label_value_changes"`
	AddedLabels             []string            `json:"added_labels"`
	RemovedLabels           []string            `json:"removed_labels"`
	ExcludeMetrics          []string            `json:"exclude_metrics"`
	LabelMatchers           map[string]string   `json:"label_matchers"`
	Removed                 bool                `json:"removed"`
	ReplacementQuery        string              `json:"replacement_query"`
	Note                    string              `json:"note"`
}

type Engine struct {
	source    string
	target    string
	reference string
	rules     []Rule
}

// New verifies and loads only the accepted, embedded 467-rule candidate.
func New() (*Engine, error) {
	data := migrationrules.JSON()
	if fmt.Sprintf("%x", sha256.Sum256(data)) != "be0b9901295f57dec444ccd2fb3129fdddb2f50a22cfe7cdbdd08b9771012b4c" {
		return nil, fmt.Errorf("migration rules checksum differs from the reviewed candidate")
	}
	var doc struct {
		Source    string `json:"source"`
		Target    string `json:"target"`
		Reference string `json:"reference"`
		Rules     []Rule `json:"rules"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("load migration rules: %w", err)
	}
	if len(doc.Rules) != 467 || doc.Source != "0.31.x" || doc.Target != "0.32.1" {
		return nil, fmt.Errorf("unexpected migration rules version or count")
	}
	return &Engine{source: doc.Source, target: doc.Target, reference: doc.Reference, rules: doc.Rules}, nil
}

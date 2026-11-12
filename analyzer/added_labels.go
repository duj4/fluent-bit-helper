package analyzer

import (
	"fmt"
	"slices"

	promparser "github.com/prometheus/prometheus/promql/parser"
)

// addedLabelImpact follows only this selector's ancestors, from the selector
// outward. A projection can prove that the added label is neither observed nor
// returned. A matcher, grouping key, implicit join or range operation before
// that projection still needs review. This does not infer sample cardinality or
// evaluate PromQL: safe projections assume the existing series relationships
// remain unique, as they did before the label was introduced.
func (ctx selectorContext) addedLabelImpact(label string) string {
	for _, m := range ctx.matchers {
		if m.Name == label {
			return fmt.Sprintf("The selector filters on added label %q, which was absent in 0.31; confirm the intended selected series.", label)
		}
	}
	output := fmt.Sprintf("The output retains added label %q; confirm that the changed series identity is intended.", label)
	var branch promparser.Node = ctx.selector
	for i := len(ctx.path) - 1; i >= 0; i-- {
		parent := ctx.path[i]
		switch n := parent.(type) {
		case *promparser.ParenExpr, *promparser.UnaryExpr:
			// These wrappers preserve the operand's labels.
		case *promparser.MatrixSelector, *promparser.SubqueryExpr:
			return fmt.Sprintf("Range evaluation occurs before %q is projected out; changed series identity may interrupt sample history across the upgrade.", label)
		case *promparser.AggregateExpr:
			grouped := slices.Contains(n.Grouping, label)
			if grouped && !n.Without {
				return fmt.Sprintf("Aggregation by %q now groups on the added label's values instead of an absent label; confirm the intended groups.", label)
			}
			switch n.Op {
			case promparser.TOPK, promparser.BOTTOMK:
				// Selection aggregators retain the input series' labels, even
				// when their grouping list does not mention the added label.
				output = fmt.Sprintf("%s retains %q on selected output series; confirm the changed series identity.", n.Op, label)
			case promparser.LIMITK, promparser.LIMIT_RATIO:
				return fmt.Sprintf("%s samples series by identity while added label %q is present; confirm which series are selected before any later projection.", n.Op, label)
			case promparser.COUNT_VALUES:
				return fmt.Sprintf("count_values creates output labels while %q is present; confirm grouping and destination-label collisions.", label)
			default:
				if (n.Without && grouped) || (!n.Without && !grouped) {
					return ""
				}
				output = fmt.Sprintf("Aggregation without(...) retains %q and can separate the output into new groups; confirm the intended grouping.", label)
			}
		case *promparser.BinaryExpr:
			vm := n.VectorMatching
			if vm == nil {
				break // Vector/scalar arithmetic preserves vector labels.
			}
			listed := slices.Contains(vm.MatchingLabels, label)
			if vm.On && listed {
				return fmt.Sprintf("Vector matching on %q now uses the added label's values; confirm matching labels on both operands.", label)
			}
			if !vm.On && !listed {
				return fmt.Sprintf("Default or ignoring(...) vector matching includes added label %q; matching series may change. Confirm both operands or specify matching labels explicitly.", label)
			}
			switch {
			case n.Op == promparser.LAND || n.Op == promparser.LUNLESS:
				if branch == n.RHS {
					return "" // RHS contributes membership, not output labels.
				}
			case n.Op == promparser.LOR:
				// Both operands can contribute their original output labels.
			case vm.Card == promparser.CardOneToOne:
				return "" // on(...) keeps only keys; ignoring(label) drops it.
			case (vm.Card == promparser.CardManyToOne && branch == n.RHS) ||
				(vm.Card == promparser.CardOneToMany && branch == n.LHS):
				if !slices.Contains(vm.Include, label) {
					return "" // The one-side label is neither matched nor copied.
				}
				output = fmt.Sprintf("group_left/group_right copies added label %q into the output; confirm that label projection.", label)
			case slices.Contains(vm.Include, label):
				return "" // On the many side, Include overwrites/drops this label using the one side.
			}
		case *promparser.Call:
			switch n.Func.Name {
			case "scalar", "absent":
				return "" // Neither copies arbitrary input labels to its output.
			case "label_join", "label_replace":
				for j, arg := range n.Args {
					if s, ok := arg.(*promparser.StringLiteral); ok && s.Val == label &&
						((n.Func.Name == "label_replace" && j == 3) || (n.Func.Name == "label_join" && j >= 3)) {
						return fmt.Sprintf("%s reads added label %q to construct another label; confirm the projected value.", n.Func.Name, label)
					}
				}
				if destination, ok := n.Args[1].(*promparser.StringLiteral); ok && destination.Val == label {
					if n.Func.Name == "label_join" {
						return "" // Unconditional overwrite, without reading this label.
					}
					return fmt.Sprintf("label_replace conditionally overwrites added label %q; unmatched series may retain the exporter value.", label)
				}
			case "abs", "ceil", "floor", "round", "clamp", "clamp_min", "clamp_max", "sgn",
				"sqrt", "exp", "ln", "log2", "log10", "sort", "sort_desc", "timestamp":
				// A small allowlist of instant functions that preserve labels.
			default:
				return fmt.Sprintf("The effect of added label %q through %s() is not proven by this analysis; confirm its label and series semantics.", label, n.Func.Name)
			}
		default:
			return fmt.Sprintf("The effect of added label %q through this expression is not proven; review its label semantics.", label)
		}
		branch = parent
	}
	return output
}

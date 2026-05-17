package tableformat

import (
	"fmt"

	"github.com/jeduden/mdsmith/internal/lint"
	"github.com/jeduden/mdsmith/internal/rule"
	"github.com/jeduden/mdsmith/internal/rules/settings"
	"github.com/jeduden/mdsmith/internal/rules/tablefmt"
)

func init() {
	rule.Register(&Rule{Pad: 1})
}

// Rule checks that markdown tables are formatted with consistent
// column widths and padding (prettier-style).
type Rule struct {
	Pad             int  // spaces on each side of cell content
	SeparatorSpaced bool // true: | --- | --- |, false (default): |---|---|
}

// ID implements rule.Rule.
func (r *Rule) ID() string { return "MDS025" }

// Name implements rule.Rule.
func (r *Rule) Name() string { return "table-format" }

// Category implements rule.Rule.
func (r *Rule) Category() string { return "table" }

// GetPad returns the current pad setting.
func (r *Rule) GetPad() int { return r.Pad }

// GetSeparatorSpaced returns true when the spaced separator style is active.
func (r *Rule) GetSeparatorSpaced() bool { return r.SeparatorSpaced }

// ApplySettings implements rule.Configurable.
func (r *Rule) ApplySettings(s map[string]any) error {
	for k, v := range s {
		switch k {
		case "pad":
			n, ok := settings.ToInt(v)
			if !ok {
				return fmt.Errorf("table-format: pad must be an integer, got %T", v)
			}
			if n < 0 {
				return fmt.Errorf("table-format: pad must be non-negative, got %d", n)
			}
			r.Pad = n
		case "separator-style":
			sv, ok := v.(string)
			if !ok {
				return fmt.Errorf("table-format: separator-style must be a string, got %T", v)
			}
			switch sv {
			case "compact":
				r.SeparatorSpaced = false
			case "spaced":
				r.SeparatorSpaced = true
			default:
				return fmt.Errorf("table-format: separator-style must be \"compact\" or \"spaced\", got %q", sv)
			}
		default:
			return fmt.Errorf("table-format: unknown setting %q", k)
		}
	}
	return nil
}

// DefaultSettings implements rule.Configurable.
func (r *Rule) DefaultSettings() map[string]any {
	return map[string]any{
		"pad":             1,
		"separator-style": "compact",
	}
}

// Check implements rule.Rule.
func (r *Rule) Check(f *lint.File) []lint.Diagnostic {
	codeLines := lint.CollectCodeBlockLines(f)
	cfg := tablefmt.Config{Pad: r.Pad, SeparatorSpaced: r.SeparatorSpaced}
	var diags []lint.Diagnostic
	for _, v := range tablefmt.Violations(f.Lines, codeLines, cfg) {
		diags = append(diags, lint.Diagnostic{
			File:     f.Path,
			Line:     v.StartLine,
			Column:   1,
			RuleID:   r.ID(),
			RuleName: r.Name(),
			Severity: lint.Warning,
			Message:  v.Message,
		})
	}
	return diags
}

// Fix implements rule.FixableRule.
func (r *Rule) Fix(f *lint.File) []byte {
	codeLines := lint.CollectCodeBlockLines(f)
	cfg := tablefmt.Config{Pad: r.Pad, SeparatorSpaced: r.SeparatorSpaced}
	return tablefmt.FormatLines(f.Source, f.Lines, codeLines, cfg)
}

var _ rule.FixableRule = (*Rule)(nil)
var _ rule.Configurable = (*Rule)(nil)

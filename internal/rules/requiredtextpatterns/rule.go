// Package requiredtextpatterns implements MDS057, which flags
// heading-bounded sections whose body text does not match a configured
// regex.
package requiredtextpatterns

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jeduden/mdsmith/internal/lint"
	"github.com/jeduden/mdsmith/internal/mdtext"
	"github.com/jeduden/mdsmith/internal/rule"
	"github.com/jeduden/mdsmith/internal/rules/astutil"
	"github.com/yuin/goldmark/ast"
)

func init() {
	rule.Register(&Rule{})
}

// Rule flags heading-bounded sections whose plain text does not match a
// configured regular expression. Walks every heading in the document
// and, for each one, gathers the prose under the heading (paragraphs,
// including those in nested sub-sections) and tests each configured
// pattern against the gathered text. A failing pattern emits one
// diagnostic anchored at the heading line so the per-scope override
// from plan 146 keeps only diagnostics that fall inside the configured
// scope's line range.
type Rule struct {
	Patterns []Pattern
}

// Pattern is a compiled required-text pattern with optional message
// and skip-indices.
type Pattern struct {
	Source      string
	Regex       *regexp.Regexp
	Message     string
	SkipIndices []int
}

// ID implements rule.Rule.
func (r *Rule) ID() string { return "MDS057" }

// Name implements rule.Rule.
func (r *Rule) Name() string { return "required-text-patterns" }

// Category implements rule.Rule.
func (r *Rule) Category() string { return "meta" }

// EnabledByDefault implements rule.Defaultable.
func (r *Rule) EnabledByDefault() bool { return false }

// Check implements rule.Rule.
func (r *Rule) Check(f *lint.File) []lint.Diagnostic {
	if f.AST == nil || len(r.Patterns) == 0 {
		return nil
	}
	headings := collectHeadings(f)
	paragraphs := collectParagraphs(f)
	if len(headings) == 0 {
		return nil
	}

	totalLines := len(f.Lines)
	if totalLines > 0 && len(f.Lines[totalLines-1]) == 0 {
		totalLines--
	}

	var diags []lint.Diagnostic
	for i, h := range headings {
		end := scopeEnd(headings, i, totalLines)
		body := sectionBody(paragraphs, h.line, end)
		for _, p := range r.Patterns {
			if p.Regex == nil {
				continue
			}
			if p.Regex.MatchString(body) {
				continue
			}
			diags = append(diags, lint.Diagnostic{
				File:     f.Path,
				Line:     h.line,
				Column:   1,
				RuleID:   r.ID(),
				RuleName: r.Name(),
				Severity: lint.Warning,
				Message:  formatMessage(p),
			})
		}
	}
	return diags
}

func formatMessage(p Pattern) string {
	if p.Message != "" {
		return fmt.Sprintf("required text missing: %s", p.Message)
	}
	return fmt.Sprintf("required text pattern not matched: %s", p.Source)
}

// ApplySettings implements rule.Configurable.
func (r *Rule) ApplySettings(s map[string]any) error {
	for k, v := range s {
		switch k {
		case "patterns":
			ps, err := parsePatterns(v)
			if err != nil {
				return err
			}
			r.Patterns = ps
		default:
			return fmt.Errorf(
				"required-text-patterns: unknown setting %q", k,
			)
		}
	}
	return nil
}

// DefaultSettings implements rule.Configurable.
func (r *Rule) DefaultSettings() map[string]any {
	return map[string]any{
		"patterns": []any{},
	}
}

func parsePatterns(v any) ([]Pattern, error) {
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf(
			"required-text-patterns: patterns must be a list, got %T", v,
		)
	}
	out := make([]Pattern, 0, len(items))
	for i, item := range items {
		m, err := asStringMap(item)
		if err != nil {
			return nil, fmt.Errorf(
				"required-text-patterns: patterns[%d] %w", i, err,
			)
		}
		patternStr, _ := m["pattern"].(string)
		if patternStr == "" {
			return nil, fmt.Errorf(
				"required-text-patterns: patterns[%d].pattern must be a non-empty string",
				i,
			)
		}
		re, err := regexp.Compile(patternStr)
		if err != nil {
			return nil, fmt.Errorf(
				"required-text-patterns: patterns[%d].pattern invalid: %w", i, err,
			)
		}
		msg, _ := m["message"].(string)
		skip, err := parseSkipIndices(m["skip-indices"])
		if err != nil {
			return nil, fmt.Errorf(
				"required-text-patterns: patterns[%d].skip-indices %w", i, err,
			)
		}
		out = append(out, Pattern{
			Source:      patternStr,
			Regex:       re,
			Message:     msg,
			SkipIndices: skip,
		})
	}
	return out, nil
}

func parseSkipIndices(v any) ([]int, error) {
	if v == nil {
		return nil, nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("must be a list, got %T", v)
	}
	out := make([]int, 0, len(list))
	for i, item := range list {
		n, ok := toInt(item)
		if !ok {
			return nil, fmt.Errorf("entry %d must be an integer, got %T", i, item)
		}
		out = append(out, n)
	}
	sort.Ints(out)
	return out, nil
}

func asStringMap(v any) (map[string]any, error) {
	switch m := v.(type) {
	case map[string]any:
		return m, nil
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out, nil
	}
	return nil, fmt.Errorf("must be a map, got %T", v)
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n != float64(int(n)) {
			return 0, false
		}
		return int(n), true
	}
	return 0, false
}

type heading struct {
	level int
	line  int
}

func collectHeadings(f *lint.File) []heading {
	var out []heading
	_ = ast.Walk(f.AST, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		out = append(out, heading{
			level: h.Level,
			line:  astutil.HeadingLine(h, f),
		})
		return ast.WalkSkipChildren, nil
	})
	sort.Slice(out, func(i, j int) bool {
		return out[i].line < out[j].line
	})
	return out
}

type paragraph struct {
	line int
	text string
}

func collectParagraphs(f *lint.File) []paragraph {
	var out []paragraph
	_ = ast.Walk(f.AST, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		p, ok := n.(*ast.Paragraph)
		if !ok {
			return ast.WalkContinue, nil
		}
		if astutil.IsTable(p, f) {
			return ast.WalkContinue, nil
		}
		out = append(out, paragraph{
			line: astutil.ParagraphLine(p, f),
			text: mdtext.ExtractPlainText(p, f.Source),
		})
		return ast.WalkContinue, nil
	})
	return out
}

// scopeEnd returns the exclusive end line of the section that begins
// at headings[i]. The section ends at the first subsequent heading
// whose level is less than or equal to headings[i].level (so
// sub-sections are kept inside the section), or at totalLines+1 when
// no such heading exists.
func scopeEnd(headings []heading, i, totalLines int) int {
	for j := i + 1; j < len(headings); j++ {
		if headings[j].level <= headings[i].level {
			return headings[j].line
		}
	}
	return totalLines + 1
}

// sectionBody concatenates paragraph text inside [start, end). Joins
// with a space so adjacent paragraphs do not appear as one word when
// the regex tries cross-paragraph matches.
func sectionBody(paragraphs []paragraph, start, end int) string {
	var parts []string
	for _, p := range paragraphs {
		if p.line < start || p.line >= end {
			continue
		}
		parts = append(parts, p.text)
	}
	return strings.Join(parts, " ")
}

var (
	_ rule.Configurable = (*Rule)(nil)
	_ rule.Defaultable  = (*Rule)(nil)
)

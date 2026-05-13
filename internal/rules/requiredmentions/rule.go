// Package requiredmentions implements MDS058, which flags
// heading-bounded sections whose body text does not contain every
// configured substring.
package requiredmentions

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jeduden/mdsmith/internal/lint"
	"github.com/jeduden/mdsmith/internal/mdtext"
	"github.com/jeduden/mdsmith/internal/rule"
	"github.com/jeduden/mdsmith/internal/rules/astutil"
	"github.com/jeduden/mdsmith/internal/rules/settings"
	"github.com/yuin/goldmark/ast"
)

func init() {
	rule.Register(&Rule{})
}

// Rule flags heading-bounded sections that do not mention every
// configured substring at least once. Walks every heading and tests
// each entry in `mentions:` against the section's prose (including
// nested sub-sections). A missing mention emits one diagnostic at the
// section's heading line so the per-scope override from plan 146 can
// retain it via line-range filtering.
type Rule struct {
	Mentions []string
}

// ID implements rule.Rule.
func (r *Rule) ID() string { return "MDS058" }

// Name implements rule.Rule.
func (r *Rule) Name() string { return "required-mentions" }

// Category implements rule.Rule.
func (r *Rule) Category() string { return "meta" }

// EnabledByDefault implements rule.Defaultable.
func (r *Rule) EnabledByDefault() bool { return false }

// Check implements rule.Rule.
func (r *Rule) Check(f *lint.File) []lint.Diagnostic {
	if f.AST == nil || len(r.Mentions) == 0 {
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
		for _, m := range r.Mentions {
			if m == "" {
				continue
			}
			if strings.Contains(body, m) {
				continue
			}
			diags = append(diags, lint.Diagnostic{
				File:     f.Path,
				Line:     h.line,
				Column:   1,
				RuleID:   r.ID(),
				RuleName: r.Name(),
				Severity: lint.Warning,
				Message: fmt.Sprintf(
					"section is missing required mention %q", m,
				),
			})
		}
	}
	return diags
}

// ApplySettings implements rule.Configurable.
func (r *Rule) ApplySettings(s map[string]any) error {
	for k, v := range s {
		switch k {
		case "mentions":
			ss, ok := settings.ToStringSlice(v)
			if !ok {
				return fmt.Errorf(
					"required-mentions: mentions must be a list of strings, got %T",
					v,
				)
			}
			r.Mentions = ss
		default:
			return fmt.Errorf(
				"required-mentions: unknown setting %q", k,
			)
		}
	}
	return nil
}

// DefaultSettings implements rule.Configurable.
func (r *Rule) DefaultSettings() map[string]any {
	return map[string]any{
		"mentions": []string{},
	}
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

func scopeEnd(headings []heading, i, totalLines int) int {
	for j := i + 1; j < len(headings); j++ {
		if headings[j].level <= headings[i].level {
			return headings[j].line
		}
	}
	return totalLines + 1
}

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

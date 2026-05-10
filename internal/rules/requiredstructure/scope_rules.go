package requiredstructure

import (
	"strings"

	"github.com/jeduden/mdsmith/internal/fieldinterp"
	"github.com/jeduden/mdsmith/internal/lint"
	"github.com/jeduden/mdsmith/internal/rule"
	"github.com/jeduden/mdsmith/internal/schema"
)

// applyScopeRules walks the schema tree to find scopes that declare
// per-scope rule overrides and re-runs each named rule against the
// document, filtering diagnostics to the scope's line range. This is
// the entry point for plan 146's per-scope rule-config feature.
//
// The implementation is intentionally minimal: the override applies
// on top of the rule's defaults rather than the file's full
// effective config. The fixture for this feature (same prose in two
// sections, one with a stricter override) is met by this baseline;
// the full file→scope merge is a follow-up.
func (r *Rule) applyScopeRules(f *lint.File, sch *schema.Schema) []lint.Diagnostic {
	if sch == nil {
		return nil
	}
	heads := schema.ExtractDocHeadings(f)
	rootLevel := sch.EffectiveRootLevel()
	body := skipBelow(heads, rootLevel)
	var diags []lint.Diagnostic
	walkScopes(sch.Sections, body, 0, rootLevel, len(f.Lines)+1,
		func(sc schema.Scope, startLine, endLine int) {
			if len(sc.Rules) == 0 {
				return
			}
			diags = append(diags, runScopeRules(f, sc, startLine, endLine)...)
		})
	return diags
}

// skipBelow strips heading entries whose level is shallower than
// rootLevel so the walker starts at the first heading the schema
// actually covers.
func skipBelow(heads []schema.DocHeading, rootLevel int) []schema.DocHeading {
	for i, h := range heads {
		if h.Level >= rootLevel {
			return heads[i:]
		}
	}
	return nil
}

// walkScopes performs a positional walk of the schema scope tree
// against doc headings, mirroring the validator's matching order. It
// calls visit for each scope that matched a doc heading, providing
// the inclusive 1-based start line and the exclusive end line of
// the scope's content range.
//
// The walk is structural — it pairs scopes to doc headings by
// matching text and level so the visit callback receives accurate
// line ranges to filter diagnostics with.
func walkScopes(
	scopes []schema.Scope, heads []schema.DocHeading,
	docIdx, expectedLevel, fileEnd int,
	visit func(sc schema.Scope, startLine, endLine int),
) int {
	for _, sc := range scopes {
		if sc.Wildcard {
			continue
		}
		matched := findMatchingHead(sc, heads, docIdx, expectedLevel)
		if matched < 0 {
			continue
		}
		dh := heads[matched]
		end := scopeEndLine(heads, matched, expectedLevel, fileEnd)
		visit(sc, dh.Line, end)
		if len(sc.Sections) > 0 {
			walkScopes(sc.Sections, heads, matched+1,
				expectedLevel+1, end, visit)
		}
		docIdx = matched + 1
	}
	return docIdx
}

func findMatchingHead(sc schema.Scope, heads []schema.DocHeading, start, expectedLevel int) int {
	for j := start; j < len(heads); j++ {
		dh := heads[j]
		if dh.Level < expectedLevel {
			return -1
		}
		if scopeTextMatches(sc, dh) {
			return j
		}
	}
	return -1
}

// scopeEndLine returns the exclusive end-line of the section
// beginning at heads[matched]. The section ends at the first
// subsequent heading whose level is <= expectedLevel, or at fileEnd
// when no such heading follows.
func scopeEndLine(
	heads []schema.DocHeading, matched, expectedLevel, fileEnd int,
) int {
	for j := matched + 1; j < len(heads); j++ {
		if heads[j].Level <= expectedLevel {
			return heads[j].Line
		}
	}
	return fileEnd
}

// scopeTextMatches reports whether sc matches dh by heading text.
// "?" matches any text; aliases are tried alongside the primary
// heading. Field-interpolated patterns use a literal-fragment scan
// so the walker can identify the heading the validator just
// confirmed, without re-deriving the field values.
func scopeTextMatches(sc schema.Scope, dh schema.DocHeading) bool {
	if sc.Wildcard {
		return false
	}
	if sc.Heading == "?" {
		return true
	}
	if matchesScopeText(sc.Heading, dh.Text) {
		return true
	}
	for _, a := range sc.Aliases {
		if matchesScopeText(a, dh.Text) {
			return true
		}
	}
	return false
}

func matchesScopeText(pattern, text string) bool {
	if !fieldinterp.ContainsField(pattern) {
		return pattern == text
	}
	parts := fieldinterp.SplitOnFields(pattern)
	idx := 0
	for _, p := range parts {
		if p == "" {
			continue
		}
		next := strings.Index(text[idx:], p)
		if next < 0 {
			return false
		}
		idx += next + len(p)
	}
	return true
}

// runScopeRules executes each rule named in sc.Rules and returns
// diagnostics that fall within the scope's line range. Each rule is
// cloned and configured with its defaults deep-merged with the
// scope's override.
func runScopeRules(
	f *lint.File, sc schema.Scope, startLine, endLine int,
) []lint.Diagnostic {
	var diags []lint.Diagnostic
	for name, override := range sc.Rules {
		base := rule.ByName(name)
		if base == nil {
			continue
		}
		configured := rule.CloneRule(base)
		if c, ok := configured.(rule.Configurable); ok {
			if err := c.ApplySettings(override); err != nil {
				continue
			}
		}
		for _, d := range configured.Check(f) {
			if d.Line >= startLine && d.Line < endLine {
				diags = append(diags, d)
			}
		}
	}
	return diags
}

// Package schema models the document-structure schemas that drive
// MDS020 (required-structure). A schema describes what a Markdown
// document's front matter, filename, and heading tree must look like.
//
// Two sources feed the same in-memory representation:
//
//   - Inline. A YAML schema block under kinds.<name>.schema: in
//     .mdsmith.yml.
//   - File. A proto.md file referenced by
//     rules.required-structure.schema: (the legacy heading-template
//     form).
//
// Both parse into a Schema whose Sections is a recursive tree of
// Scope nodes. The validator walks a document's AST against that
// tree, emitting diagnostics through the lint.Diagnostic shape.
//
// See plan/146_inline-schema-in-kinds.md for the design context.
package schema

// SectionWildcard is the literal text used in a proto.md heading or
// an inline `sections:` entry to mark a positional escape hatch: a
// slot that does not require any heading and that tolerates any
// unlisted sections in its place, even under closed: true.
const SectionWildcard = "..."

// Schema is the parsed representation of a single document schema.
// It is produced by the inline YAML parser or the proto.md file
// parser; both feed the same struct.
type Schema struct {
	// Frontmatter maps each front-matter key to a CUE expression that
	// constrains its value. The map preserves user keys verbatim,
	// including any trailing "?" optional-field marker.
	Frontmatter map[string]string

	// Require carries constraints that apply to the document as a
	// whole (filename pattern, etc.).
	Require Require

	// Sections holds the top-level section list (H2). Each Scope may
	// itself nest further sections (H3, H4, ...). The Schema's H1 is
	// reserved for the document title and is constrained separately
	// via the first-line-heading rule and any title-bearing
	// frontmatter field, so it is not represented here.
	Sections []Scope

	// Closed reports whether the root scope is strict: when true,
	// unlisted top-level headings produce a diagnostic; when false,
	// they are tolerated between listed sections. File-based schemas
	// default to Closed=true to preserve the historical
	// heading-template semantics; inline schemas default to false
	// per plan 146.
	Closed bool

	// Source is a human-readable label (file path for file-based
	// schemas, kind name for inline schemas) used in diagnostics
	// referring to the schema itself.
	Source string

	// RootLevel is the heading level of entries in Sections.
	// Inline schemas use 2 (H1 belongs to the title). File-based
	// schemas adopt whatever level the topmost heading in the
	// proto.md uses — usually 1 for a `# ?` wildcard, 2 when the
	// file declares only `## ...` rows.
	RootLevel int
}

// Require captures the schema-level constraints that apply to the
// document as a whole.
type Require struct {
	// Filename is a glob the document basename must match. Empty
	// means no filename constraint.
	Filename string
}

// Scope binds an AST subtree (a section) to a set of constraints and
// per-rule config overrides. The root scope's children are the
// top-level (H2) section list; their children are H3, and so on.
// Levels come from depth in the tree.
type Scope struct {
	// Heading is the literal heading text. No "#" markers; the level
	// comes from depth in the tree. May contain placeholder tokens
	// ({n}, {slug}, {title}) when Repeats is true. Empty when
	// Wildcard is true.
	Heading string

	// Required reports whether a matching heading must appear in
	// the document. Defaults to true on parse; wildcard scopes set
	// it to false.
	Required bool

	// Aliases lists alternate heading texts that match this scope.
	// An empty list means only Heading matches.
	Aliases []string

	// Sections is the recursive list of nested sections (one level
	// deeper in the document tree).
	Sections []Scope

	// Repeats reports whether Heading is a pattern (with placeholder
	// tokens) that may match zero or more sections.
	Repeats bool

	// Sequential, on a repeating scope, asserts no gaps and no
	// duplicates in the {n} placeholder values.
	Sequential bool

	// Min and Max bound the match count of a repeating scope. Zero
	// means unbounded.
	Min int
	Max int

	// Closed reports whether this scope is strict: when true,
	// unlisted child headings produce a diagnostic; when false, they
	// are tolerated between listed sub-sections.
	Closed bool

	// Wildcard reports whether this scope is a "..." escape hatch:
	// it does not require any heading and tolerates any unlisted
	// sections at its position, even under Closed: true.
	Wildcard bool

	// Rules carries per-scope rule-config overrides. Each entry maps
	// a rule name to a settings map. The MDS020 walker re-runs each
	// named rule with these settings against the document and
	// filters diagnostics to the scope's heading range.
	//
	// Today the override stacks on the rule's defaults, not the
	// file's full effective config (defaults → kinds → file globs →
	// scope). Threading the full merge through the engine is a
	// tracked follow-up on plan 146; see docs/guides/schemas.md.
	Rules map[string]map[string]any
}

// IsEmpty reports whether s carries no constraints. Used by callers
// (notably MDS020) to short-circuit when a kind declares no schema.
func (s *Schema) IsEmpty() bool {
	if s == nil {
		return true
	}
	return len(s.Frontmatter) == 0 &&
		s.Require.Filename == "" &&
		len(s.Sections) == 0
}

// EffectiveRootLevel returns the heading level of the root scope
// list, falling back to 2 when unset.
func (s *Schema) EffectiveRootLevel() int {
	if s == nil || s.RootLevel <= 0 {
		return 2
	}
	return s.RootLevel
}

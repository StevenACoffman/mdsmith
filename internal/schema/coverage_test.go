package schema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- ParseInline edge cases ----

func TestParseInline_RejectsNonIntegerFloat(t *testing.T) {
	raw := map[string]any{
		"sections": []any{
			map[string]any{
				"heading": "Repeating",
				"repeats": true,
				"min":     1.5,
			},
		},
	}
	_, err := ParseInline(raw, "kind x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be an integer")
}

func TestParseInline_AcceptsIntegerFloat(t *testing.T) {
	// YAML decoders surface plain numbers as float64; whole-number
	// floats must still pass as integers.
	raw := map[string]any{
		"sections": []any{
			map[string]any{
				"heading": "Repeating",
				"repeats": true,
				"min":     1.0,
				"max":     3.0,
			},
		},
	}
	sch, err := ParseInline(raw, "kind x")
	require.NoError(t, err)
	require.Len(t, sch.Sections, 1)
	assert.Equal(t, 1, sch.Sections[0].Min)
	assert.Equal(t, 3, sch.Sections[0].Max)
}

func TestParseInline_RejectsEmptyHeading(t *testing.T) {
	raw := map[string]any{
		"sections": []any{
			map[string]any{"required": true},
		},
	}
	_, err := ParseInline(raw, "kind x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty heading")
}

func TestParseInline_RejectsBlankHeading(t *testing.T) {
	raw := map[string]any{
		"sections": []any{
			map[string]any{"heading": "   ", "required": true},
		},
	}
	_, err := ParseInline(raw, "kind x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty heading")
}

func TestParseInline_AcceptsScopeRulesMapping(t *testing.T) {
	raw := map[string]any{
		"sections": []any{
			map[string]any{
				"heading": "Decision",
				"rules": map[string]any{
					"paragraph-readability": map[string]any{
						"max-index": 12.0,
					},
				},
			},
		},
	}
	sch, err := ParseInline(raw, "kind x")
	require.NoError(t, err)
	require.Len(t, sch.Sections, 1)
	require.Contains(t, sch.Sections[0].Rules, "paragraph-readability")
}

func TestParseInline_FrontmatterExprAcceptsScalars(t *testing.T) {
	// Scalars (bool/number) become JSON-encoded CUE constants —
	// this exercises the frontmatterExpr non-string branches.
	raw := map[string]any{
		"frontmatter": map[string]any{
			"active":  true,
			"version": 1,
		},
	}
	sch, err := ParseInline(raw, "kind x")
	require.NoError(t, err)
	cue := sch.FrontmatterCUE()
	assert.Contains(t, cue, "active: true")
	assert.Contains(t, cue, "version: 1")
}

// ---- ParseFile include expansion ----

func TestParseFile_ExpandsInclude(t *testing.T) {
	dir := t.TempDir()
	// Fragment to include.
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "frag.md"),
		[]byte("## Tasks\n"), 0o644))
	main := writeFile(t, dir, "proto.md",
		"# ?\n\n## Goal\n\n<?include\nfile: frag.md\n?>\n")
	sch, err := ParseFile(&FileReader{}, main)
	require.NoError(t, err)
	require.Len(t, sch.Sections, 1)
	children := sch.Sections[0].Sections
	require.Len(t, children, 2, "include should splice Tasks after Goal")
	assert.Equal(t, "Goal", children[0].Heading)
	assert.Equal(t, "Tasks", children[1].Heading)
}

func TestParseFile_RejectsAbsoluteIncludePath(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "proto.md",
		"# ?\n\n<?include\nfile: /etc/passwd\n?>\n")
	_, err := ParseFile(&FileReader{}, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute file path")
}

func TestParseFile_RejectsTraversalInIncludePath(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "proto.md",
		"# ?\n\n<?include\nfile: ../leak.md\n?>\n")
	_, err := ParseFile(&FileReader{}, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `".."`)
}

func TestParseFile_DetectsIncludeCycle(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "a.md"),
		[]byte("<?include\nfile: b.md\n?>\n"), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "b.md"),
		[]byte("<?include\nfile: a.md\n?>\n"), 0o644))
	_, err := ParseFile(&FileReader{}, filepath.Join(dir, "a.md"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cyclic include")
}

// ---- ValidateFrontmatterSyntax ----

func TestValidateFrontmatterSyntax_AcceptsEmpty(t *testing.T) {
	require.NoError(t, ValidateFrontmatterSyntax(&Schema{}))
}

func TestValidateFrontmatterSyntax_AcceptsValid(t *testing.T) {
	sch := &Schema{Frontmatter: map[string]string{
		"id": `=~"^RFC-[0-9]{4}$"`,
	}}
	require.NoError(t, ValidateFrontmatterSyntax(sch))
}

func TestValidateFrontmatterSyntax_RejectsInvalidCUE(t *testing.T) {
	sch := &Schema{Frontmatter: map[string]string{
		"id": "int &",
	}}
	err := ValidateFrontmatterSyntax(sch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid schema frontmatter CUE")
}

// ---- Validate frontmatter CUE-placeholder skip ----

func TestValidate_SkipsCUECheckWhenFmIsCUE(t *testing.T) {
	raw := map[string]any{
		"frontmatter": map[string]any{
			"id": `=~"^RFC-[0-9]{4}$"`,
		},
	}
	sch, err := ParseInline(raw, "kind rfc")
	require.NoError(t, err)
	doc := newDocFile(t, "doc.md",
		"---\nid: NOT-AN-RFC\n---\n# T\n")
	diags := Validate(doc, sch, map[string]any{"id": "NOT-AN-RFC"}, true, makeDiagForTest)
	assert.Empty(t, diags,
		"fmIsCUE=true should skip the CUE check entirely")
}

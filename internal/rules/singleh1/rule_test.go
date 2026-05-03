package singleh1

import (
	"testing"

	"github.com/jeduden/mdsmith/internal/lint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFile(t *testing.T, src string) *lint.File {
	t.Helper()
	f, err := lint.NewFile("test.md", []byte(src))
	require.NoError(t, err)
	return f
}

func newFileStrip(t *testing.T, src string) *lint.File {
	t.Helper()
	f, err := lint.NewFileFromSource("test.md", []byte(src), true)
	require.NoError(t, err)
	return f
}

// --- Check tests ---

func TestCheck_OneH1_NoViolation(t *testing.T) {
	f := newFile(t, "# Title\n\n## Section\n\n### Sub\n")
	diags := (&Rule{}).Check(f)
	assert.Empty(t, diags)
}

func TestCheck_NoHeadings_NoViolation(t *testing.T) {
	f := newFile(t, "Just text.\n")
	diags := (&Rule{}).Check(f)
	assert.Empty(t, diags)
}

func TestCheck_ZeroH1_NoViolation(t *testing.T) {
	f := newFile(t, "## Section\n\n### Sub\n")
	diags := (&Rule{}).Check(f)
	assert.Empty(t, diags)
}

func TestCheck_TwoH1s_SecondFlagged(t *testing.T) {
	f := newFile(t, "# First\n\n## Section\n\n# Second\n")
	diags := (&Rule{}).Check(f)
	require.Len(t, diags, 1)
	assert.Equal(t, "extra H1 heading; only one H1 is allowed per file", diags[0].Message)
	assert.Equal(t, 5, diags[0].Line)
}

func TestCheck_ThreeH1s_SecondAndThirdFlagged(t *testing.T) {
	f := newFile(t, "# First\n\n# Second\n\n# Third\n")
	diags := (&Rule{}).Check(f)
	require.Len(t, diags, 2)
	assert.Equal(t, 3, diags[0].Line)
	assert.Equal(t, 5, diags[1].Line)
}

func TestCheck_SetextH1_Second_Flagged(t *testing.T) {
	src := "# First\n\nSecond\n======\n"
	f := newFile(t, src)
	diags := (&Rule{}).Check(f)
	require.Len(t, diags, 1)
	assert.Equal(t, "extra H1 heading; only one H1 is allowed per file", diags[0].Message)
	assert.Equal(t, 3, diags[0].Line)
}

func TestCheck_FrontMatterTitle_Conflict(t *testing.T) {
	src := "---\ntitle: Foo\n---\n\n# Title\n"
	f := newFileStrip(t, src)
	diags := (&Rule{FrontMatterTitle: "title"}).Check(f)
	require.Len(t, diags, 1)
	assert.Equal(t, "h1 heading conflicts with front-matter title", diags[0].Message)
}

func TestCheck_FrontMatterTitle_Empty_Field_NoConflict(t *testing.T) {
	// front-matter-title: "" disables the check
	src := "---\ntitle: Foo\n---\n\n# Title\n"
	f := newFileStrip(t, src)
	diags := (&Rule{FrontMatterTitle: ""}).Check(f)
	assert.Empty(t, diags)
}

func TestCheck_FrontMatterTitle_FieldAbsent_NoConflict(t *testing.T) {
	// front matter has no 'title' field
	src := "---\nauthor: Alice\n---\n\n# Title\n"
	f := newFileStrip(t, src)
	diags := (&Rule{FrontMatterTitle: "title"}).Check(f)
	assert.Empty(t, diags)
}

func TestCheck_FrontMatterTitle_NoFrontMatter_NoConflict(t *testing.T) {
	f := newFile(t, "# Title\n")
	diags := (&Rule{FrontMatterTitle: "title"}).Check(f)
	assert.Empty(t, diags)
}

func TestCheck_RuleID(t *testing.T) {
	f := newFile(t, "# First\n\n# Second\n")
	diags := (&Rule{}).Check(f)
	require.Len(t, diags, 1)
	assert.Equal(t, "MDS051", diags[0].RuleID)
	assert.Equal(t, "single-h1", diags[0].RuleName)
}

// --- Fix tests ---

func TestFix_TwoH1s_DemotesSecond(t *testing.T) {
	src := "# First\n\n# Second\n"
	f := newFile(t, src)
	got := (&Rule{}).Fix(f)
	assert.Equal(t, "# First\n\n## Second\n", string(got))
}

func TestFix_ThreeH1s_DemotesSecondAndThird(t *testing.T) {
	src := "# First\n\n# Second\n\n# Third\n"
	f := newFile(t, src)
	got := (&Rule{}).Fix(f)
	assert.Equal(t, "# First\n\n## Second\n\n## Third\n", string(got))
}

func TestFix_SetextH1_DemotesToSetextH2(t *testing.T) {
	src := "# First\n\nSecond\n======\n"
	f := newFile(t, src)
	got := (&Rule{}).Fix(f)
	assert.Equal(t, "# First\n\nSecond\n------\n", string(got))
}

func TestFix_FrontMatterConflict_NoFix(t *testing.T) {
	src := "---\ntitle: Foo\n---\n\n# Title\n"
	f := newFileStrip(t, src)
	got := (&Rule{FrontMatterTitle: "title"}).Fix(f)
	// Source unchanged (Fix returns original source when only FM conflict present)
	assert.Equal(t, string(f.Source), string(got))
}

func TestFix_OneH1_NoChange(t *testing.T) {
	src := "# Title\n\n## Section\n"
	f := newFile(t, src)
	got := (&Rule{}).Fix(f)
	assert.Equal(t, src, string(got))
}

// --- Configurable ---

func TestApplySettings_FrontMatterTitle(t *testing.T) {
	r := &Rule{FrontMatterTitle: "title"}
	require.NoError(t, r.ApplySettings(map[string]any{"front-matter-title": ""}))
	assert.Equal(t, "", r.FrontMatterTitle)
}

func TestApplySettings_UnknownKey(t *testing.T) {
	r := &Rule{}
	err := r.ApplySettings(map[string]any{"bogus": "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown setting")
}

func TestDefaultSettings(t *testing.T) {
	r := &Rule{FrontMatterTitle: "title"}
	ds := r.DefaultSettings()
	assert.Equal(t, "title", ds["front-matter-title"])
}

func TestEnabledByDefault(t *testing.T) {
	assert.False(t, (&Rule{}).EnabledByDefault())
}

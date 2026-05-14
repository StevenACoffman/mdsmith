---
summary: >-
  Section-schema reference: the entry-shape
  vocabulary used in inline `kinds.<name>.schema:`
  blocks and `proto.md` files. Covers the
  `heading:` discriminator, the `regex:` matcher
  with `{n}` and `{field}` preprocessor tokens,
  the `repeat: {min, max}` cardinality field, and
  the matching algorithm.
---
# Section schema

> **Status: upcoming.** This page documents the
> shape defined by
> [plan 156](../../plan/156_schema-entry-unification.md).
> It is not yet implemented. The current parser
> accepts the older shape documented in the
> [schema guide](../guides/schemas.md). When plan
> 156 lands, this notice is removed and the guide
> is rewritten to drop every reference to the old
> shape.

A **section schema** describes the heading
structure mdsmith expects in a document. It
pairs with frontmatter and filename constraints
to form a kind's required-structure schema.

## At a glance

```yaml
schema:
  filename: "RFC-[0-9][0-9][0-9][0-9].md"
  frontmatter:
    id: '=~"^RFC-[0-9]{4}$"'
    status: '"draft" | "ratified"'
  sections:
    - heading: null                              # preamble
    - heading: "Overview"                        # exact-match
    - heading:
        regex: 'Intro|Getting Started'           # disjunction
    - heading:
        regex: 'Step {n}'                        # numeric pattern
        repeat: { min: 1 }                       # one or more
        sequential: true                         # {n} ordered
      sections: [...]
      content: [...]
    - heading:
        regex: '{id}: {name}'                    # frontmatter interpolation
    - heading:
        regex: '.+'
        repeat: { min: 0 }                       # zero or more (slot)
    - heading: "References"
```

Three orthogonal axes describe each entry:

- **Discriminator** — what kind of section it
  is (`heading:` value).
- **Matcher** — what text it accepts
  (`regex:`).
- **Cardinality** — how many headings it claims
  (`repeat:`).

## Entry shapes

Every entry in `sections:` sets exactly one
`heading:` key. Its value's YAML type
discriminates the form.

### `heading: null` — no-heading section

```yaml
- heading: null
```

Represents the preamble. At the top level, the
preamble is the content before any heading. In a
nested `sections:`, it is the content between
the parent heading and the first child.

Only valid as the first entry of its `sections:`
list. Any later position parse-errors.

A null entry accepts `content:` and `rules:`. It
rejects `regex:` and `repeat:` — those live
inside the `heading:` mapping form.

### `heading: <string>` — exact-match sugar

```yaml
- heading: "Overview"
```

Sugar for the mapping form with the string
regex-escaped into `regex:`. Equivalent to:

```yaml
- heading:
    regex: 'Overview'
```

The bare string is the most common form. Use it
when the heading text is fixed and you want
exactly one occurrence.

### `heading: { regex, repeat?, sequential? }` — full form

```yaml
- heading:
    regex: 'Step {n}'
    repeat: { min: 1, max: 5 }
    sequential: true
```

The full form makes regex, cardinality, and
ordering explicit. `regex:` is required when the
value is a mapping.

## The regex matcher

`regex:` is a CUE-compatible regex over the
heading's rendered plain text.

**Dialect.** Go RE2, the same dialect CUE's
`=~` operator uses. No backreferences, no
lookaround. Plus two mdsmith preprocessor
tokens described below.

**Anchoring.** Whole-string. `regex: 'Overview'`
matches a heading whose text is exactly
`Overview`. The bare-string sugar behaves the
same way. For a substring, write
`regex: '.*Overview.*'`.

**Match target.** The regex sees the heading's
rendered plain text, not the raw source.
Rendering strips inline emphasis, link wrappers
(keeping link text), code-span backticks
(keeping contents), heading attribute lists
(`{#id}`), and trailing ATX `#`s.

**Case.** Sensitive. Use `(?i)` for
insensitive.

## Preprocessor: `{n}` and `{field}`

`{n}` and `{field}` are mdsmith preprocessor
tokens. They rewrite the `regex:` string before
RE2 compilation. The same tokens work in
`proto.md` heading rows.

### `{n}` — numeric placeholder

`{n}` expands to `(?P<n>[0-9]+)` — a named
numeric capture. Use it for headings like
`## Step 1`, `## Step 2`. Limit: one `{n}` per
pattern.

```yaml
- heading:
    regex: 'Step {n}'
    repeat: { min: 1 }
    sequential: true
```

### `{field}` — frontmatter interpolation

`{field}` expands at validate-time. The
document's frontmatter value for `field` is
regex-escaped and inserted in place. Use it when
the heading text must equal a frontmatter value:

```yaml
- heading:
    regex: '{id}: {name}'
```

If `id: "MDS001"` and `name: "first-line"`, the
schema matches `## MDS001: first-line`.

### `sequential:` — ordering check

`sequential: true` is a sibling field on the
heading entry. Only meaningful when `regex:`
contains `{n}`. Asserts that captured numeric
values across matched headings are in
increasing order with no gaps.

`sequential: true` without `{n}` parse-errors.

## The repeat field

`repeat: { min: int, max: int }` bounds how
many consecutive headings the entry claims.
Both fields are optional within the mapping;
both must be ≥ 0.

### Defaults

| `repeat:`            | Meaning           |
|----------------------|-------------------|
| absent               | exactly one       |
| `{ min: 0 }`         | zero or more      |
| `{ min: 1 }`         | one or more       |
| `{ min: 0, max: 1 }` | optional (0 or 1) |
| `{ min: N, max: M }` | bounded N..M      |

`min:` omitted (when `repeat:` is set) defaults
to 0. `max:` omitted defaults to unbounded.

Parse-time rejection: `repeat: {}` (empty),
`max: 0`, `min > max` (both set).
`repeat:` on a `heading: null` entry is
structurally impossible — `repeat:` is a key
inside the `heading:` mapping, not a sibling.

## Matching

Entries match the document's heading sequence
as a positional quantified regex. Each entry
consumes a contiguous run, sized within its
`repeat:` bounds. The walker is greedy by
default and backtracks if a later literal entry
would otherwise be starved.

A heading whose text matches a later literal
entry's `regex:` is claimed for that entry, not
by an earlier wildcard slot. Mirrors plan 146's
slot semantics.

## Sibling fields

Each entry can carry:

- `sections:` — nested entries one heading
  level deeper. Recursive.
- `content:` — AST-node constraints inside the
  section body. See plan 149.
- `rules:` — per-scope rule-config overrides.
- `closed:` — strictness shorthand. When
  `true`, an unlisted heading inside this
  scope produces a diagnostic. Default
  `false`. Express positional flex by listing
  a wildcard slot instead.
- `sequential:` — described above.

## Schema-level fields

```yaml
schema:
  filename: "<glob>"
  frontmatter:
    <key>: <cue-expression>
    "<key>?": <cue-expression>
  sections: [...]
  closed: <bool>
```

- `filename:` — a glob the document basename
  must match. Top-level; no `require:` wrapper.
- `frontmatter:` — per-key CUE constraints.
  Trailing `?` on a key marks it optional.
- `sections:` — the top-level section list.
- `closed:` — strictness for the root scope.
  Valid only on schemas that declare
  `sections:`. A frontmatter-only kind that
  sets `closed:` parse-errors.

## `proto.md` file syntax

Proto.md files remain a literal-template
surface. Heading rows in the body act as the
schema's `sections:` list.

| Row syntax        | Equivalent inline entry                        |
|-------------------|------------------------------------------------|
| `## Literal text` | `heading: "Literal text"`                      |
| `## ?`            | `heading: { regex: '.+' }`                     |
| `## ...`          | `heading: { regex: '.+', repeat: { min: 0 } }` |
| `## Step {n}`     | `heading: { regex: 'Step {n}' }`               |
| `## {id}`         | `heading: { regex: '{id}' }`                   |

Proto.md cannot express `repeat: { min, max }`
or `sequential:`. Callers needing those switch
to the inline-YAML form on a kind in
`.mdsmith.yml`.

The `<?require filename: "..."?>` directive
in proto.md bodies is unchanged.

## Migration from the old shape

Hard cutover. Old-shape keys parse-error with
a "removed; see plan 156" diagnostic naming
the replacement.

| Old shape                          | New shape                                            |
|------------------------------------|------------------------------------------------------|
| `aliases: [A, B]`                  | `regex: 'A\|B'`                                      |
| `required: true`                   | default (omit `repeat:`)                             |
| `required: false`                  | `repeat: { min: 0, max: 1 }`                         |
| `heading: { unlisted: true }`      | `heading: { regex: '.+', repeat: { min: 0 } }`       |
| Scope-level `repeats: true`        | `repeat: { min: 1 }`                                 |
| Scope-level `min:` / `max:`        | `repeat: { min, max }`                               |
| `require: { filename: "..." }`     | top-level `filename: "..."`                          |
| `closed:` on frontmatter-only kind | dropped (no `sections:` → strictness has no meaning) |

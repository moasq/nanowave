---
name: nanowave-skill-authoring
description: "Use when creating, editing, or reviewing embedded skill files in internal/orchestration/skills/. Covers Anthropic skill format, validation rules, and loading mechanism."
---

# Skill Authoring Guide

## Anthropic Skill Format

Every skill is a directory containing `SKILL.md` with YAML frontmatter:

```yaml
---
name: my-skill-name
description: "Use when [specific trigger condition]. [What this skill provides]."
---

# Skill Body

Instructions and rules here...
```

### Frontmatter Rules

- **name**: kebab-case, `^[a-z0-9-]{1,64}$}`, must not contain "anthropic" or "claude"
- **description**: 1–1024 characters, must include "Use when" clause, no XML/HTML markup
- **Only `name` and `description`** allowed in frontmatter — no extra fields

### Body Rules

- Body must be 1–499 lines (< 500 line hard limit)
- No empty skill bodies
- References go in `references/` subdirectory (one level only)
- References > 100 lines must include a TOC/Contents heading in the first 30 lines

## Skill Categories

```
skills/
  always/              → Always-enabled skills (design-system, layout, etc.)
  always-watchos/      → watchOS-specific always-enabled
  always-tvos/         → tvOS-specific always-enabled
  core/                → Core rules (excluded from Anthropic schema validation)
  features/            → Feature-specific skills (matched by rule_keys)
  ui/                  → UI component skills
  extensions/          → Extension type skills (widget, live-activity, etc.)
  watchos/             → watchOS platform skills
  tvos/                → tvOS platform skills
  phases/              → Pipeline phase skills (analyzer, planner, builder, etc.)
```

## Loading Mechanism

Skills are embedded via `//go:embed skills` in `setup.go`.

### Phase Skills
`loadPhaseSkillContent()` in `phase_prompts.go`:
1. Reads `SKILL.md` body (strips frontmatter)
2. Reads priority references: `workflow.md`, `output-format.md`, `common-mistakes.md`, `examples.md`
3. Reads remaining `.md` files alphabetically
4. Joins all parts with `\n\n`

### Feature/Rule Skills
`loadRuleContent()` in `setup.go` loads skills matched by `rule_keys` from the plan.

### Always Skills
Written into generated project's `.claude/skills/` directory during workspace setup.

## Validation

Run `make skills-source-validate` which executes:
```
go test ./internal/orchestration -run '^TestSourceSkillsAnthropicComplianceStrict$' -count=1 -v
```

This calls `ValidateAnthropicSourceSkills()` from `skill_compliance.go`, which checks:

| Check | Code |
|-------|------|
| SKILL.md exists in each skill dir | `missing_skill_md` |
| SKILL.md not empty | `empty_skill_md` |
| Valid YAML frontmatter | `malformed_frontmatter` |
| name field present | `missing_frontmatter_name` |
| description field present | `missing_frontmatter_description` |
| name matches `^[a-z0-9-]{1,64}$` | `invalid_skill_name` |
| name doesn't contain reserved words | `reserved_skill_name` |
| description ≤ 1024 chars | `description_too_long` |
| description has no markup | `description_contains_markup` |
| description includes "Use when" | `description_missing_use_when` |
| body not empty | `empty_skill_body` |
| body < 500 lines | `skill_body_too_long` |
| local links resolve | `broken_local_link` |
| no nested reference links | `nested_reference_markdown_link` |
| long references have TOC | `long_reference_missing_toc` |

**Note**: `core/` is intentionally excluded from Anthropic schema validation. Core files are plain markdown rules, not Anthropic-format skills.

## Creating a New Skill

1. Choose the right category directory
2. Create `skills/{category}/{skill-name}/SKILL.md`
3. Add YAML frontmatter with `name` and `description`
4. Write body (< 500 lines)
5. Add `references/` subdirectory for supporting content if needed
6. Run `make skills-source-validate`
7. Run `make test` to ensure no regressions

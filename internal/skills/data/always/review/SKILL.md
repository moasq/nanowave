---
name: "review"
description: "Quality review and accessibility audit workflow: severity-based findings, structured Markdown output, fix planning. Use when running /quality-review, /accessibility-audit, reviewing code quality, or auditing accessibility compliance. Triggers: review, audit, quality, accessibility, a11y, findings, severity."
---
# Review Workflow

Use this skill when reviewing generated project quality or running a focused accessibility audit.

Read the relevant guide for your task:
- [quality-review.md](reference/quality-review.md) — project quality gate review workflow
- [accessibility-audit.md](reference/accessibility-audit.md) — code-first accessibility audit workflow
- [output-format.md](reference/output-format.md) — required structured Markdown report format

Scope boundaries:
- This skill is for auditing/reporting, not implementation details.
- For accessibility implementation patterns in SwiftUI, consult the generated `accessibility` skill when present.

## Trigger Cues
Use this skill when requests mention: `review`, `audit`, `accessibility`, `quality`, `findings`.

## Applicability
Primary targets: `ios`, `swiftui`.

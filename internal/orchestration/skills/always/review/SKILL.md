---
name: review
description: "Quality review and accessibility audit workflow for generated SwiftUI iOS apps, including severity-based findings and structured Markdown output."
user-invocable: false
version: "1"
applies_to: ["ios", "swiftui"]
triggers: ["review", "audit", "accessibility", "quality", "findings"]
---
# Review Workflow

Use this skill when reviewing generated project quality or running a focused accessibility audit.

Read the relevant guide for your task:
- [quality-review.md](quality-review.md) — project quality gate review workflow
- [accessibility-audit.md](accessibility-audit.md) — code-first accessibility audit workflow
- [output-format.md](output-format.md) — required structured Markdown report format

Scope boundaries:
- This skill is for auditing/reporting, not implementation details.
- For accessibility implementation patterns in SwiftUI, consult the generated `accessibility` skill when present.


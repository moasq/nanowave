# Accessibility Audit Workflow

Use for focused accessibility audits of generated SwiftUI code.

Default approach:
- Code-first evidence
- Screenshot/image review is optional and only when provided
- Mark uncertain visual-only issues as `needs verification`

Review areas:
- Dynamic Type / fixed font sizing
- VoiceOver labels, hints, traits
- Reduce Motion / Reduce Transparency support
- touch target size and interaction clarity
- color/contrast risks (from code and visual hints)
- form focus/navigation behavior
- status/feedback semantics (not color-only)

Remediation guidance:
- Prefer small, low-risk fixes first
- Re-run local checks after fixes
- Provide re-test steps (including previews and device settings where relevant)


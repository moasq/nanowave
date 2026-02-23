# Review Output Format (Required)

Always report findings first and order by severity.

Severity levels:
- `Critical`
- `High`
- `Medium`
- `Low`

For each finding include:
- severity
- evidence (file path + line when code-based)
- impact/risk
- remediation direction

Use the exact section headings required by the invoked command.
- `/quality-review`: `Scope`, `Findings`, `Fix Plan`, `Verification Steps`, `Escalation`
- `/accessibility-audit`: `Scope`, `Checklist Coverage`, `Findings`, `Remediation Plan`, `Re-test Steps`, `Open Questions` (only if needed)


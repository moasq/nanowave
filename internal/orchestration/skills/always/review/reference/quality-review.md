# Quality Review Workflow

Use for general project quality reviews (not full a11y audits unless explicitly requested).

Checklist focus:
- placeholders removed
- previews present for new `View` files
- AppTheme compliance: ALL colors via AppTheme.Colors.*, ALL fonts via AppTheme.Fonts.*, ALL spacing via AppTheme.Spacing.* — flag any .white, .black, raw .font(.title2), .font(.system(size:)), or numeric padding/spacing as violations
- reachable navigation / no dead feature screens
- root wiring for app-wide settings
- xcodegen configuration policy compliance

Process:
1. Run local project checks first (placeholders, previews, swift structure).
2. Review changed files and connected navigation/wiring.
3. Report findings first with severity and evidence.
4. Recommend `/accessibility-audit` when UI-heavy changes, forms, custom controls, or motion are involved.


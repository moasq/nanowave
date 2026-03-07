# Validate App

Validate the generated app project at the path specified in $ARGUMENTS.

Usage: /validate-app <project-dir> <app-name>

Run `./scripts/validate-app.sh $ARGUMENTS` and report results.
If the script is not available, manually check:
- AppTheme compliance (no hardcoded colors/fonts/spacing)
- MVVM architecture (@Observable, @MainActor, #Preview)
- Forbidden patterns (no networking, no CoreData, no deprecated APIs)
- File structure (all Swift files under 200 lines)

Report a structured PASS/FAIL summary.

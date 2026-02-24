# Common Mistakes

## Contents
- File and plan errors
- Code quality errors
- Build loop errors

## File and Plan Errors

- Writing files not in the plan without a clear reason.
- Using wrong type names or file paths that don't match the plan.
- Editing project.yml manually instead of using xcodegen MCP tools.
- Leaving Placeholder.swift scaffold files — delete them once real code exists.

## Code Quality Errors

- Leaving dead settings toggles not wired at root via @AppStorage.
- Creating views that are never referenced from any navigation path (dead code).
- Missing #Preview blocks on View files.
- Missing empty states on lists and collections.
- Re-declaring types already defined in other project files.
- Hardcoding colors (.white, .black, Color.red) instead of AppTheme.Colors.* tokens.
- Hardcoding fonts (.font(.title2), .font(.system(size:))) instead of AppTheme.Fonts.* tokens.
- Hardcoding spacing (.padding(20), VStack(spacing: 10)) instead of AppTheme.Spacing.* tokens.
- Forgetting to add textPrimary/textSecondary/textTertiary to AppTheme.Colors.
- Forgetting to add Fonts enum to AppTheme with the plan's fontDesign applied.
- Guessing Apple API signatures instead of searching docs first.
- Using NavigationStack for list-detail in universal/iPad apps — use NavigationSplitView instead (auto-collapses to stack on iPhone).

## Build Loop Errors

- Stopping before a clean build (zero errors).
- Treating quality-gate hook warnings as compiler errors.
- Fixing downstream errors before fixing the root cause.
- Making the same fix repeatedly without re-reading the error output.

## Property Wrapper Compatibility

| Observable Type            | Property Wrapper in View |
|----------------------------|--------------------------|
| @Observable (Swift 5.9+)   | @State                   |
| ObservableObject protocol  | @StateObject             |

Error "Generic struct 'StateObject' requires..." → Change @StateObject to @State.

## Common Protocol Requirements

| Feature                | Required Protocol |
|------------------------|-------------------|
| NavigationPath.append  | Hashable          |
| ForEach iteration      | Identifiable      |
| @AppStorage            | RawRepresentable  |
| JSON encoding/decoding | Codable           |

## Multi-Platform Mistakes

- Writing files in the wrong platform source directory (e.g., putting watchOS views in `{AppName}/` instead of `{AppName}Watch/`).
- Not building all schemes after writing files — each platform target must compile independently.

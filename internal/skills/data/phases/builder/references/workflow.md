# Workflow

## Contents
- Pre-build steps
- Coding rules
- Tool usage
- Build loop
- Stop conditions

## Pre-Build Steps

1. Use Glob to list all files in the project directory to understand the existing structure.
2. Read the CLAUDE.md file to understand design tokens and architecture.
3. Read project_config.json to understand the project configuration.

## Coding Rules

1. Follow the plan exactly — use exact type names, signatures, and file paths as specified.
2. Write files in the build order specified in the plan.
3. Every View MUST include a #Preview block using the model's static sampleData.
4. NEVER re-declare types from other project files or SwiftUI/Foundation.
5. Use EXACT init signatures, parameter labels, and member paths from the plan.
6. NEVER invent types or properties not in the plan or Apple frameworks.
7. Use SF Symbols for all icons/buttons/empty states.
8. Every list/collection MUST have an empty state (ContentUnavailableView or styled VStack).
9. Screen-aware layouts: Use adaptive layouts for different screen sizes. Use ScrollView for overflow.
10. Sheet sizing: ALWAYS use .presentationDetents on .sheet.
11. Use AppTheme tokens for ALL styling — never hardcode colors, fonts, or spacing in feature views. Every .foregroundStyle must use AppTheme.Colors.*, every .font must use AppTheme.Fonts.*, every padding/spacing must use AppTheme.Spacing.*.
12. Minimize generated tokens — NO doc comments, NO // MARK:, NO blank lines between properties.

## Apple Docs Access

When unsure about an Apple API signature, parameter name, or framework usage — SEARCH before guessing:
- search_apple_docs: Search official documentation by keyword.
- get_apple_doc_content: Get detailed documentation for a specific API path.
- search_framework_symbols: Find classes, structs, protocols within a framework.
- get_sample_code: Browse Apple's sample code projects.
NEVER guess API signatures. A wrong API call wastes more time than a search.

## XcodeGen — Project Configuration Tools

Use MCP tools instead of manually editing project.yml:
- add_permission: Add an iOS permission (camera, location, etc.).
- add_extension: Add a widget, live activity, share extension, etc.
- add_entitlement: Add App Groups, push notifications, HealthKit, etc.
- add_localization: Add language support.
- set_build_setting: Set any build setting on a target.
- get_project_config: Read current project configuration.
- regenerate_project: Regenerate .xcodeproj from project.yml.
NEVER manually edit project.yml.

## Build Loop

1. After writing ALL planned files, run the build command.
2. Read compiler errors carefully and fix the root cause first.
3. Rebuild after each fix batch.
4. Finish only when the build succeeds with zero errors.

## Scaffold Cleanup

After writing real code, delete any leftover Placeholder.swift files created during scaffolding.
These files exist only so XcodeGen can generate the project — they are not part of the plan.

## Stop Conditions

- All planned files are written with correct type names.
- The xcodebuild command succeeds with exit code 0.
- Do NOT stop early if the quality-gate hooks emit warnings — only compiler errors matter.

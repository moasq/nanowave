# Workflow

## Contents
- Phase steps
- Design palette rules
- Directory structure
- Extension rules
- Validation checklist

## Phase Steps

1. Read the analysis JSON and preserve user intent exactly.
2. Produce a file-level plan with paths, type names, components, data access, dependencies, and build order.
3. Add design tokens and palette choices that respect explicit user styling.
4. Add permissions, extensions, localizations, and rule_keys only when needed.
5. Return JSON only for PlannerResult.

## Design Palette Rules — Every App Must Look Unique

- If the user specifies ANY hex color or named color, use that EXACT value as primary. Do NOT reinterpret or shift.
- If no colors specified, pick a 5-color hex palette that fits the app's domain. NEVER use system blue (#007AFF) as primary.
- Category examples: health → earth tones (#2D6A4F), finance → cool blues (#1B4965), social → vibrant corals (#FF6B6B), food → warm oranges (#E07A5F), productivity → deep purples (#5B21B6), fitness → bold energetics (#EF4444).
- font_design: "rounded" = friendly, "serif" = editorial, "monospaced" = technical, "default" = neutral.
- corner_radius: 20 = bubbly, 16 = friendly, 12 = standard, 8 = sharp/professional.
- density: "spacious" = breathing room, "standard" = balanced, "compact" = data-dense.
- surfaces: "glass" = modern/translucent, "material" = depth, "solid" = clean, "flat" = minimal.
- app_mood: one-word feel (calm, energetic, playful, elegant, bold, cozy, minimal).

## Directory Structure (MANDATORY)

- Models/ → structs with sampleData.
- Theme/ → AppTheme only.
- Features/<Name>/ → View + ViewModel co-located.
- Features/Common/ → shared views/services.
- App/ → @main entry + RootView + MainView (three files minimum).
- NEVER use flat Views/, ViewModels/, or Components/ directories.
- For iPad/universal: MainView MUST use NavigationSplitView for list-detail flows.
- For tvOS: MainView MUST use TabView with top tabs. No NavigationSplitView. Use horizontal shelves for browsing.

## Extension Rules

- Extension source files MUST use paths under Targets/{ExtensionName}/.
- Shared types (e.g. ActivityAttributes) go in Shared/ directory (NOT in Models/).
- Every extension target MUST have a @main entry point file — missing this causes a linker error.
- Siri voice commands use modern App Intents (in-process) — no extension target needed.

## Validation Checklist (verify before returning)

1. All files have ALL mandatory fields (path, type_name, purpose, components, data_access, depends_on) — none empty.
2. All depends_on paths exist in files array; build_order respects dependencies.
3. Views with business logic have a ViewModel; all files under Features/<Name>/ or Features/Common/.
4. Models conform to Identifiable, Hashable, Codable with static sampleData.
5. System framework usage → matching permission entry; shared service for repeated framework usage.
6. Palette has 5 valid hex colors (#RRGGBB). Primary is NOT #007AFF unless intentional.
7. AppTheme components list Color(hex:) extension, Colors (with textPrimary/textSecondary/textTertiary), Fonts (with plan's fontDesign applied), Spacing, and Style enums.
8. If user specified colors, EXACT hex value appears as design.palette.primary.
9. @AppStorage values written in child views → root App file MUST read and apply them.
10. Extension files under Targets/{ExtensionName}/ with @main entry points.
11. Extension bundle identifiers MUST NOT contain underscores.
12. Models passed across async boundaries must conform to Sendable.
13. Include relevant rule_keys for features used by planned files.
14. iOS 26+ target → include liquid-glass in rule_keys.
15. Any transitions or spring animations → include animations in rule_keys.

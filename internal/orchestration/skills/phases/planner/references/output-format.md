# Output Format

## Contents
- JSON structure
- Field rules
- Available rule_keys

Return ONLY valid PlannerResult JSON with design, files, models, permissions, extensions, localizations, platform, platforms, watch_project_shape, device_family, rule_keys, and build_order.

## File Entry Fields

Every file entry must include:
- path: relative file path (e.g. "Features/Notes/NoteListView.swift")
- type_name: primary Swift type in this file (required, non-empty)
- purpose: what this file does
- components: key Swift types and signatures as a single string summary (NOT an array)
- data_access: "in-memory", "@AppStorage", "none", etc.
- depends_on: array of file path strings this file imports from (must exist in files array)
- platform: which platform this file belongs to — `"ios"`, `"watchos"`, `"tvos"`, or `""` for shared/cross-platform files

## Extension Entry Fields

Every extension entry MUST include:
- kind: the extension type — REQUIRED, MUST be non-empty. Valid values: `widget`, `live_activity`, `share`, `notification_service`, `safari`, `app_clip`, `tv_top_shelf`
- name: the Xcode target name (e.g. "MyAppWidget")
- purpose: what this extension does

An extension with an empty `kind` will produce a broken Xcode project (invalid bundle ID, missing NSExtensionPointIdentifier). NEVER omit kind.

## Canonical Field Rules

- `files[].components` must be a single string summary (NOT an array).
- `files[].depends_on` must be an array of file path strings.
- `extensions[].kind` MUST be non-empty — it determines the bundle ID suffix and Info.plist configuration.
- `watch_project_shape` values are only `watch_only` or `paired_ios_watch`.
- If `watch_project_shape` is present, `platform` must be `watchos`.
- `platform` values: `ios` (default), `watchos`, `tvos`.
- `platforms`: array of platform strings when targeting multiple platforms, e.g. `["ios", "watchos", "tvos"]`. Each element must be `"ios"`, `"watchos"`, or `"tvos"`. Omit or use `[]` for single-platform projects.
- For `tvos`, do not set `device_family` or `watch_project_shape`.
- `build_order`: Models → Theme → ViewModels → Views → App. Respects depends_on.

## Available rule_keys

Include a key if ANY file uses that feature. Design-system, navigation, layout, components, and swiftui are always loaded — do NOT include them.

Features: notifications, localization, dark-mode, app-review, website-links, haptics, timers, charts, camera, maps, biometrics, healthkit, speech, storage, apple-translation, siri-intents, foundation-models

UI refinement: view-complexity, typography, color-contrast, spacing-layout, feedback-states, view-composition, accessibility, gestures, adaptive-layout, liquid-glass, animations

Extensions: widgets, live-activities, share-extension, notification-service, safari-extension, app-clips

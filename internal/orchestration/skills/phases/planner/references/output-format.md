# Output Format

## Contents
- JSON structure
- Field rules
- Available rule_keys

Return ONLY valid PlannerResult JSON with design, files, models, permissions, extensions, localizations, platform, platforms, watch_project_shape, device_family, rule_keys, packages, and build_order.

## File Entry Fields

Every file entry must include:
- path: relative file path (e.g. "Features/Notes/NoteListView.swift")
- type_name: primary Swift type in this file (required, non-empty)
- purpose: what this file does
- components: key Swift types and signatures as a single string summary (NOT an array)
- data_access: "in-memory", "@AppStorage", "none", etc.
- depends_on: array of file path strings this file imports from (must exist in files array)
- platform: which platform this file belongs to — `"ios"`, `"watchos"`, `"tvos"`, `"visionos"`, `"macos"`, or `""` for shared/cross-platform files

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
- `platform` values: `ios` (default), `watchos`, `tvos`, `visionos`, `macos`.
- `platforms`: array of platform strings when targeting multiple platforms, e.g. `["ios", "watchos", "tvos"]`. Each element must be `"ios"`, `"watchos"`, `"tvos"`, `"visionos"`, or `"macos"`. Omit or use `[]` for single-platform projects.
- For `tvos`, do not set `device_family` or `watch_project_shape`.
- `build_order`: Models → Theme → ViewModels → Views → App. Respects depends_on.

## Available rule_keys

Include a key if ANY file uses that feature. Design-system, navigation, layout, components, and swiftui are always loaded — do NOT include them.

Features: notifications, localization, dark-mode, app-review, website-links, haptics, timers, charts, camera, maps, biometrics, healthkit, speech, storage, apple-translation, siri-intents, foundation-models

UI refinement: view-complexity, typography, color-contrast, spacing-layout, feedback-states, view-composition, accessibility, gestures, adaptive-layout, liquid-glass, animations

Extensions: widgets, live-activities, share-extension, notification-service, safari-extension, app-clips

## Package Entries

**Default is ZERO packages.** Most apps need none. Only add a package when: (1) no native API exists for the capability, or (2) the native approach would require 100+ lines of complex code that the package eliminates. See the workflow reference for the full decision threshold and native-first table.

Each entry has `name` (the package name) and `reason` (what it enables that native code cannot reasonably achieve).

Available curated packages (the build phase resolves exact URLs, versions, and products):

| Category | Available packages |
|---|---|
| Image loading & caching | Kingfisher, Nuke, SDWebImageSwiftUI |
| Animated GIFs | Gifu |
| SVG rendering | SVGView |
| Image editing | Brightroom, CropViewController |
| Audio waveform | DSWaveformImage |
| Audio engine | AudioKit |
| Animations | Lottie |
| Visual effects | ConfettiSwiftUI, Pow, Vortex |
| Shimmer (animated) | Shimmer |
| Loading indicators | ActivityIndicatorView |
| Toasts & popups | PopupView, AlertToast |
| Onboarding | WhatsNewKit, ConcentricOnboarding |
| Calendar UI | HorizonCalendar |
| Chat UI | ExyteChat |
| Flow / wrap layout | SwiftUI-Flow |
| Waterfall / masonry grid | WaterfallGrid |
| Markdown rendering | MarkdownUI |
| Rich text editing | RichTextKit |
| Syntax highlighting | Highlightr |
| QR codes (stylized) | EFQRCode |
| Keychain storage | KeychainSwift, Valet |

If a feature needs a package not in this table, include your best guess — the build phase will search the internet and resolve it.

Example — app with a photo grid that loads hundreds of remote images with prefetch and disk caching:
```json
"packages": [
  {"name": "Kingfisher", "reason": "Disk-cached image loading with prefetch and downsampling for photo grid — AsyncImage has no disk cache or prefetch"}
]
```

Example — no packages needed (most apps):
```json
"packages": []
```
